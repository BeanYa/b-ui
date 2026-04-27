package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	database "github.com/alireza0/b-ui/src/backend/internal/infra/db"
	"github.com/alireza0/b-ui/src/backend/internal/infra/db/model"
)

type clusterProbeStore interface {
	ListMembersWithDomain() ([]model.ClusterMember, error)
	GetLocalNodeID() (string, error)
}

type ClusterPeerProbeService struct {
	store          clusterProbeStore
	reachability   *ClusterReachabilityService
	secretProvider clusterSecretProvider
	httpClient     *http.Client
}

var errInvalidPeerProtocolResponse = errors.New("invalid peer protocol response")

type clusterProbeResponse struct {
	Success bool   `json:"success"`
	Status  string `json:"status"`
	Code    string `json:"code"`
	NodeID  string `json:"nodeId"`
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
	localNodeID, err := s.getStore().GetLocalNodeID()
	if err != nil {
		return err
	}
	members, err := s.getStore().ListMembersWithDomain()
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
		targetNodeIDs := make([]string, 0, len(domainMembers))
		for _, member := range domainMembers {
			targetNodeIDs = append(targetNodeIDs, member.NodeID)
		}
		if err := s.getReachability().ReconcileMembers(domainID, targetNodeIDs); err != nil {
			rememberErr(err)
			continue
		}
		if len(domainMembers) <= 1 {
			continue
		}
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
			if err := s.probeMember(ctx, member); err != nil {
				if _, recordErr := s.getReachability().RecordTransportFailure(member.DomainID, member.NodeID, "probe"); recordErr != nil {
					rememberErr(recordErr)
				}
				continue
			}
			if _, err := s.getReachability().RecordTransportSuccess(member.DomainID, member.NodeID, "probe"); err != nil {
				rememberErr(err)
			}
		}
	}
	return firstErr
}

func (s *ClusterPeerProbeService) probeMember(ctx context.Context, member model.ClusterMember) error {
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
