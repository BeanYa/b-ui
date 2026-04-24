package service

import (
	"context"
	"errors"
	"net"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	database "github.com/alireza0/s-ui/src/backend/internal/infra/db"
	"github.com/alireza0/s-ui/src/backend/internal/infra/db/model"
	"github.com/gofrs/uuid/v5"
)

type ClusterRegisterRequest struct {
	JoinURI string `json:"joinUri" form:"joinUri"`
	HubURL  string `json:"hubUrl" form:"hubUrl"`
	Name    string `json:"name" form:"name"`
	Domain  string `json:"domain" form:"domain"`
	Token   string `json:"token" form:"token"`
	BaseURL string `json:"baseUrl" form:"baseUrl"`
}

type ClusterOperationStatus struct {
	ID        string `json:"id"`
	State     string `json:"state"`
	Message   string `json:"message,omitempty"`
	createdAt time.Time
}

type ClusterDomainResponse struct {
	ID                           uint   `json:"id"`
	Domain                       string `json:"domain"`
	HubURL                       string `json:"hubUrl"`
	CommunicationEndpointPath    string `json:"communicationEndpointPath"`
	CommunicationProtocolVersion string `json:"communicationProtocolVersion"`
	LastVersion                  int64  `json:"lastVersion"`
}

type ClusterMemberResponse struct {
	ID          uint   `json:"id"`
	DomainID    uint   `json:"domainId"`
	NodeID      string `json:"nodeId"`
	Name        string `json:"name"`
	BaseURL     string `json:"baseUrl"`
	LastVersion int64  `json:"lastVersion"`
}

type ClusterService struct {
	SettingService
	localIdentity  ClusterLocalIdentityService
	syncService    ClusterSyncService
	hubClient      clusterHubClient
	store          clusterServiceStore
	secretProvider clusterSecretProvider
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

func (s *ClusterService) Register(request ClusterRegisterRequest) (*ClusterOperationStatus, error) {
	if err := NormalizeClusterRegisterRequest(&request); err != nil {
		return nil, err
	}
	request.Domain = strings.TrimSpace(request.Domain)
	request.HubURL = strings.TrimSpace(request.HubURL)
	request.BaseURL = strings.TrimSpace(request.BaseURL)
	request.Name = strings.TrimSpace(request.Name)
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
	client := s.hubClient
	if client == nil {
		client = &ClusterHubClient{}
	}
	requestID, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}
	_, err = client.RegisterNode(context.Background(), request.HubURL, ClusterHubRegisterNodeRequest{
		RequestID:   requestID.String(),
		DomainID:    request.Domain,
		DomainToken: request.Token,
		Member: ClusterHubMemberRegister{
			MemberID:  identity.NodeID,
			NodeID:    identity.NodeID,
			Address:   request.BaseURL,
			BaseURL:   request.BaseURL,
			PublicKey: identity.PublicKey,
			Name:      request.Name,
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
	for _, item := range snapshot.Members {
		peerTokenEncrypted := ""
		peerToken := item.EffectivePeerToken()
		if peerToken != "" {
			peerTokenEncrypted, err = EncryptClusterDomainToken(secret, peerToken)
			if err != nil {
				return nil, err
			}
		}
		members = append(members, model.ClusterMember{
			NodeID:             item.EffectiveNodeID(),
			Name:               item.Name,
			BaseURL:            item.EffectiveBaseURL(),
			PublicKey:          item.EffectivePublicKey(),
			PeerTokenEncrypted: peerTokenEncrypted,
			DomainID:           domain.Id,
			LastVersion:        snapshot.Version,
		})
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
	path := strings.Trim(parsed.EscapedPath(), "/")
	domainValue := ""
	if path != "" {
		parts := strings.Split(path, "/")
		if len(parts) >= 2 && strings.EqualFold(parts[0], "domain") {
			domainValue = strings.Join(parts[1:], "/")
		} else if len(parts) == 1 {
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
		})
	}
	return response, nil
}

func (s *ClusterService) ListMembers() ([]ClusterMemberResponse, error) {
	members, err := s.getStore().ListMembers()
	if err != nil {
		return nil, err
	}
	response := make([]ClusterMemberResponse, 0, len(members))
	for _, member := range members {
		response = append(response, ClusterMemberResponse{ID: member.Id, DomainID: member.DomainID, NodeID: member.NodeID, Name: member.Name, BaseURL: member.BaseURL, LastVersion: member.LastVersion})
	}
	return response, nil
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
		return nil, err
	}
	status, err := newClusterOperationStatus("completed", "sync triggered")
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
	client := s.hubClient
	if client == nil {
		client = &ClusterHubClient{}
	}
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
	client := s.hubClient
	if client == nil {
		client = &ClusterHubClient{}
	}
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
	if member == nil {
		return errClusterMemberNotFound
	}
	if err := VerifyClusterPeerMessage(message, member.PublicKey, time.Now().Unix()); err != nil {
		return err
	}
	syncService := s.peerSyncService()
	dispatcher := ClusterPeerDispatcher{syncService: &syncService}
	return dispatcher.Dispatch(context.Background(), domain, member, message)
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

func (s *ClusterService) getHubSyncer() clusterHubSyncer {
	return &ClusterHubSyncer{client: s.hubClient, store: s.getStore(), secretProvider: s.getSecretProvider()}
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
