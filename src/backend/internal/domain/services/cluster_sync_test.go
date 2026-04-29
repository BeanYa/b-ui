package service

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
)

func TestClusterMessageEnvelopeAcceptsSignedSyncNotifyVersionV1(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate keypair: %v", err)
	}

	local := &model.ClusterLocalNode{
		NodeID:     "node-local",
		PublicKey:  base64.StdEncoding.EncodeToString(publicKey),
		PrivateKey: base64.StdEncoding.EncodeToString(privateKey),
	}

	envelope, err := SignClusterNotifyVersionEnvelope(local, "edge.example.com", 12, 1700000000)
	if err != nil {
		t.Fatalf("sign envelope: %v", err)
	}

	message, err := VerifyClusterEnvelope(envelope, local.PublicKey)
	if err != nil {
		t.Fatalf("verify envelope: %v", err)
	}
	if message.Version != 12 {
		t.Fatalf("expected notify version 12, got %d", message.Version)
	}
	if message.Domain != "edge.example.com" {
		t.Fatalf("expected domain edge.example.com, got %q", message.Domain)
	}
}

func TestClusterSyncServiceSuppressesDuplicateNotifyVersion(t *testing.T) {
	store := &stubClusterSyncStore{
		members: map[string]*model.ClusterMember{
			stubClusterSyncKey(0, "node-a"): {NodeID: "node-a", DomainID: 0, LastVersion: 7},
		},
	}
	service := &ClusterSyncService{store: store}

	processed, err := service.HandleIncomingNotifyVersion(context.Background(), 0, "node-a", 7)
	if err != nil {
		t.Fatalf("handle duplicate notify version: %v", err)
	}
	if processed {
		t.Fatal("expected duplicate notify version to be suppressed")
	}
}

func TestClusterSyncServiceDoesNotRebroadcastReceivedNotifyVersion(t *testing.T) {
	store := &stubClusterSyncStore{
		domains: map[uint]*model.ClusterDomain{
			1: {Id: 1, Domain: "edge.example.com", HubURL: "https://hub.example.com", LastVersion: 1},
		},
		members: map[string]*model.ClusterMember{
			stubClusterSyncKey(1, "node-a"): {NodeID: "node-a", DomainID: 1, LastVersion: 3},
			stubClusterSyncKey(0, "node-b"): {NodeID: "node-b", DomainID: 0, LastVersion: 1},
		},
	}
	broadcaster := &stubClusterBroadcaster{}
	hub := &stubClusterHubSyncer{}
	service := &ClusterSyncService{store: store, broadcaster: broadcaster, hubSyncer: hub}

	processed, err := service.HandleIncomingNotifyVersion(context.Background(), 1, "node-a", 4)
	if err != nil {
		t.Fatalf("handle notify version: %v", err)
	}
	if !processed {
		t.Fatal("expected fresh notify version to trigger sync")
	}
	if hub.syncCalls != 1 {
		t.Fatalf("expected one hub sync call, got %d", hub.syncCalls)
	}
	if broadcaster.calls != 0 {
		t.Fatalf("expected no rebroadcast for received notify version, got %d", broadcaster.calls)
	}
}

func TestClusterSyncServiceVersionPollIsNoOpWhenHubVersionUnchanged(t *testing.T) {
	store := &stubClusterSyncStore{
		domains: map[uint]*model.ClusterDomain{
			1: {Id: 1, Domain: "edge.example.com", HubURL: "https://hub.example.com", LastVersion: 9},
		},
	}
	hub := &stubClusterHubSyncer{latestVersions: []int64{9, 9}}
	service := &ClusterSyncService{store: store, hubSyncer: hub}

	if err := service.PollAndNotifyVersion(context.Background()); err != nil {
		t.Fatalf("first poll: %v", err)
	}
	if err := service.PollAndNotifyVersion(context.Background()); err != nil {
		t.Fatalf("second poll: %v", err)
	}
	if hub.syncCalls != 0 {
		t.Fatalf("expected unchanged hub version to skip sync, got %d syncs", hub.syncCalls)
	}
	if hub.versionChecks != 2 {
		t.Fatalf("expected two hub version checks, got %d", hub.versionChecks)
	}
}

func TestClusterSyncServiceVersionPollBackfillsMissingMemberDisplayNames(t *testing.T) {
	store := &stubClusterSyncStore{
		domains: map[uint]*model.ClusterDomain{
			1: {Id: 1, Domain: "edge.example.com", HubURL: "https://hub.example.com", LastVersion: 9},
		},
		members: map[string]*model.ClusterMember{
			stubClusterSyncKey(1, "node-a"): {NodeID: "node-a", DomainID: 1, LastVersion: 9},
		},
	}
	hub := &stubClusterHubSyncer{latestVersions: []int64{9}}
	service := &ClusterSyncService{store: store, hubSyncer: hub}

	if err := service.PollAndNotifyVersion(context.Background()); err != nil {
		t.Fatalf("poll and backfill member display names: %v", err)
	}
	if hub.syncCalls != 1 {
		t.Fatalf("expected unchanged hub version to sync missing member display names, got %d syncs", hub.syncCalls)
	}
	if hub.syncedVersions[0] != 9 {
		t.Fatalf("expected synced version 9, got %d", hub.syncedVersions[0])
	}
}

func TestClusterSyncServiceManualPollSyncsFromHubWhenRemoteVersionNewer(t *testing.T) {
	store := &stubClusterSyncStore{
		domains: map[uint]*model.ClusterDomain{
			1: {Id: 1, Domain: "edge.example.com", HubURL: "https://hub.example.com", LastVersion: 2},
		},
	}
	hub := &stubClusterHubSyncer{latestVersions: []int64{5}}
	service := &ClusterSyncService{store: store, hubSyncer: hub}

	if err := service.PollAndNotifyVersion(context.Background()); err != nil {
		t.Fatalf("poll and sync: %v", err)
	}
	if hub.syncCalls != 1 {
		t.Fatalf("expected one hub snapshot sync, got %d", hub.syncCalls)
	}
	if hub.syncedVersions[0] != 5 {
		t.Fatalf("expected synced version 5, got %d", hub.syncedVersions[0])
	}
}

func TestClusterSyncServicePollChecksPanelUpdatesWhenHubVersionUnchanged(t *testing.T) {
	store := &stubClusterSyncStore{
		domains: map[uint]*model.ClusterDomain{
			1: {Id: 1, Domain: "edge.example.com", HubURL: "https://hub.example.com", LastVersion: 9, UpdatePolicy: ClusterDomainUpdatePolicyAuto},
		},
	}
	hub := &stubClusterHubSyncer{latestVersions: []int64{9}}
	panel := &stubClusterPanelUpdater{info: &PanelUpdateInfo{CurrentVersion: "v1.0.0", LatestVersion: "v1.0.0", Comparison: "same"}}
	service := &ClusterSyncService{store: store, hubSyncer: hub, panelService: panel}

	if err := service.PollAndNotifyVersion(context.Background()); err != nil {
		t.Fatalf("poll and check panel update: %v", err)
	}
	if hub.syncCalls != 0 {
		t.Fatalf("expected unchanged hub version to skip snapshot sync, got %d syncs", hub.syncCalls)
	}
	if panel.infoCalls != 1 {
		t.Fatalf("expected panel update check even when hub version is unchanged, got %d checks", panel.infoCalls)
	}
}

func TestClusterSyncServiceManualPolicyBroadcastsUpdateAvailableWithoutStartingUpdate(t *testing.T) {
	secret := []byte("panel-secret-for-cluster-tests")
	store := &stubClusterSyncStore{
		domains: map[uint]*model.ClusterDomain{
			1: {
				Id:             1,
				Domain:         "edge.example.com",
				HubURL:         "https://hub.example.com",
				TokenEncrypted: mustEncryptClusterToken(t, string(secret), "domain-token"),
				UpdatePolicy:   ClusterDomainUpdatePolicyManual,
			},
		},
	}
	panel := &stubClusterPanelUpdater{info: &PanelUpdateInfo{CurrentVersion: "v1.0.0", LatestVersion: "v999.0.0", Comparison: "older", UpdateAvailable: true}}
	broadcaster := &stubClusterBroadcaster{}
	hub := &stubClusterUpdateHubClient{}
	service := &ClusterSyncService{
		store:          store,
		panelService:   panel,
		broadcaster:    broadcaster,
		hubClient:      hub,
		secretProvider: stubClusterSecretProvider{secret: secret},
		localIdentity:  &ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{node: &model.ClusterLocalNode{NodeID: "node-local"}}},
	}

	result, err := service.CheckAndBroadcastUpdate(context.Background(), store.domains[1])
	if err != nil {
		t.Fatalf("manual check update: %v", err)
	}
	if result == nil || !result.UpdateAvailable || result.AutoUpdate {
		t.Fatalf("expected manual update availability without auto update, got %#v", result)
	}
	if panel.startCalls != 0 {
		t.Fatalf("expected manual policy not to start update, got %d starts", panel.startCalls)
	}
	if broadcaster.updateCalls != 1 {
		t.Fatalf("expected one update available broadcast, got %d", broadcaster.updateCalls)
	}
	if hub.claimCalls != 1 {
		t.Fatalf("expected one hub update claim, got %d", hub.claimCalls)
	}
	if saved := store.domains[1]; !saved.PanelUpdateAvailable || saved.LatestPanelVersion != "v999.0.0" {
		t.Fatalf("expected saved update availability, got %#v", saved)
	}
}

func TestClusterSyncServiceAutoPolicyStartsUpdateAndMarksOffline(t *testing.T) {
	secret := []byte("panel-secret-for-cluster-tests")
	store := &stubClusterSyncStore{
		domains: map[uint]*model.ClusterDomain{
			1: {
				Id:             1,
				Domain:         "edge.example.com",
				HubURL:         "https://hub.example.com",
				TokenEncrypted: mustEncryptClusterToken(t, string(secret), "domain-token"),
				UpdatePolicy:   ClusterDomainUpdatePolicyAuto,
			},
		},
	}
	panel := &stubClusterPanelUpdater{info: &PanelUpdateInfo{CurrentVersion: "v1.0.0", LatestVersion: "v999.0.0", Comparison: "older", UpdateAvailable: true}}
	hub := &stubClusterUpdateHubClient{}
	service := &ClusterSyncService{
		store:          store,
		panelService:   panel,
		broadcaster:    &stubClusterBroadcaster{},
		hubClient:      hub,
		secretProvider: stubClusterSecretProvider{secret: secret},
		localIdentity:  &ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{node: &model.ClusterLocalNode{NodeID: "node-local"}}},
	}

	result, err := service.CheckAndBroadcastUpdate(context.Background(), store.domains[1])
	if err != nil {
		t.Fatalf("auto check update: %v", err)
	}
	if result == nil || !result.UpdateAvailable || !result.AutoUpdate || !result.UpdateStarted {
		t.Fatalf("expected automatic update to start, got %#v", result)
	}
	if panel.startCalls != 1 || panel.startedVersions[0] != "v999.0.0" {
		t.Fatalf("expected one automatic update to v999.0.0, got calls=%d versions=%#v", panel.startCalls, panel.startedVersions)
	}
	if hub.setStatusCalls != 1 || hub.lastStatus != "offline" {
		t.Fatalf("expected local member to be marked offline, got calls=%d status=%q", hub.setStatusCalls, hub.lastStatus)
	}
}

func TestClusterSyncServiceAutoPolicyWinsAcrossDomainsWhenHandlingUpdateEvent(t *testing.T) {
	store := &stubClusterSyncStore{
		domains: map[uint]*model.ClusterDomain{
			1: {Id: 1, Domain: "manual.example.com", UpdatePolicy: ClusterDomainUpdatePolicyManual},
			2: {Id: 2, Domain: "auto.example.com", UpdatePolicy: ClusterDomainUpdatePolicyAuto},
		},
	}
	panel := &stubClusterPanelUpdater{}
	service := &ClusterSyncService{store: store, panelService: panel}

	result, err := service.HandlePanelUpdateAvailable(context.Background(), store.domains[1], "v999.0.0")
	if err != nil {
		t.Fatalf("handle update event: %v", err)
	}
	if result == nil || !result.AutoUpdate || !result.UpdateStarted {
		t.Fatalf("expected auto policy in another domain to trigger update, got %#v", result)
	}
	if panel.startCalls != 1 {
		t.Fatalf("expected one automatic update start, got %d", panel.startCalls)
	}
}

func TestClusterSyncServiceUsesDomainScopedMemberLookup(t *testing.T) {
	store := &stubClusterSyncStore{
		domains: map[uint]*model.ClusterDomain{
			2: {Id: 2, Domain: "domain-y", HubURL: "https://hub.example.com", LastVersion: 3},
		},
		members: map[string]*model.ClusterMember{
			stubClusterSyncKey(1, "node-a"): {NodeID: "node-a", DomainID: 1, LastVersion: 1},
			stubClusterSyncKey(2, "node-a"): {NodeID: "node-a", DomainID: 2, LastVersion: 2},
		},
	}
	hub := &stubClusterHubSyncer{}
	service := &ClusterSyncService{store: store, hubSyncer: hub}

	processed, err := service.HandleIncomingNotifyVersion(context.Background(), 2, "node-a", 4)
	if err != nil {
		t.Fatalf("handle notify version: %v", err)
	}
	if !processed {
		t.Fatal("expected notify version to be processed")
	}
	if store.members[stubClusterSyncKey(1, "node-a")].LastVersion != 1 {
		t.Fatalf("expected domain 1 member version to remain 1, got %d", store.members[stubClusterSyncKey(1, "node-a")].LastVersion)
	}
	if store.members[stubClusterSyncKey(2, "node-a")].LastVersion != 4 {
		t.Fatalf("expected domain 2 member version to become 4, got %d", store.members[stubClusterSyncKey(2, "node-a")].LastVersion)
	}
}

type stubClusterSyncStore struct {
	domains map[uint]*model.ClusterDomain
	members map[string]*model.ClusterMember
}

func (s *stubClusterSyncStore) GetMember(domainID uint, nodeID string) (*model.ClusterMember, error) {
	member := s.members[stubClusterSyncKey(domainID, nodeID)]
	if member == nil {
		return nil, errClusterMemberNotFound
	}
	copy := *member
	return &copy, nil
}

func (s *stubClusterSyncStore) GetMembers(domainID uint) ([]model.ClusterMember, error) {
	var result []model.ClusterMember
	for _, member := range s.members {
		if member.DomainID == domainID {
			result = append(result, *member)
		}
	}
	return result, nil
}

func (s *stubClusterSyncStore) SaveMember(member *model.ClusterMember) error {
	copy := *member
	s.members[stubClusterSyncKey(member.DomainID, member.NodeID)] = &copy
	return nil
}

func (s *stubClusterSyncStore) ListMembers() ([]model.ClusterMember, error) {
	members := make([]model.ClusterMember, 0, len(s.members))
	for _, member := range s.members {
		members = append(members, *member)
	}
	return members, nil
}

func (s *stubClusterSyncStore) GetDomain(id uint) (*model.ClusterDomain, error) {
	domain := s.domains[id]
	if domain == nil {
		return nil, errClusterDomainNotFound
	}
	copy := *domain
	return &copy, nil
}

func (s *stubClusterSyncStore) SaveDomain(domain *model.ClusterDomain) error {
	copy := *domain
	s.domains[domain.Id] = &copy
	return nil
}

func (s *stubClusterSyncStore) ListDomains() ([]model.ClusterDomain, error) {
	domains := make([]model.ClusterDomain, 0, len(s.domains))
	for _, domain := range s.domains {
		domains = append(domains, *domain)
	}
	return domains, nil
}

type stubClusterSyncRunner struct {
	calls    int
	nodeIDs  []string
	versions []int64
}

func (s *stubClusterSyncRunner) SyncMember(_ context.Context, nodeID string, version int64) error {
	s.calls++
	s.nodeIDs = append(s.nodeIDs, nodeID)
	s.versions = append(s.versions, version)
	return nil
}

type stubClusterBroadcaster struct {
	calls                int
	versions             []int64
	excludes             []string
	updateCalls          int
	updateTargetVersions []string
	updateDomainIDs      []uint
}

func (s *stubClusterBroadcaster) BroadcastNotifyVersion(_ context.Context, version int64, excludeNodeID string) error {
	s.calls++
	s.versions = append(s.versions, version)
	s.excludes = append(s.excludes, excludeNodeID)
	return nil
}

func (s *stubClusterBroadcaster) BroadcastUpdateAvailable(_ context.Context, domainID uint, _ string, targetVersion string, _ string) error {
	s.updateCalls++
	s.updateDomainIDs = append(s.updateDomainIDs, domainID)
	s.updateTargetVersions = append(s.updateTargetVersions, targetVersion)
	return nil
}

type stubClusterPanelUpdater struct {
	info            *PanelUpdateInfo
	infoErr         error
	infoCalls       int
	startCalls      int
	startedVersions []string
	startedForces   []bool
}

func (s *stubClusterPanelUpdater) GetUpdateInfo() (*PanelUpdateInfo, error) {
	s.infoCalls++
	if s.infoErr != nil {
		return nil, s.infoErr
	}
	if s.info != nil {
		return s.info, nil
	}
	return &PanelUpdateInfo{CurrentVersion: "v1.0.0", LatestVersion: "v999.0.0", Comparison: "older", UpdateAvailable: true}, nil
}

func (s *stubClusterPanelUpdater) StartUpdate(targetVersion string, force bool) (*PanelUpdateStartResult, error) {
	s.startCalls++
	s.startedVersions = append(s.startedVersions, targetVersion)
	s.startedForces = append(s.startedForces, force)
	return &PanelUpdateStartResult{TargetVersion: targetVersion, Force: force}, nil
}

type stubClusterUpdateHubClient struct {
	claimCalls         int
	setStatusCalls     int
	lastClaimTarget    string
	lastStatus         string
	claimResponse      *ClusterHubClaimUpdateResponse
	claimErr           error
	setMemberStatusErr error
}

func (s *stubClusterUpdateHubClient) ClaimUpdate(_ context.Context, _ string, _ string, _ string, _ string, targetVersion string) (*ClusterHubClaimUpdateResponse, error) {
	s.claimCalls++
	s.lastClaimTarget = targetVersion
	if s.claimErr != nil {
		return nil, s.claimErr
	}
	if s.claimResponse != nil {
		return s.claimResponse, nil
	}
	return &ClusterHubClaimUpdateResponse{Proceed: true, TargetVersion: targetVersion}, nil
}

func (s *stubClusterUpdateHubClient) SetMemberStatus(_ context.Context, _ string, _ string, _ string, _ string, _ string, status string, _ string) (*ClusterHubMemberStatusResponse, error) {
	s.setStatusCalls++
	s.lastStatus = status
	if s.setMemberStatusErr != nil {
		return nil, s.setMemberStatusErr
	}
	return &ClusterHubMemberStatusResponse{OK: true}, nil
}

type stubClusterVersionSource struct {
	versions []int64
	index    int
}

type stubClusterHubSyncer struct {
	latestVersions []int64
	versionChecks  int
	syncCalls      int
	syncedVersions []int64
}

func stubClusterSyncKey(domainID uint, nodeID string) string {
	return fmt.Sprintf("%d:%s", domainID, nodeID)
}

func (s *stubClusterHubSyncer) LatestVersion(_ context.Context, _ *model.ClusterDomain) (int64, error) {
	s.versionChecks++
	index := s.versionChecks - 1
	if index >= len(s.latestVersions) {
		index = len(s.latestVersions) - 1
	}
	return s.latestVersions[index], nil
}

func (s *stubClusterHubSyncer) SyncDomain(_ context.Context, _ *model.ClusterDomain, version int64) error {
	s.syncCalls++
	s.syncedVersions = append(s.syncedVersions, version)
	return nil
}

func (s *stubClusterVersionSource) CurrentVersion(context.Context) (int64, error) {
	value := s.versions[s.index]
	if s.index < len(s.versions)-1 {
		s.index++
	}
	return value, nil
}
