package service

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/BeanYa/b-ui/src/backend/internal/domain/config"
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

func TestClusterSyncServiceSyncNowRefreshesSnapshotWhenHubVersionUnchanged(t *testing.T) {
	store := &stubClusterSyncStore{
		domains: map[uint]*model.ClusterDomain{
			1: {Id: 1, Domain: "edge.example.com", HubURL: "https://hub.example.com", LastVersion: 9},
		},
	}
	hub := &stubClusterHubSyncer{latestVersions: []int64{9, 9}}
	service := &ClusterSyncService{store: store, hubSyncer: hub}

	if err := service.SyncNow(context.Background()); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	if err := service.SyncNow(context.Background()); err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if hub.syncCalls != 2 {
		t.Fatalf("expected explicit sync to refresh unchanged hub version, got %d syncs", hub.syncCalls)
	}
	if hub.versionChecks != 2 {
		t.Fatalf("expected two hub version checks, got %d", hub.versionChecks)
	}
}

func TestClusterSyncServiceSyncNowBackfillsMissingMemberDisplayNames(t *testing.T) {
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

	if err := service.SyncNow(context.Background()); err != nil {
		t.Fatalf("sync and backfill member display names: %v", err)
	}
	if hub.syncCalls != 1 {
		t.Fatalf("expected unchanged hub version to sync missing member display names, got %d syncs", hub.syncCalls)
	}
	if hub.syncedVersions[0] != 9 {
		t.Fatalf("expected synced version 9, got %d", hub.syncedVersions[0])
	}
}

func TestClusterSyncServiceSyncNowSyncsFromHubWhenRemoteVersionNewer(t *testing.T) {
	store := &stubClusterSyncStore{
		domains: map[uint]*model.ClusterDomain{
			1: {Id: 1, Domain: "edge.example.com", HubURL: "https://hub.example.com", LastVersion: 2},
		},
	}
	hub := &stubClusterHubSyncer{latestVersions: []int64{5}}
	service := &ClusterSyncService{store: store, hubSyncer: hub}

	if err := service.SyncNow(context.Background()); err != nil {
		t.Fatalf("sync now: %v", err)
	}
	if hub.syncCalls != 1 {
		t.Fatalf("expected one hub snapshot sync, got %d", hub.syncCalls)
	}
	if hub.syncedVersions[0] != 5 {
		t.Fatalf("expected synced version 5, got %d", hub.syncedVersions[0])
	}
}

func TestClusterSyncServiceSyncNowRefreshesSnapshotAndChecksPanelUpdatesWhenHubVersionUnchanged(t *testing.T) {
	store := &stubClusterSyncStore{
		domains: map[uint]*model.ClusterDomain{
			1: {Id: 1, Domain: "edge.example.com", HubURL: "https://hub.example.com", LastVersion: 9, UpdatePolicy: ClusterDomainUpdatePolicyAuto},
		},
	}
	hub := &stubClusterHubSyncer{latestVersions: []int64{9}}
	panel := &stubClusterPanelUpdater{info: &PanelUpdateInfo{CurrentVersion: "v1.0.0", LatestVersion: "v1.0.0", Comparison: "same"}}
	service := &ClusterSyncService{store: store, hubSyncer: hub, panelService: panel}

	if err := service.SyncNow(context.Background()); err != nil {
		t.Fatalf("sync and check panel update: %v", err)
	}
	if hub.syncCalls != 1 {
		t.Fatalf("expected explicit sync to refresh unchanged hub version, got %d syncs", hub.syncCalls)
	}
	if panel.infoCalls != 1 {
		t.Fatalf("expected panel update check even when hub version is unchanged, got %d checks", panel.infoCalls)
	}
}

func TestClusterSyncServiceSyncNowRefreshesSnapshotWhenMemberIsUpdating(t *testing.T) {
	store := &stubClusterSyncStore{
		domains: map[uint]*model.ClusterDomain{
			1: {Id: 1, Domain: "edge.example.com", HubURL: "https://hub.example.com", LastVersion: 9},
		},
		members: map[string]*model.ClusterMember{
			stubClusterSyncKey(1, "node-local"): {NodeID: "node-local", DomainID: 1, DisplayName: "local", LastVersion: 9, Status: "online"},
			stubClusterSyncKey(1, "node-peer"):  {NodeID: "node-peer", DomainID: 1, DisplayName: "peer", LastVersion: 9, Status: "updating", PanelVersion: "v1.0.0"},
		},
	}
	hub := &stubClusterHubSyncer{latestVersions: []int64{9}}
	panel := &stubClusterPanelUpdater{info: &PanelUpdateInfo{CurrentVersion: "v1.0.0", LatestVersion: "v1.0.0", Comparison: "same"}}
	service := &ClusterSyncService{store: store, hubSyncer: hub, panelService: panel}

	if err := service.SyncNow(context.Background()); err != nil {
		t.Fatalf("sync and refresh updating member: %v", err)
	}
	if hub.syncCalls != 1 {
		t.Fatalf("expected unchanged hub version to refresh snapshot for updating member, got %d syncs", hub.syncCalls)
	}
	if hub.syncedVersions[0] != 9 {
		t.Fatalf("expected synced version 9, got %d", hub.syncedVersions[0])
	}
}

func TestClusterSyncServiceManualPolicyRecordsUpdateAvailableWithoutStartingUpdate(t *testing.T) {
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
	if broadcaster.updateCalls != 0 {
		t.Fatalf("expected update availability checks not to broadcast, got %d broadcasts", broadcaster.updateCalls)
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
	if broadcaster.updateCalls != 0 {
		t.Fatalf("expected automatic update checks not to broadcast availability, got %d broadcasts", broadcaster.updateCalls)
	}
	if broadcaster.statusCalls != 1 || broadcaster.statuses[0] != ClusterPanelUpdateStatusUpdating {
		t.Fatalf("expected automatic update to keep status broadcasts, got %#v", broadcaster)
	}
}

func TestClusterSyncServiceCheckAutoPanelUpdatesOnlyChecksAutoDomains(t *testing.T) {
	store := &stubClusterSyncStore{
		domains: map[uint]*model.ClusterDomain{
			1: {Id: 1, Domain: "manual.example.com", UpdatePolicy: ClusterDomainUpdatePolicyManual},
		},
	}
	panel := &stubClusterPanelUpdater{info: &PanelUpdateInfo{CurrentVersion: "v1.0.0", LatestVersion: "v999.0.0", Comparison: "older", UpdateAvailable: true}}
	service := &ClusterSyncService{store: store, panelService: panel}

	if err := service.CheckAutoPanelUpdates(context.Background()); err != nil {
		t.Fatalf("check auto panel updates: %v", err)
	}
	if panel.infoCalls != 0 {
		t.Fatalf("expected manual-only domains not to check panel updates, got %d checks", panel.infoCalls)
	}
}

func TestClusterSyncServiceCheckAutoPanelUpdatesChecksAtMostOncePerRun(t *testing.T) {
	store := &stubClusterSyncStore{
		domains: map[uint]*model.ClusterDomain{
			1: {Id: 1, Domain: "auto-a.example.com", UpdatePolicy: ClusterDomainUpdatePolicyAuto},
			2: {Id: 2, Domain: "auto-b.example.com", UpdatePolicy: ClusterDomainUpdatePolicyAuto},
		},
	}
	panel := &stubClusterPanelUpdater{info: &PanelUpdateInfo{CurrentVersion: "v1.0.0", LatestVersion: "v1.0.0", Comparison: "same"}}
	service := &ClusterSyncService{store: store, panelService: panel}

	if err := service.CheckAutoPanelUpdates(context.Background()); err != nil {
		t.Fatalf("check auto panel updates: %v", err)
	}
	if panel.infoCalls != 1 {
		t.Fatalf("expected one panel update check per cron run, got %d", panel.infoCalls)
	}
	if store.domains[1].PanelUpdateAvailable || store.domains[2].PanelUpdateAvailable {
		t.Fatalf("expected cached result to update both auto domains, got a=%#v b=%#v", store.domains[1], store.domains[2])
	}
}

func TestClusterSyncServiceCheckAutoPanelUpdatesChecksAtMostOnceWhenUpdateInfoFails(t *testing.T) {
	store := &stubClusterSyncStore{
		domains: map[uint]*model.ClusterDomain{
			1: {Id: 1, Domain: "auto-a.example.com", UpdatePolicy: ClusterDomainUpdatePolicyAuto},
			2: {Id: 2, Domain: "auto-b.example.com", UpdatePolicy: ClusterDomainUpdatePolicyAuto},
		},
	}
	panel := &stubClusterPanelUpdater{infoErr: errors.New("release check failed")}
	service := &ClusterSyncService{store: store, panelService: panel}

	if err := service.CheckAutoPanelUpdates(context.Background()); err == nil {
		t.Fatal("expected update check error")
	}
	if panel.infoCalls != 1 {
		t.Fatalf("expected one failed panel update check per cron run, got %d", panel.infoCalls)
	}
}

func TestClusterSyncServiceCheckAutoPanelUpdatesDoesNotClaimHubUpdate(t *testing.T) {
	secret := []byte("panel-secret-for-cluster-tests")
	store := &stubClusterSyncStore{
		domains: map[uint]*model.ClusterDomain{
			1: {
				Id:             1,
				Domain:         "auto-a.example.com",
				HubURL:         "https://hub-a.example.com",
				TokenEncrypted: mustEncryptClusterToken(t, string(secret), "domain-token-a"),
				UpdatePolicy:   ClusterDomainUpdatePolicyAuto,
			},
			2: {
				Id:             2,
				Domain:         "auto-b.example.com",
				HubURL:         "https://hub-b.example.com",
				TokenEncrypted: mustEncryptClusterToken(t, string(secret), "domain-token-b"),
				UpdatePolicy:   ClusterDomainUpdatePolicyAuto,
			},
		},
		members: map[string]*model.ClusterMember{
			stubClusterSyncKey(1, "node-local"): {NodeID: "node-local", DomainID: 1, PanelVersion: "v1.0.0", Status: "online"},
			stubClusterSyncKey(2, "node-local"): {NodeID: "node-local", DomainID: 2, PanelVersion: "v1.0.0", Status: "online"},
		},
	}
	panel := &stubClusterPanelUpdater{info: &PanelUpdateInfo{CurrentVersion: "v1.0.0", LatestVersion: "v999.0.0", Comparison: "older", UpdateAvailable: true}}
	hub := &stubClusterUpdateHubClient{claimErr: errors.New("hub claim should not be called")}
	service := &ClusterSyncService{
		store:          store,
		panelService:   panel,
		broadcaster:    &stubClusterBroadcaster{},
		hubClient:      hub,
		secretProvider: stubClusterSecretProvider{secret: secret},
		localIdentity:  &ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{node: &model.ClusterLocalNode{NodeID: "node-local"}}},
	}

	if err := service.CheckAutoPanelUpdates(context.Background()); err != nil {
		t.Fatalf("check auto panel updates: %v", err)
	}
	if panel.infoCalls != 1 {
		t.Fatalf("expected one panel update check shared across domains, got %d", panel.infoCalls)
	}
	if hub.claimCalls != 0 {
		t.Fatalf("expected auto check not to claim hub update, got %d claims", hub.claimCalls)
	}
	if panel.startCalls != 1 {
		t.Fatalf("expected auto check to start local update from GitHub result, got %d starts", panel.startCalls)
	}
}

func TestClusterSyncServiceCheckAutoPanelUpdatesStartsUpdateWithoutAvailableBroadcast(t *testing.T) {
	secret := []byte("panel-secret-for-cluster-tests")
	store := &stubClusterSyncStore{
		domains: map[uint]*model.ClusterDomain{
			1: {
				Id:             1,
				Domain:         "auto.example.com",
				HubURL:         "https://hub.example.com",
				TokenEncrypted: mustEncryptClusterToken(t, string(secret), "domain-token"),
				UpdatePolicy:   ClusterDomainUpdatePolicyAuto,
			},
		},
		members: map[string]*model.ClusterMember{
			stubClusterSyncKey(1, "node-local"): {NodeID: "node-local", DomainID: 1, PanelVersion: "v1.0.0", Status: "online"},
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

	if err := service.CheckAutoPanelUpdates(context.Background()); err != nil {
		t.Fatalf("check auto panel updates: %v", err)
	}
	if panel.infoCalls != 1 {
		t.Fatalf("expected one local panel update check, got %d", panel.infoCalls)
	}
	if panel.startCalls != 1 || panel.startedVersions[0] != "v999.0.0" {
		t.Fatalf("expected automatic update to v999.0.0, got calls=%d versions=%#v", panel.startCalls, panel.startedVersions)
	}
	if broadcaster.updateCalls != 0 {
		t.Fatalf("expected no update available broadcast, got %d", broadcaster.updateCalls)
	}
	if broadcaster.statusCalls != 1 || broadcaster.statuses[0] != ClusterPanelUpdateStatusUpdating {
		t.Fatalf("expected updating status broadcast to remain, got %#v", broadcaster)
	}
}

func TestClusterSyncServiceAutoPolicyWinsAcrossDomainsWhenHandlingUpdateEvent(t *testing.T) {
	store := &stubClusterSyncStore{
		domains: map[uint]*model.ClusterDomain{
			1: {Id: 1, Domain: "manual.example.com", UpdatePolicy: ClusterDomainUpdatePolicyManual},
			2: {Id: 2, Domain: "auto.example.com", UpdatePolicy: ClusterDomainUpdatePolicyAuto},
		},
		members: map[string]*model.ClusterMember{
			stubClusterSyncKey(1, "node-local"): {NodeID: "node-local", DomainID: 1, PanelVersion: "v1.0.0", Status: "online"},
		},
	}
	panel := &stubClusterPanelUpdater{}
	service := &ClusterSyncService{
		store:         store,
		panelService:  panel,
		broadcaster:   &stubClusterBroadcaster{},
		localIdentity: &ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{node: &model.ClusterLocalNode{NodeID: "node-local"}}},
	}

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

func TestClusterSyncServiceManualUpdateRequestMarksUpdatingAndBroadcastsStatus(t *testing.T) {
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
		members: map[string]*model.ClusterMember{
			stubClusterSyncKey(1, "node-local"): {NodeID: "node-local", DomainID: 1, PanelVersion: "v1.0.0", Status: "online"},
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

	result, err := service.HandlePanelUpdateRequest(context.Background(), store.domains[1], "v999.0.0")
	if err != nil {
		t.Fatalf("handle panel update request: %v", err)
	}
	if result == nil || !result.UpdateAvailable || !result.UpdateStarted {
		t.Fatalf("expected update request to start, got %#v", result)
	}
	if panel.startCalls != 1 || panel.startedVersions[0] != "v999.0.0" || panel.startedForces[0] {
		t.Fatalf("expected one non-force update start to v999.0.0, got calls=%d versions=%#v forces=%#v", panel.startCalls, panel.startedVersions, panel.startedForces)
	}
	if hub.setStatusCalls != 1 || hub.lastStatus != "offline" {
		t.Fatalf("expected hub offline status before update, got calls=%d status=%q", hub.setStatusCalls, hub.lastStatus)
	}
	if member := store.members[stubClusterSyncKey(1, "node-local")]; member.Status != "updating" {
		t.Fatalf("expected local member status updating, got %#v", member)
	}
	if broadcaster.statusCalls != 1 || broadcaster.statuses[0] != "updating" || broadcaster.statusTargetVersions[0] != "v999.0.0" {
		t.Fatalf("expected updating status broadcast, got %#v", broadcaster)
	}
}

func TestClusterSyncServicePanelUpdateStatusUpdatesSourceMember(t *testing.T) {
	store := &stubClusterSyncStore{
		domains: map[uint]*model.ClusterDomain{
			1: {Id: 1, Domain: "edge.example.com"},
		},
		members: map[string]*model.ClusterMember{
			stubClusterSyncKey(1, "node-peer"): {NodeID: "node-peer", DomainID: 1, PanelVersion: "v1.0.0", Status: "online"},
		},
	}
	service := &ClusterSyncService{store: store}

	if err := service.HandlePanelUpdateStatus(context.Background(), store.domains[1], "node-peer", "updating", "v999.0.0", "v1.0.0"); err != nil {
		t.Fatalf("handle updating status: %v", err)
	}
	if member := store.members[stubClusterSyncKey(1, "node-peer")]; member.Status != "updating" || member.PanelVersion != "v1.0.0" {
		t.Fatalf("expected updating member with original version, got %#v", member)
	}

	if err := service.HandlePanelUpdateStatus(context.Background(), store.domains[1], "node-peer", "online", "v999.0.0", "v999.0.0"); err != nil {
		t.Fatalf("handle online status: %v", err)
	}
	if member := store.members[stubClusterSyncKey(1, "node-peer")]; member.Status != "online" || member.PanelVersion != "v999.0.0" {
		t.Fatalf("expected online member with updated version, got %#v", member)
	}
}

func TestClusterSyncServiceReportsLocalPanelStatusAfterRestart(t *testing.T) {
	stateDir := t.TempDir()
	originalStatePath := panelUpdateStateFilePath
	panelUpdateStateFilePath = func() string {
		return filepath.Join(stateDir, "panel-update-state.json")
	}
	t.Cleanup(func() {
		panelUpdateStateFilePath = originalStatePath
	})

	currentVersion := canonicalizeReleaseTag(config.GetVersion())
	secret := []byte("panel-secret-for-cluster-tests")
	store := &stubClusterSyncStore{
		domains: map[uint]*model.ClusterDomain{
			1: {
				Id:             1,
				Domain:         "edge.example.com",
				HubURL:         "https://hub.example.com",
				TokenEncrypted: mustEncryptClusterToken(t, string(secret), "domain-token"),
			},
		},
		members: map[string]*model.ClusterMember{
			stubClusterSyncKey(1, "node-local"): {NodeID: "node-local", DomainID: 1, PanelVersion: "v0.1.61", Status: "offline"},
			stubClusterSyncKey(1, "node-peer"):  {NodeID: "node-peer", DomainID: 1, PanelVersion: "v0.1.61", Status: "offline"},
		},
	}
	broadcaster := &stubClusterBroadcaster{}
	hub := &stubClusterUpdateHubClient{}
	service := &ClusterSyncService{
		store:          store,
		broadcaster:    broadcaster,
		hubClient:      hub,
		secretProvider: stubClusterSecretProvider{secret: secret},
		localIdentity:  &ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{node: &model.ClusterLocalNode{NodeID: "node-local"}}},
	}

	if err := service.ReportLocalPanelStatus(context.Background()); err != nil {
		t.Fatalf("report local panel status: %v", err)
	}

	member := store.members[stubClusterSyncKey(1, "node-local")]
	if member.Status != ClusterPanelUpdateStatusOnline || member.PanelVersion != currentVersion {
		t.Fatalf("expected local member online with current version %s, got %#v", currentVersion, member)
	}
	if hub.setStatusCalls != 1 || hub.lastStatus != ClusterPanelUpdateStatusOnline || hub.lastPanelVersion != currentVersion {
		t.Fatalf("expected hub online status report with current version %s, got %#v", currentVersion, hub)
	}
	if broadcaster.statusCalls != 1 || broadcaster.statuses[0] != ClusterPanelUpdateStatusOnline || broadcaster.statusPanelVersions[0] != currentVersion {
		t.Fatalf("expected peer online broadcast with current version, got %#v", broadcaster)
	}
}

func TestClusterSyncServiceSkipsPanelStatusReportWithoutDomains(t *testing.T) {
	stateDir := t.TempDir()
	originalStatePath := panelUpdateStateFilePath
	panelUpdateStateFilePath = func() string {
		return filepath.Join(stateDir, "panel-update-state.json")
	}
	t.Cleanup(func() {
		panelUpdateStateFilePath = originalStatePath
	})

	store := &stubClusterSyncStore{
		domains: map[uint]*model.ClusterDomain{},
		members: map[string]*model.ClusterMember{},
	}
	identity := &stubClusterLocalIdentityProvider{}
	broadcaster := &stubClusterBroadcaster{}
	hub := &stubClusterUpdateHubClient{}
	service := &ClusterSyncService{
		store:         store,
		broadcaster:   broadcaster,
		hubClient:     hub,
		localIdentity: identity,
	}

	if err := service.ReportLocalPanelStatus(context.Background()); err != nil {
		t.Fatalf("report local panel status without domains: %v", err)
	}
	if identity.calls != 0 {
		t.Fatalf("expected no local identity access without domains, got %d calls", identity.calls)
	}
	if hub.setStatusCalls != 0 {
		t.Fatalf("expected no hub status report without domains, got %d calls", hub.setStatusCalls)
	}
	if broadcaster.statusCalls != 0 {
		t.Fatalf("expected no peer broadcast without domains, got %d calls", broadcaster.statusCalls)
	}
}

func TestClusterSyncServiceSkipsPanelStatusReportWithoutNamedDomains(t *testing.T) {
	stateDir := t.TempDir()
	originalStatePath := panelUpdateStateFilePath
	panelUpdateStateFilePath = func() string {
		return filepath.Join(stateDir, "panel-update-state.json")
	}
	t.Cleanup(func() {
		panelUpdateStateFilePath = originalStatePath
	})

	store := &stubClusterSyncStore{
		domains: map[uint]*model.ClusterDomain{
			1: {Id: 1},
		},
		members: map[string]*model.ClusterMember{},
	}
	identity := &stubClusterLocalIdentityProvider{}
	service := &ClusterSyncService{
		store:         store,
		localIdentity: identity,
	}

	if err := service.ReportLocalPanelStatus(context.Background()); err != nil {
		t.Fatalf("report local panel status without named domains: %v", err)
	}
	if identity.calls != 0 {
		t.Fatalf("expected no local identity access without named domains, got %d calls", identity.calls)
	}
}

func TestClusterSyncServiceSkipsPanelStatusReportWithoutLocalMember(t *testing.T) {
	stateDir := t.TempDir()
	originalStatePath := panelUpdateStateFilePath
	panelUpdateStateFilePath = func() string {
		return filepath.Join(stateDir, "panel-update-state.json")
	}
	t.Cleanup(func() {
		panelUpdateStateFilePath = originalStatePath
	})

	secret := []byte("panel-secret-for-cluster-tests")
	store := &stubClusterSyncStore{
		domains: map[uint]*model.ClusterDomain{
			1: {
				Id:             1,
				Domain:         "edge.example.com",
				HubURL:         "https://hub.example.com",
				TokenEncrypted: mustEncryptClusterToken(t, string(secret), "domain-token"),
			},
		},
		members: map[string]*model.ClusterMember{
			stubClusterSyncKey(1, "node-peer"): {NodeID: "node-peer", DomainID: 1, PanelVersion: "v0.1.61", Status: "offline"},
		},
	}
	broadcaster := &stubClusterBroadcaster{}
	hub := &stubClusterUpdateHubClient{}
	service := &ClusterSyncService{
		store:          store,
		broadcaster:    broadcaster,
		hubClient:      hub,
		secretProvider: stubClusterSecretProvider{secret: secret},
		localIdentity:  &stubClusterLocalIdentityProvider{node: &model.ClusterLocalNode{NodeID: "node-local"}},
	}

	if err := service.ReportLocalPanelStatus(context.Background()); err != nil {
		t.Fatalf("report local panel status without local member: %v", err)
	}
	if hub.setStatusCalls != 0 {
		t.Fatalf("expected no hub status report without local member, got %d calls", hub.setStatusCalls)
	}
	if broadcaster.statusCalls != 0 {
		t.Fatalf("expected no peer broadcast without local member, got %d calls", broadcaster.statusCalls)
	}
}

func TestSyncMemberProviderSkipsIdentityWithoutDomains(t *testing.T) {
	store := &stubClusterSyncStore{
		domains: map[uint]*model.ClusterDomain{},
		members: map[string]*model.ClusterMember{},
	}
	identity := &stubClusterLocalIdentityProvider{}
	provider := &syncMemberProvider{
		store:          store,
		secretProvider: stubClusterSecretProvider{secret: []byte("panel-secret-for-cluster-tests")},
		localIdentity:  identity,
	}

	domains, err := provider.GetAllDomains()
	if err != nil {
		t.Fatalf("get domains: %v", err)
	}
	if len(domains) != 0 {
		t.Fatalf("expected no domains, got %#v", domains)
	}
	if identity.calls != 0 {
		t.Fatalf("expected no local identity access without domains, got %d calls", identity.calls)
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
	statusCalls          int
	statuses             []string
	statusTargetVersions []string
	statusPanelVersions  []string
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

func (s *stubClusterBroadcaster) BroadcastUpdateStatus(_ context.Context, _ uint, _ string, status string, targetVersion string, panelVersion string, _ string) error {
	s.statusCalls++
	s.statuses = append(s.statuses, status)
	s.statusTargetVersions = append(s.statusTargetVersions, targetVersion)
	s.statusPanelVersions = append(s.statusPanelVersions, panelVersion)
	return nil
}

type stubClusterLocalIdentityProvider struct {
	calls int
	node  *model.ClusterLocalNode
	err   error
}

func (s *stubClusterLocalIdentityProvider) GetOrCreate() (*model.ClusterLocalNode, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	if s.node != nil {
		return s.node, nil
	}
	return &model.ClusterLocalNode{NodeID: "node-local"}, nil
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
	lastPanelVersion   string
	claimResponse      *ClusterHubClaimUpdateResponse
	claimResponses     []*ClusterHubClaimUpdateResponse
	claimErr           error
	setMemberStatusErr error
}

func (s *stubClusterUpdateHubClient) ClaimUpdate(_ context.Context, _ string, _ string, _ string, _ string, targetVersion string) (*ClusterHubClaimUpdateResponse, error) {
	s.claimCalls++
	s.lastClaimTarget = targetVersion
	if s.claimErr != nil {
		return nil, s.claimErr
	}
	if len(s.claimResponses) > 0 {
		index := s.claimCalls - 1
		if index >= len(s.claimResponses) {
			index = len(s.claimResponses) - 1
		}
		return s.claimResponses[index], nil
	}
	if s.claimResponse != nil {
		return s.claimResponse, nil
	}
	return &ClusterHubClaimUpdateResponse{Proceed: true, TargetVersion: targetVersion}, nil
}

func (s *stubClusterUpdateHubClient) SetMemberStatus(_ context.Context, _ string, _ string, _ string, _ string, _ string, status string, panelVersion string) (*ClusterHubMemberStatusResponse, error) {
	s.setStatusCalls++
	s.lastStatus = status
	s.lastPanelVersion = panelVersion
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
