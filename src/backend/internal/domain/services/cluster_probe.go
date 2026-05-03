package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	database "github.com/BeanYa/b-ui/src/backend/internal/infra/db"
	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
	logger "github.com/BeanYa/b-ui/src/backend/internal/infra/logging"
)

type clusterProbeStore interface {
	ListMembersWithDomain() ([]model.ClusterMember, error)
	GetLocalNodeID() (string, error)
	SaveMember(*model.ClusterMember) error
}

type ClusterPeerProbeService struct {
	store          clusterProbeStore
	reachability   *ClusterReachabilityService
	secretProvider clusterSecretProvider
	httpClient     *http.Client
}

var errInvalidPeerProtocolResponse = errors.New("invalid peer protocol response")

type clusterProbeResponse struct {
	Success bool           `json:"success"`
	Status  string         `json:"status"`
	Code    string         `json:"code"`
	NodeID  string         `json:"nodeId"`
	Details map[string]any `json:"details,omitempty"`
}

type DBClusterProbeStore struct{}

func NewRuntimeClusterPeerProbeService() *ClusterPeerProbeService {
	return &ClusterPeerProbeService{
		store:          &DBClusterProbeStore{},
		reachability:   &ClusterReachabilityService{store: &dbClusterReachabilityStore{}, policy: DefaultClusterReachabilityPolicy()},
		secretProvider: &SettingService{},
	}
}

func (s *ClusterPeerProbeService) ProbeIdlePeers(ctx context.Context) error {
	members, err := s.getStore().ListMembersWithDomain()
	if err != nil {
		return err
	}
	if len(members) == 0 {
		return nil
	}
	localNodeID, err := s.getStore().GetLocalNodeID()
	if err != nil {
		return err
	}

	byDomain := map[uint][]model.ClusterMember{}
	for _, member := range members {
		byDomain[member.DomainID] = append(byDomain[member.DomainID], member)
	}

	var firstErr error
	rememberErr := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	for domainID, domainMembers := range byDomain {
		if !clusterProbeDomainHasLocalMember(domainMembers, localNodeID) {
			continue
		}
		domainName := domainMembers[0].Domain.Domain
		targetNodeIDs := make([]string, 0, len(domainMembers))
		for _, member := range domainMembers {
			targetNodeIDs = append(targetNodeIDs, member.NodeID)
		}
		if err := s.getReachability().ReconcileMembers(domainID, targetNodeIDs); err != nil {
			logger.ClusterError(logger.ClusterCron, "reachability_probe.reconcile", map[string]interface{}{
				"domain": domainName,
				"error":  err.Error(),
			})
			rememberErr(err)
			continue
		}
		if len(domainMembers) <= 1 {
			continue
		}
		domainProbeCount := 0
		for _, member := range domainMembers {
			if member.NodeID == localNodeID || member.BaseURL == "" || member.Domain == nil {
				continue
			}
			entry, err := s.getReachability().load(member.DomainID, member.NodeID)
			if err != nil {
				rememberErr(err)
				continue
			}
			shouldProbe, err := s.getReachability().shouldProbeWithError(entry)
			if err != nil {
				rememberErr(err)
			}
			if !shouldProbe {
				continue
			}
			if err := s.probeMember(ctx, member, localNodeID); err != nil {
				logger.ClusterError(logger.ClusterCron, "reachability_probe.member_failed", map[string]interface{}{
					"domain":     domainName,
					"targetNode": member.NodeID,
					"error":      err.Error(),
				})
				if _, recordErr := s.getReachability().RecordTransportFailure(member.DomainID, member.NodeID, "probe"); recordErr != nil {
					rememberErr(recordErr)
				}
				continue
			}
			domainProbeCount++
			if _, err := s.getReachability().RecordTransportSuccess(member.DomainID, member.NodeID, "probe"); err != nil {
				rememberErr(err)
			}
		}
		logger.ClusterDebug(logger.ClusterCron, "reachability_probe.domain_done", map[string]interface{}{
			"domain":       domainName,
			"probed_count": domainProbeCount,
		})
	}
	return firstErr
}

func (s *ClusterPeerProbeService) probeMember(ctx context.Context, member model.ClusterMember, localNodeID string) error {
	peerToken := ""
	if member.PeerTokenEncrypted != "" {
		secret, err := s.getSecretProvider().GetSecret()
		if err != nil {
			return err
		}
		peerToken, err = DecryptClusterDomainToken(secret, member.PeerTokenEncrypted)
		if err != nil {
			return err
		}
	}
	heartbeatURL, err := clusterPeerActionURL(
		member.BaseURL,
		effectiveClusterCommunicationEndpointPath(member.Domain.CommunicationEndpointPath),
		effectiveClusterCommunicationProtocolVersion(member.Domain.CommunicationProtocolVersion),
		"heartbeat",
	)
	if err != nil {
		return err
	}
	if localNodeID != "" {
		heartbeatURL += "?node_id=" + url.QueryEscape(localNodeID)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, heartbeatURL, nil)
	if err != nil {
		return err
	}
	if peerToken != "" {
		request.Header.Set("X-Cluster-Token", peerToken)
	}
	response, err := s.httpClientOrDefault().Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if err := requireHTTPSuccess(response, "cluster peer heartbeat"); err != nil {
		return err
	}

	var payload clusterProbeResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return err
	}
	updated := false
	if panelVersion, ok := payload.Details["panelVersion"].(string); ok && panelVersion != "" {
		if member.PanelVersion != panelVersion {
			member.PanelVersion = panelVersion
			updated = true
		}
	}
	if payload.Success && member.Status != "online" {
		member.Status = "online"
		updated = true
	}
	if updated {
		if saveErr := s.getStore().SaveMember(&member); saveErr != nil {
			// member update failure should not fail the probe
		}
	}
	if payload.Success {
		return nil
	}
	if strings.TrimSpace(payload.Status) == "" || strings.TrimSpace(payload.Code) == "" {
		return errInvalidPeerProtocolResponse
	}
	return nil
}

func (s *ClusterPeerProbeService) getStore() clusterProbeStore {
	if s.store != nil {
		return s.store
	}
	return &DBClusterProbeStore{}
}

func (s *ClusterPeerProbeService) getReachability() *ClusterReachabilityService {
	if s.reachability != nil {
		return s.reachability
	}
	return &ClusterReachabilityService{store: &dbClusterReachabilityStore{}, policy: DefaultClusterReachabilityPolicy()}
}

func (s *ClusterPeerProbeService) getSecretProvider() clusterSecretProvider {
	if s.secretProvider != nil {
		return s.secretProvider
	}
	return &SettingService{}
}

func (s *ClusterPeerProbeService) httpClientOrDefault() *http.Client {
	if s.httpClient != nil {
		return s.httpClient
	}
	return &http.Client{Timeout: s.getReachability().policy.ProbeTimeout}
}

func clusterProbeDomainHasLocalMember(members []model.ClusterMember, localNodeID string) bool {
	for _, member := range members {
		if member.NodeID == localNodeID {
			return true
		}
	}
	return false
}

func clusterPeerActionURL(baseURL string, endpointPath string, protocolVersion string, action string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	if err := validateClusterPeerScheme(parsed); err != nil {
		return "", err
	}
	normalizedEndpointPath := "/" + strings.Trim(strings.TrimSpace(endpointPath), "/")
	normalizedProtocolVersion := strings.Trim(strings.TrimSpace(protocolVersion), "/")
	parsed.Path = strings.TrimSuffix(parsed.Path, "/") + normalizedEndpointPath + "/" + normalizedProtocolVersion + "/" + action
	parsed.RawPath = ""
	return parsed.String(), nil
}

func (s *DBClusterProbeStore) ListMembersWithDomain() ([]model.ClusterMember, error) {
	var members []model.ClusterMember
	if err := database.GetDB().Preload("Domain").Find(&members).Error; err != nil {
		return nil, err
	}
	return members, nil
}

func (s *DBClusterProbeStore) GetLocalNodeID() (string, error) {
	localNode, err := (&ClusterLocalIdentityService{}).GetOrCreate()
	if err != nil {
		return "", err
	}
	return localNode.NodeID, nil
}

func (s *DBClusterProbeStore) SaveMember(member *model.ClusterMember) error {
	return database.GetDB().Save(member).Error
}
