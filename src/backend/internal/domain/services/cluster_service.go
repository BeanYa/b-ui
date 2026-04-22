package service

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	database "github.com/alireza0/s-ui/src/backend/internal/infra/db"
	"github.com/alireza0/s-ui/src/backend/internal/infra/db/model"
	"github.com/gofrs/uuid/v5"
)

type ClusterRegisterRequest struct {
	HubURL  string `json:"hubUrl"`
	Name    string `json:"name"`
	Domain  string `json:"domain" binding:"required"`
	Token   string `json:"token" binding:"required"`
	BaseURL string `json:"baseUrl"`
}

type ClusterOperationStatus struct {
	ID        string `json:"id"`
	State     string `json:"state"`
	Message   string `json:"message,omitempty"`
	createdAt time.Time
}

type ClusterDomainResponse struct {
	ID          uint   `json:"id"`
	Domain      string `json:"domain"`
	HubURL      string `json:"hubUrl"`
	LastVersion int64  `json:"lastVersion"`
}

type ClusterMemberResponse struct {
	ID          uint   `json:"id"`
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
	GetMemberByNodeID(string) (*model.ClusterMember, error)
	SaveMember(*model.ClusterMember) error
	ListMembers() ([]model.ClusterMember, error)
	DeleteMember(uint) error
	ReplaceDomainMembers(uint, []model.ClusterMember) error
}

var clusterOperations = struct {
	mu    sync.RWMutex
	items map[string]ClusterOperationStatus
}{items: map[string]ClusterOperationStatus{}}

const maxClusterOperations = 128

func (s *ClusterService) Register(request ClusterRegisterRequest) (*ClusterOperationStatus, error) {
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
	if request.HubURL != "" {
		client := s.hubClient
		if client == nil {
			client = &ClusterHubClient{}
		}
		response, err := client.RegisterNode(context.Background(), request.HubURL, ClusterHubRegisterNodeRequest{
			NodeID:    identity.NodeID,
			Name:      request.Name,
			Domain:    request.Domain,
			PublicKey: identity.PublicKey,
			BaseURL:   request.BaseURL,
		})
		if err != nil {
			return nil, err
		}
		domain.Domain = request.Domain
		domain.HubURL = request.HubURL
		domain.TokenEncrypted = encryptedToken
		if err := store.SaveDomain(domain); err != nil {
			return nil, err
		}
		if response.Member.NodeID != "" {
			member, err := store.GetMemberByNodeID(response.Member.NodeID)
			if err != nil && !errors.Is(err, errClusterMemberNotFound) {
				return nil, err
			}
			if member == nil {
				member = &model.ClusterMember{}
			}
			member.NodeID = response.Member.NodeID
			member.Name = response.Member.Name
			member.BaseURL = response.Member.BaseURL
			member.PublicKey = response.Member.PublicKey
			member.DomainID = domain.Id
			if err := store.SaveMember(member); err != nil {
				return nil, err
			}
		}
	} else {
		domain.Domain = request.Domain
		domain.HubURL = request.HubURL
		domain.TokenEncrypted = encryptedToken
		if err := store.SaveDomain(domain); err != nil {
			return nil, err
		}
	}
	status, err := newClusterOperationStatus("completed", "registered")
	if err != nil {
		return nil, err
	}
	return status, nil
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
		response = append(response, ClusterDomainResponse{ID: domain.Id, Domain: domain.Domain, HubURL: domain.HubURL, LastVersion: domain.LastVersion})
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
		response = append(response, ClusterMemberResponse{ID: member.Id, NodeID: member.NodeID, Name: member.Name, BaseURL: member.BaseURL, LastVersion: member.LastVersion})
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
	return s.getStore().DeleteMember(id)
}

func (s *ClusterService) ReceiveMessage(envelope *ClusterEnvelope, token string) error {
	domain, err := s.findDomainByToken(token)
	if err != nil {
		return err
	}
	member, err := s.getStore().GetMemberByNodeID(envelope.SourceNodeID)
	if err != nil {
		return err
	}
	if member.DomainID != domain.Id {
		return errors.New("cluster member domain mismatch")
	}
	message, err := VerifyClusterEnvelope(envelope, member.PublicKey)
	if err != nil {
		return err
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
	_, err = syncService.HandleIncomingNotifyVersion(context.Background(), message.SourceNodeID, message.Version)
	if err != nil {
		return err
	}
	domain.LastVersion = message.Version
	return s.getStore().SaveDomain(domain)
}

func (s *ClusterService) findDomainByToken(token string) (*model.ClusterDomain, error) {
	secret, err := s.getSecretProvider().GetSecret()
	if err != nil {
		return nil, err
	}
	domains, err := s.getStore().ListDomains()
	if err != nil {
		return nil, err
	}
	for _, domain := range domains {
		decrypted, err := DecryptClusterDomainToken(secret, domain.TokenEncrypted)
		if err == nil && decrypted == token {
			copy := domain
			return &copy, nil
		}
	}
	return nil, errors.New("cluster domain token not found")
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

func (s *dbClusterSyncStore) GetMember(nodeID string) (*model.ClusterMember, error) {
	return (&dbClusterStore{}).GetMemberByNodeID(nodeID)
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
	return &ClusterHubSyncer{client: s.hubClient, store: s.getStore()}
}

type clusterSyncStoreAdapter struct{ store clusterServiceStore }

func (a *clusterSyncStoreAdapter) GetMember(nodeID string) (*model.ClusterMember, error) {
	return a.store.GetMemberByNodeID(nodeID)
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
