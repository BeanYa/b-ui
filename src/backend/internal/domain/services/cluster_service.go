package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/BeanYa/b-ui/src/backend/internal/domain/config"
	"github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster"
	"github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster/router"
	clustertypes "github.com/BeanYa/b-ui/src/backend/internal/domain/services/cluster/types"
	database "github.com/BeanYa/b-ui/src/backend/internal/infra/db"
	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
	"github.com/gofrs/uuid/v5"
)

type ClusterRegisterRequest struct {
	JoinURI     string `json:"joinUri" form:"joinUri"`
	HubURL      string `json:"hubUrl" form:"hubUrl"`
	Name        string `json:"name" form:"name"`
	DisplayName string `json:"displayName" form:"displayName"`
	Domain      string `json:"domain" form:"domain"`
	Token       string `json:"token" form:"token"`
	BaseURL     string `json:"baseUrl" form:"baseUrl"`
}

type ClusterOperationStatus struct {
	ID        string `json:"id"`
	State     string `json:"state"`
	Message   string `json:"message,omitempty"`
	createdAt time.Time
}

type ClusterDomainResponse struct {
	ID                           uint     `json:"id"`
	Domain                       string   `json:"domain"`
	HubURL                       string   `json:"hubUrl"`
	CommunicationEndpointPath    string   `json:"communicationEndpointPath"`
	CommunicationProtocolVersion string   `json:"communicationProtocolVersion"`
	LastVersion                  int64    `json:"lastVersion"`
	SupportedActions             []string `json:"supportedActions"`
}

type ClusterMemberResponse struct {
	ID           uint   `json:"id"`
	DomainID     uint   `json:"domainId"`
	NodeID       string `json:"nodeId"`
	Name         string `json:"name"`
	DisplayName  string `json:"displayName"`
	BaseURL      string `json:"baseUrl"`
	LastVersion  int64  `json:"lastVersion"`
	IsLocal      bool   `json:"isLocal"`
	PanelVersion string `json:"panelVersion"`
	Status       string `json:"status"`
}

type ClusterMemberConnectionResponse struct {
	NodeID      string `json:"nodeId"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	BaseURL     string `json:"baseUrl"`
	Token       string `json:"token,omitempty"`
}

type ClusterMemberActionRequest struct {
	NodeID  string                     `json:"node_id"`
	Request clustertypes.ActionRequest `json:"request"`
}

type ClusterPeerStatus struct {
	Status  string         `json:"status"`
	Code    string         `json:"code"`
	NodeID  string         `json:"nodeId,omitempty"`
	Message string         `json:"message,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

type ClusterService struct {
	SettingService
	localIdentity  ClusterLocalIdentityService
	syncService    ClusterSyncService
	hubClient      clusterHubClient
	store          clusterServiceStore
	secretProvider clusterSecretProvider
	actionRouter   *router.ActionRouter
	runtime        *cluster.Runtime
}

type clusterSecretProvider interface {
	GetSecret() ([]byte, error)
}

type clusterServiceStore interface {
	GetDomainByName(string) (*model.ClusterDomain, error)
	SaveDomain(*model.ClusterDomain) error
	ListDomains() ([]model.ClusterDomain, error)
	GetDomain(uint) (*model.ClusterDomain, error)
	GetMember(uint) (*model.ClusterMember, error)
	GetMemberByNodeID(string) (*model.ClusterMember, error)
	GetMemberByDomainNodeID(uint, string) (*model.ClusterMember, error)
	SaveMember(*model.ClusterMember) error
	ListMembers() ([]model.ClusterMember, error)
	DeleteMember(uint) error
	DeleteDomain(uint) error
	ReplaceDomainMembers(uint, []model.ClusterMember) error
}

var clusterOperations = struct {
	mu    sync.RWMutex
	items map[string]ClusterOperationStatus
}{items: map[string]ClusterOperationStatus{}}

const maxClusterOperations = 128

var errClusterHubURLRequired = errors.New("cluster hub URL is required")
var errClusterDomainRequired = errors.New("cluster domain is required")
var errClusterTokenRequired = errors.New("cluster domain token is required")
var errClusterBaseURLRequired = errors.New("cluster node base URL is required")
var errClusterJoinURIInvalid = errors.New("cluster join URI is invalid")

var clusterDisplayNameBaseURLPattern = regexp.MustCompile(`(?i)^https?://([^/:?#]+)(?::\d+)?(?:[/?#]|$)`)

func (s *ClusterService) Register(request ClusterRegisterRequest) (*ClusterOperationStatus, error) {
	if err := NormalizeClusterRegisterRequest(&request); err != nil {
		return nil, err
	}
	request.Domain = strings.TrimSpace(request.Domain)
	request.HubURL = strings.TrimSpace(request.HubURL)
	request.BaseURL = strings.TrimSpace(request.BaseURL)
	request.Name = strings.TrimSpace(request.Name)
	request.DisplayName = strings.TrimSpace(request.DisplayName)
	if request.HubURL == "" {
		return nil, errClusterHubURLRequired
	}
	if request.Domain == "" {
		return nil, errClusterDomainRequired
	}
	if request.Token == "" {
		return nil, errClusterTokenRequired
	}
	if request.BaseURL == "" {
		return nil, errClusterBaseURLRequired
	}
	if request.DisplayName == "" {
		request.DisplayName = deriveClusterDisplayNameFromBaseURL(request.BaseURL)
	}

	store := s.getStore()
	identity, err := s.localIdentity.GetOrCreate()
	if err != nil {
		return nil, err
	}
	secret, err := s.getSecretProvider().GetSecret()
	if err != nil {
		return nil, err
	}
	encryptedToken, err := EncryptClusterDomainToken(secret, request.Token)
	if err != nil {
		return nil, err
	}
	domain, err := store.GetDomainByName(request.Domain)
	if err != nil && !errors.Is(err, errClusterDomainNotFound) {
		return nil, err
	}
	if domain == nil {
		domain = &model.ClusterDomain{}
	}
	client := s.getHubClient()
	requestID, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}
	_, err = client.RegisterNode(context.Background(), request.HubURL, ClusterHubRegisterNodeRequest{
		RequestID:   requestID.String(),
		DomainID:    request.Domain,
		DomainToken: request.Token,
		Member: ClusterHubMemberRegister{
			MemberID:     identity.NodeID,
			NodeID:       identity.NodeID,
			Address:      request.BaseURL,
			BaseURL:      request.BaseURL,
			PublicKey:    identity.PublicKey,
			Name:         request.Name,
			DisplayName:  request.DisplayName,
			PanelVersion: canonicalizeReleaseTag(config.GetVersion()),
			Status:       "online",
		},
	})
	if err != nil {
		return nil, err
	}
	domain.Domain = request.Domain
	domain.HubURL = request.HubURL
	domain.TokenEncrypted = encryptedToken
	snapshot, err := client.GetSnapshot(context.Background(), request.HubURL, request.Domain, request.Token)
	if err != nil {
		return nil, err
	}
	domain.CommunicationEndpointPath = snapshot.EffectiveCommunicationEndpointPath()
	domain.CommunicationProtocolVersion = snapshot.EffectiveCommunicationProtocolVersion()
	if err := store.SaveDomain(domain); err != nil {
		return nil, err
	}
	members := make([]model.ClusterMember, 0, len(snapshot.Members))
	memberBaseURLIndexes := map[string]int{}
	for _, item := range snapshot.Members {
		peerTokenEncrypted := ""
		peerToken := item.EffectivePeerToken()
		if peerToken != "" {
			peerTokenEncrypted, err = EncryptClusterDomainToken(secret, peerToken)
			if err != nil {
				return nil, err
			}
		}
		member := model.ClusterMember{
			NodeID:             item.EffectiveNodeID(),
			Name:               item.Name,
			DisplayName:        item.EffectiveDisplayName(),
			BaseURL:            item.EffectiveBaseURL(),
			PublicKey:          item.EffectivePublicKey(),
			PeerTokenEncrypted: peerTokenEncrypted,
			DomainID:           domain.Id,
			LastVersion:        snapshot.Version,
			PanelVersion:       item.EffectivePanelVersion(),
			Status:             item.EffectiveStatus(),
		}
		baseURLKey := normalizeClusterBaseURLForIdentity(member.BaseURL)
		if baseURLKey != "" {
			if existingIndex, exists := memberBaseURLIndexes[baseURLKey]; exists {
				if members[existingIndex].NodeID != identity.NodeID && member.NodeID == identity.NodeID {
					members[existingIndex] = member
				}
				continue
			}
			memberBaseURLIndexes[baseURLKey] = len(members)
		}
		members = append(members, member)
	}
	if err := store.ReplaceDomainMembers(domain.Id, members); err != nil {
		return nil, err
	}
	domain.LastVersion = snapshot.Version
	if err := store.SaveDomain(domain); err != nil {
		return nil, err
	}
	status, err := newClusterOperationStatus("completed", "registered")
	if err != nil {
		return nil, err
	}
	return status, nil
}

func NormalizeClusterRegisterRequest(request *ClusterRegisterRequest) error {
	request.JoinURI = strings.TrimSpace(request.JoinURI)
	if request.JoinURI == "" {
		return nil
	}
	parsed, err := parseClusterHubJoinURI(request.JoinURI)
	if err != nil {
		return err
	}
	request.HubURL = parsed.HubURL
	request.Domain = parsed.Domain
	request.Token = parsed.Token
	return nil
}

type clusterHubJoinURI struct {
	HubURL string
	Domain string
	Token  string
}

func parseClusterHubJoinURI(raw string) (*clusterHubJoinURI, error) {
	if !strings.HasPrefix(strings.ToLower(raw), "buihub://") {
		return nil, errClusterJoinURIInvalid
	}
	withoutScheme := raw[len("buihub://"):]
	if strings.HasPrefix(strings.ToLower(withoutScheme), "http://") || strings.HasPrefix(strings.ToLower(withoutScheme), "https://") {
		return nil, errClusterJoinURIInvalid
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, errClusterJoinURIInvalid
	}
	if parsed.Scheme != "buihub" || parsed.Host == "" || parsed.User != nil {
		return nil, errClusterJoinURIInvalid
	}

	domain, err := clusterJoinURIDomain(parsed)
	if err != nil {
		return nil, err
	}
	token := firstQueryValue(parsed.Query(), "domain_token", "domainToken", "domain-token", "token")
	if token == "" {
		return nil, errClusterTokenRequired
	}
	protocol, err := clusterJoinURIProtocol(parsed)
	if err != nil {
		return nil, err
	}
	return &clusterHubJoinURI{
		HubURL: protocol + "://" + parsed.Host,
		Domain: domain,
		Token:  token,
	}, nil
}

func clusterJoinURIDomain(parsed *url.URL) (string, error) {
	if domainID := firstQueryValue(parsed.Query(), "id"); domainID != "" {
		return strings.TrimSpace(domainID), nil
	}

	path := strings.Trim(parsed.EscapedPath(), "/")
	domainValue := ""
	if path != "" {
		parts := strings.Split(path, "/")
		if len(parts) >= 2 && strings.EqualFold(parts[0], "domain") {
			domainValue = strings.Join(parts[1:], "/")
		} else if len(parts) == 1 && !strings.EqualFold(parts[0], "domain") {
			domainValue = parts[0]
		}
	}
	if domainValue == "" {
		domainValue = firstQueryValue(parsed.Query(), "domain_id", "domainId", "domain")
		if domainValue == "" {
			return "", errClusterDomainRequired
		}
		return strings.TrimSpace(domainValue), nil
	}
	domain, err := url.PathUnescape(domainValue)
	if err != nil {
		return "", errClusterJoinURIInvalid
	}
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return "", errClusterDomainRequired
	}
	return domain, nil
}

func clusterJoinURIProtocol(parsed *url.URL) (string, error) {
	protocol := firstQueryValue(parsed.Query(), "hub_protocol", "protocol")
	if protocol == "" {
		host := parsed.Hostname()
		if isClusterLocalHost(host) {
			return "http", nil
		}
		return "https", nil
	}
	if protocol != "http" && protocol != "https" {
		return "", errClusterJoinURIInvalid
	}
	return protocol, nil
}

func firstQueryValue(values url.Values, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(values.Get(name)); value != "" {
			return value
		}
	}
	return ""
}

func isClusterLocalHost(host string) bool {
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func deriveClusterDisplayNameFromBaseURL(baseURL string) string {
	matches := clusterDisplayNameBaseURLPattern.FindStringSubmatch(strings.TrimSpace(baseURL))
	if len(matches) < 2 {
		return ""
	}
	return strings.ToLower(matches[1])
}

func normalizeClusterBaseURLForIdentity(baseURL string) string {
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return strings.TrimRight(strings.ToLower(trimmed), "/")
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.RawPath = ""
	parsed.Path = strings.TrimRight(parsed.Path, "/")

	host := strings.ToLower(parsed.Hostname())
	port := parsed.Port()
	if port != "" && !((parsed.Scheme == "https" && port == "443") || (parsed.Scheme == "http" && port == "80")) {
		parsed.Host = net.JoinHostPort(host, port)
	} else if strings.Contains(host, ":") {
		parsed.Host = "[" + host + "]"
	} else {
		parsed.Host = host
	}
	return strings.TrimRight(parsed.String(), "/")
}

func effectiveClusterCommunicationEndpointPath(value string) string {
	if strings.TrimSpace(value) == "" {
		return ClusterCommunicationEndpointPath
	}
	return value
}

func effectiveClusterCommunicationProtocolVersion(value string) string {
	if strings.TrimSpace(value) == "" {
		return ClusterCommunicationProtocolVersion
	}
	return value
}

func (s *ClusterService) GetOperation(id string) (*ClusterOperationStatus, error) {
	clusterOperations.mu.RLock()
	status, ok := clusterOperations.items[id]
	clusterOperations.mu.RUnlock()
	if !ok {
		return nil, errors.New("cluster operation not found")
	}
	return &status, nil
}

func (s *ClusterService) ListDomains() ([]ClusterDomainResponse, error) {
	domains, err := s.getStore().ListDomains()
	if err != nil {
		return nil, err
	}
	response := make([]ClusterDomainResponse, 0, len(domains))
	for _, domain := range domains {
		response = append(response, ClusterDomainResponse{
			ID:                           domain.Id,
			Domain:                       domain.Domain,
			HubURL:                       domain.HubURL,
			CommunicationEndpointPath:    effectiveClusterCommunicationEndpointPath(domain.CommunicationEndpointPath),
			CommunicationProtocolVersion: effectiveClusterCommunicationProtocolVersion(domain.CommunicationProtocolVersion),
			LastVersion:                  domain.LastVersion,
			SupportedActions:             ClusterCommunicationSupportedActions(),
		})
	}
	return response, nil
}

func (s *ClusterService) ListMembers() ([]ClusterMemberResponse, error) {
	members, err := s.getStore().ListMembers()
	if err != nil {
		return nil, err
	}
	localIdentity, err := s.localIdentity.GetOrCreate()
	if err != nil {
		return nil, err
	}
	response := make([]ClusterMemberResponse, 0, len(members))
	for _, member := range members {
		response = append(response, ClusterMemberResponse{ID: member.Id, DomainID: member.DomainID, NodeID: member.NodeID, Name: member.Name, DisplayName: member.DisplayName, BaseURL: member.BaseURL, LastVersion: member.LastVersion, IsLocal: member.NodeID == localIdentity.NodeID, PanelVersion: member.PanelVersion, Status: member.Status})
	}
	return response, nil
}

func (s *ClusterService) GetMemberConnection(nodeID string) (*ClusterMemberConnectionResponse, error) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return nil, errClusterMemberNotFound
	}
	member, err := s.getStore().GetMemberByNodeID(nodeID)
	if err != nil {
		return nil, err
	}
	if member.PeerTokenEncrypted == "" {
		return nil, errClusterTokenRequired
	}
	secret, err := s.getSecretProvider().GetSecret()
	if err != nil {
		return nil, err
	}
	token, err := DecryptClusterDomainToken(secret, member.PeerTokenEncrypted)
	if err != nil {
		return nil, err
	}
	return &ClusterMemberConnectionResponse{
		NodeID:      member.NodeID,
		Name:        member.Name,
		DisplayName: member.DisplayName,
		BaseURL:     member.BaseURL,
		Token:       token,
	}, nil
}

func (s *ClusterService) GetMemberInfo(nodeID string) (*clustertypes.InfoResponse, error) {
	connection, err := s.GetMemberConnection(nodeID)
	if err != nil {
		return nil, err
	}
	infoURL, err := clusterPeerActionURL(
		connection.BaseURL,
		ClusterCommunicationEndpointPath,
		ClusterCommunicationProtocolVersion,
		"info",
	)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, infoURL, nil)
	if err != nil {
		return nil, err
	}
	if connection.Token != "" {
		request.Header.Set("X-Cluster-Token", connection.Token)
	}
	response, err := s.peerHTTPClient().Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if err := requireHTTPSuccess(response, "cluster peer info"); err != nil {
		return nil, err
	}
	var info clustertypes.InfoResponse
	if err := json.NewDecoder(response.Body).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}

func (s *ClusterService) SendMemberAction(nodeID string, actionRequest clustertypes.ActionRequest) (*clustertypes.ActionResponse, error) {
	connection, err := s.GetMemberConnection(nodeID)
	if err != nil {
		return nil, err
	}
	actionURL, err := clusterPeerActionURL(
		connection.BaseURL,
		ClusterCommunicationEndpointPath,
		ClusterCommunicationProtocolVersion,
		"action",
	)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(actionRequest)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, actionURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	if connection.Token != "" {
		request.Header.Set("X-Cluster-Token", connection.Token)
	}
	response, err := s.peerHTTPClient().Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if err := requireHTTPSuccess(response, "cluster peer action"); err != nil {
		return nil, err
	}
	var actionResponse clustertypes.ActionResponse
	if err := json.NewDecoder(response.Body).Decode(&actionResponse); err != nil {
		return nil, err
	}
	return &actionResponse, nil
}

func (s *ClusterService) peerHTTPClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

func (s *ClusterService) ManualSync() (*ClusterOperationStatus, error) {
	syncService := s.syncService
	if syncService.store == nil && syncService.hubSyncer == nil {
		syncService = NewRuntimeClusterSyncService()
	}
	if syncService.store == nil {
		syncService.store = &clusterSyncStoreAdapter{store: s.getStore()}
	}
	if syncService.hubSyncer == nil {
		syncService.hubSyncer = s.getHubSyncer()
	}
	if err := syncService.PollAndNotifyVersion(context.Background()); err != nil {
		var removedMirrorErr *clusterDomainMirrorRemovedError
		if errors.As(err, &removedMirrorErr) {
			status, statusErr := newClusterOperationStatus("completed", removedMirrorErr.Error())
			if statusErr != nil {
				return nil, statusErr
			}
			return status, nil
		}
		return nil, err
	}
	status, err := newClusterOperationStatus("completed", "")
	if err != nil {
		return nil, err
	}
	return status, nil
}

func (s *ClusterService) DeleteMember(id uint) error {
	store := s.getStore()
	member, err := store.GetMember(id)
	if err != nil {
		return err
	}
	domain, err := store.GetDomain(member.DomainID)
	if err != nil {
		return err
	}
	if domain.HubURL == "" {
		return errors.New("cluster hub URL not set")
	}
	secret, err := s.getSecretProvider().GetSecret()
	if err != nil {
		return err
	}
	domainToken, err := DecryptClusterDomainToken(secret, domain.TokenEncrypted)
	if err != nil {
		return err
	}
	client := s.getHubClient()
	if _, err := client.DeleteMember(context.Background(), domain.HubURL, domain.Domain, domainToken, member.NodeID); err != nil {
		return err
	}
	return store.DeleteMember(id)
}

func (s *ClusterService) LeaveDomain(id uint) error {
	store := s.getStore()
	domain, err := store.GetDomain(id)
	if err != nil {
		return err
	}
	if domain.HubURL == "" {
		return errors.New("cluster hub URL not set")
	}
	localIdentity, err := s.localIdentity.GetOrCreate()
	if err != nil {
		return err
	}
	secret, err := s.getSecretProvider().GetSecret()
	if err != nil {
		return err
	}
	domainToken, err := DecryptClusterDomainToken(secret, domain.TokenEncrypted)
	if err != nil {
		return err
	}
	client := s.getHubClient()
	if _, err := client.DeleteMember(context.Background(), domain.HubURL, domain.Domain, domainToken, localIdentity.NodeID); err != nil {
		return err
	}
	return store.DeleteDomain(id)
}

func (s *ClusterService) ReceiveMessage(envelope *ClusterEnvelope, token string) error {
	domain, err := s.getStore().GetDomainByName(envelope.Domain)
	if err != nil {
		return err
	}
	localIdentity, err := s.localIdentity.GetOrCreate()
	if err != nil {
		return err
	}
	localMember, err := findClusterMemberByDomainNodeID(s.getStore(), domain.Id, localIdentity.NodeID)
	if err != nil {
		return err
	}
	if localMember == nil {
		return errClusterMemberNotFound
	}
	if err := s.validateClusterPeerToken(localMember, token); err != nil {
		return err
	}
	member, err := findClusterMemberByDomainNodeID(s.getStore(), domain.Id, envelope.SourceNodeID)
	if err != nil {
		return err
	}
	if member == nil {
		return errClusterMemberNotFound
	}
	message, err := VerifyClusterEnvelope(envelope, member.PublicKey)
	if err != nil {
		return err
	}
	if message.Domain != domain.Domain {
		return errors.New("cluster member domain mismatch")
	}
	syncService := s.syncService
	if syncService.store == nil {
		syncService.store = &dbClusterSyncStore{}
		if s.store != nil {
			syncService.store = &clusterSyncStoreAdapter{store: s.store}
		}
	}
	if syncService.hubSyncer == nil {
		syncService.hubSyncer = s.getHubSyncer()
	}
	_, err = syncService.HandleIncomingNotifyVersion(context.Background(), domain.Id, message.SourceNodeID, message.Version)
	if err != nil {
		return err
	}
	domain.LastVersion = message.Version
	return s.getStore().SaveDomain(domain)
}

func (s *ClusterService) Heartbeat(token string) (*ClusterPeerStatus, error) {
	localIdentity, err := s.localIdentity.GetOrCreate()
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	if token == "" {
		return &ClusterPeerStatus{
			Status: "processed",
			Code:   "ok",
			NodeID: localIdentity.NodeID,
			Details: map[string]any{
				"observedAt": now,
			},
		}, nil
	}

	member, domain, err := s.findLocalMemberByPeerToken(token)
	if err != nil {
		return nil, err
	}
	if member == nil || domain == nil {
		return &ClusterPeerStatus{
			Status:  "rejected",
			Code:    "invalid_token",
			Message: "cluster peer token not found",
		}, nil
	}
	return &ClusterPeerStatus{
		Status: "processed",
		Code:   "ok",
		NodeID: localIdentity.NodeID,
		Details: map[string]any{
			"domainId":          domain.Domain,
			"membershipVersion": domain.LastVersion,
			"observedAt":        now,
			"memberId":          member.Id,
		},
	}, nil
}

func (s *ClusterService) Ping(string) (*ClusterPeerStatus, error) {
	localIdentity, err := s.localIdentity.GetOrCreate()
	if err != nil {
		return nil, err
	}
	return &ClusterPeerStatus{
		Status: "processed",
		Code:   "ok",
		NodeID: localIdentity.NodeID,
		Details: map[string]any{
			"observedAt": time.Now().Unix(),
		},
	}, nil
}

func (s *ClusterService) Info(c *gin.Context) {
	r := s.resolvedRouter()
	c.JSON(http.StatusOK, clustertypes.InfoResponse{
		Actions: r.Actions(),
	})
}

func (s *ClusterService) HandleAction(c *gin.Context) {
	var req clustertypes.ActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, clustertypes.ActionResponse{
			Status:       "error",
			ErrorMessage: "invalid request: " + err.Error(),
		})
		return
	}
	r := s.resolvedRouter()
	resp := r.Handle(req)
	c.JSON(http.StatusOK, resp)
}

// SetRuntime sets the runtime with wired handlers. When set, Info and HandleAction
// will use the runtime's router instead of the bare actionRouter.
func (s *ClusterService) SetRuntime(rt *cluster.Runtime) {
	s.runtime = rt
}

// resolvedRouter returns the runtime's router if available, otherwise falls back
// to the bare actionRouter (lazily initialized for backward compatibility).
func (s *ClusterService) resolvedRouter() *router.ActionRouter {
	if s.runtime != nil {
		return s.runtime.Router
	}
	if s.actionRouter == nil {
		s.actionRouter = router.NewActionRouter()
	}
	return s.actionRouter
}

func (s *ClusterService) ReceivePeerMessage(message *PeerMessage, token string) error {
	domain, err := s.getStore().GetDomainByName(message.DomainID)
	if err != nil {
		return err
	}
	localIdentity, err := s.localIdentity.GetOrCreate()
	if err != nil {
		return err
	}
	localMember, err := findClusterMemberByDomainNodeID(s.getStore(), domain.Id, localIdentity.NodeID)
	if err != nil {
		return err
	}
	if localMember == nil {
		return errClusterMemberNotFound
	}
	if err := s.validateClusterPeerToken(localMember, token); err != nil {
		return err
	}
	member, err := findClusterMemberByDomainNodeID(s.getStore(), domain.Id, message.SourceNodeID)
	if err != nil {
		return err
	}
	if member == nil || message.MembershipVersion > domain.LastVersion {
		if err := s.refreshPeerMembership(context.Background(), domain, message.MembershipVersion); err != nil {
			return err
		}
		domain, err = s.getStore().GetDomainByName(message.DomainID)
		if err != nil {
			return err
		}
		member, err = findClusterMemberByDomainNodeID(s.getStore(), domain.Id, message.SourceNodeID)
		if err != nil {
			return err
		}
	}
	if member == nil {
		return errClusterMemberNotFound
	}
	if err := VerifyClusterPeerMessage(message, member.PublicKey, time.Now().Unix()); err != nil {
		return err
	}
	syncService := s.peerSyncService()
	dispatcher := ClusterPeerDispatcher{
		syncService:    &syncService,
		identity:       s.localIdentity,
		secretProvider: s.getSecretProvider(),
	}
	return dispatcher.Dispatch(context.Background(), domain, member, message)
}

func (s *ClusterService) refreshPeerMembership(ctx context.Context, domain *model.ClusterDomain, version int64) error {
	if domain.HubURL == "" {
		return nil
	}
	return s.getHubSyncer().SyncDomain(ctx, domain, version)
}

func (s *ClusterService) peerSyncService() ClusterSyncService {
	syncService := s.syncService
	if syncService.store == nil {
		syncService.store = &dbClusterSyncStore{}
		if s.store != nil {
			syncService.store = &clusterSyncStoreAdapter{store: s.store}
		}
	}
	if syncService.hubSyncer == nil {
		syncService.hubSyncer = s.getHubSyncer()
	}
	return syncService
}

func (s *ClusterService) validateClusterPeerToken(member *model.ClusterMember, token string) error {
	secret, err := s.getSecretProvider().GetSecret()
	if err != nil {
		return err
	}
	decrypted, err := DecryptClusterDomainToken(secret, member.PeerTokenEncrypted)
	if err != nil {
		return err
	}
	if decrypted != token {
		return errors.New("cluster peer token not found")
	}
	return nil
}

func (s *ClusterService) findLocalMemberByPeerToken(token string) (*model.ClusterMember, *model.ClusterDomain, error) {
	localIdentity, err := s.localIdentity.GetOrCreate()
	if err != nil {
		return nil, nil, err
	}
	members, err := s.getStore().ListMembers()
	if err != nil {
		return nil, nil, err
	}
	secret, err := s.getSecretProvider().GetSecret()
	if err != nil {
		return nil, nil, err
	}
	for _, member := range members {
		if member.NodeID != localIdentity.NodeID || member.PeerTokenEncrypted == "" {
			continue
		}
		decrypted, err := DecryptClusterDomainToken(secret, member.PeerTokenEncrypted)
		if err != nil {
			return nil, nil, err
		}
		if decrypted != token {
			continue
		}
		domain, err := s.getStore().GetDomain(member.DomainID)
		if err != nil {
			return nil, nil, err
		}
		copy := member
		return &copy, domain, nil
	}
	return nil, nil, nil
}

func newClusterOperationStatus(state string, message string) (*ClusterOperationStatus, error) {
	id, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}
	status := ClusterOperationStatus{ID: id.String(), State: state, Message: message, createdAt: time.Now()}
	clusterOperations.mu.Lock()
	trimClusterOperationsLocked()
	clusterOperations.items[status.ID] = status
	clusterOperations.mu.Unlock()
	return &status, nil
}

func trimClusterOperationsLocked() {
	if len(clusterOperations.items) < maxClusterOperations {
		return
	}
	type opEntry struct {
		id string
		ts time.Time
	}
	entries := make([]opEntry, 0, len(clusterOperations.items))
	for id, status := range clusterOperations.items {
		entries = append(entries, opEntry{id: id, ts: status.createdAt})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ts.Before(entries[j].ts) })
	removeCount := len(clusterOperations.items) - maxClusterOperations + 1
	for i := 0; i < removeCount && i < len(entries); i++ {
		delete(clusterOperations.items, entries[i].id)
	}
}

type dbClusterSyncStore struct{}

func (s *dbClusterSyncStore) GetMember(domainID uint, nodeID string) (*model.ClusterMember, error) {
	return (&dbClusterStore{}).GetMemberByDomainNodeID(domainID, nodeID)
}

func (s *dbClusterSyncStore) GetMembers(domainID uint) ([]model.ClusterMember, error) {
	var members []model.ClusterMember
	if err := database.GetDB().Where("domain_id = ?", domainID).Find(&members).Error; err != nil {
		return nil, err
	}
	return members, nil
}

func (s *dbClusterSyncStore) SaveMember(member *model.ClusterMember) error {
	return (&dbClusterStore{}).SaveMember(member)
}

func (s *dbClusterSyncStore) ListMembers() ([]model.ClusterMember, error) {
	return (&dbClusterStore{}).ListMembers()
}

func (s *dbClusterSyncStore) GetDomain(id uint) (*model.ClusterDomain, error) {
	return (&dbClusterStore{}).GetDomain(id)
}

func (s *dbClusterSyncStore) SaveDomain(domain *model.ClusterDomain) error {
	return (&dbClusterStore{}).SaveDomain(domain)
}

func (s *dbClusterSyncStore) ListDomains() ([]model.ClusterDomain, error) {
	return (&dbClusterStore{}).ListDomains()
}

type dbClusterStore struct{}

func (s *dbClusterStore) GetDomainByName(name string) (*model.ClusterDomain, error) {
	domain := &model.ClusterDomain{}
	err := database.GetDB().Where("domain = ?", name).First(domain).Error
	if database.IsNotFound(err) {
		return nil, errClusterDomainNotFound
	}
	if err != nil {
		return nil, err
	}
	return domain, nil
}

func (s *dbClusterStore) SaveDomain(domain *model.ClusterDomain) error {
	return database.GetDB().Save(domain).Error
}

func (s *dbClusterStore) ListDomains() ([]model.ClusterDomain, error) {
	var domains []model.ClusterDomain
	if err := database.GetDB().Order("id asc").Find(&domains).Error; err != nil {
		return nil, err
	}
	return domains, nil
}

func (s *dbClusterStore) GetDomain(id uint) (*model.ClusterDomain, error) {
	domain := &model.ClusterDomain{}
	err := database.GetDB().First(domain, id).Error
	if database.IsNotFound(err) {
		return nil, errClusterDomainNotFound
	}
	if err != nil {
		return nil, err
	}
	return domain, nil
}

func (s *dbClusterStore) GetMember(id uint) (*model.ClusterMember, error) {
	member := &model.ClusterMember{}
	err := database.GetDB().First(member, id).Error
	if database.IsNotFound(err) {
		return nil, errClusterMemberNotFound
	}
	if err != nil {
		return nil, err
	}
	return member, nil
}

func (s *dbClusterStore) GetMemberByNodeID(nodeID string) (*model.ClusterMember, error) {
	member := &model.ClusterMember{}
	err := database.GetDB().Where("node_id = ?", nodeID).First(member).Error
	if database.IsNotFound(err) {
		return nil, errClusterMemberNotFound
	}
	if err != nil {
		return nil, err
	}
	return member, nil
}

func (s *dbClusterStore) GetMemberByDomainNodeID(domainID uint, nodeID string) (*model.ClusterMember, error) {
	member := &model.ClusterMember{}
	err := database.GetDB().Where("domain_id = ? AND node_id = ?", domainID, nodeID).First(member).Error
	if database.IsNotFound(err) {
		return nil, errClusterMemberNotFound
	}
	if err != nil {
		return nil, err
	}
	return member, nil
}

func (s *dbClusterStore) SaveMember(member *model.ClusterMember) error {
	return database.GetDB().Save(member).Error
}

func (s *dbClusterStore) ListMembers() ([]model.ClusterMember, error) {
	var members []model.ClusterMember
	if err := database.GetDB().Order("id asc").Find(&members).Error; err != nil {
		return nil, err
	}
	return members, nil
}

func (s *dbClusterStore) DeleteMember(id uint) error {
	return database.GetDB().Delete(&model.ClusterMember{}, id).Error
}

func (s *dbClusterStore) DeleteDomain(id uint) error {
	tx := database.GetDB().Begin()
	if err := tx.Where("domain_id = ?", id).Delete(&model.ClusterMember{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Delete(&model.ClusterDomain{}, id).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func (s *dbClusterStore) ReplaceDomainMembers(domainID uint, members []model.ClusterMember) error {
	tx := database.GetDB().Begin()
	if err := tx.Where("domain_id = ?", domainID).Delete(&model.ClusterMember{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	for i := range members {
		members[i].DomainID = domainID
	}
	if len(members) > 0 {
		if err := tx.Create(&members).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit().Error
}

func (s *ClusterService) getStore() clusterServiceStore {
	if s.store != nil {
		return s.store
	}
	return &dbClusterStore{}
}

func (s *ClusterService) getSecretProvider() clusterSecretProvider {
	if s.secretProvider != nil {
		return s.secretProvider
	}
	return &s.SettingService
}

func (s *ClusterService) getHubClient() clusterHubClient {
	if s.hubClient != nil {
		return s.hubClient
	}
	return &ClusterHubClient{localIdentity: &s.localIdentity}
}

func (s *ClusterService) getHubSyncer() clusterHubSyncer {
	return &ClusterHubSyncer{client: s.getHubClient(), store: s.getStore(), secretProvider: s.getSecretProvider(), localIdentity: &s.localIdentity}
}

func findClusterMemberByDomainNodeID(store clusterServiceStore, domainID uint, nodeID string) (*model.ClusterMember, error) {
	members, err := store.ListMembers()
	if err != nil {
		return nil, err
	}
	for _, member := range members {
		if member.DomainID == domainID && member.NodeID == nodeID {
			copy := member
			return &copy, nil
		}
	}
	return nil, nil
}

type clusterSyncStoreAdapter struct{ store clusterServiceStore }

func (a *clusterSyncStoreAdapter) GetMember(domainID uint, nodeID string) (*model.ClusterMember, error) {
	return a.store.GetMemberByDomainNodeID(domainID, nodeID)
}

func (a *clusterSyncStoreAdapter) GetMembers(domainID uint) ([]model.ClusterMember, error) {
	members, err := a.store.ListMembers()
	if err != nil {
		return nil, err
	}
	var result []model.ClusterMember
	for _, m := range members {
		if m.DomainID == domainID {
			result = append(result, m)
		}
	}
	return result, nil
}

func (a *clusterSyncStoreAdapter) SaveMember(member *model.ClusterMember) error {
	return a.store.SaveMember(member)
}

func (a *clusterSyncStoreAdapter) ListMembers() ([]model.ClusterMember, error) {
	return a.store.ListMembers()
}

func (a *clusterSyncStoreAdapter) GetDomain(id uint) (*model.ClusterDomain, error) {
	return a.store.GetDomain(id)
}

func (a *clusterSyncStoreAdapter) SaveDomain(domain *model.ClusterDomain) error {
	return a.store.SaveDomain(domain)
}

func (a *clusterSyncStoreAdapter) ListDomains() ([]model.ClusterDomain, error) {
	return a.store.ListDomains()
}
