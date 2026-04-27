package service

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	database "github.com/BeanYa/b-ui/src/backend/internal/infra/db"
	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
)

func TestClusterServiceRegisterPersistsHubURLOnDomain(t *testing.T) {
	store := &stubClusterServiceStore{}
	hub := &stubClusterHubClient{
		registerResponse: &ClusterHubOperationResponse{OperationID: "op-register", Status: "completed"},
		snapshotResponse: &ClusterHubSnapshotResponse{Version: 4, Members: []ClusterHubMemberResponse{{NodeID: "node-a", BaseURL: "https://node-a.example.com", PeerToken: "peer-token-a"}}},
	}
	service := &ClusterService{
		secretProvider: stubClusterSecretProvider{secret: []byte("panel-secret-for-cluster-tests")},
		localIdentity:  ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{}},
		store:          store,
		hubClient:      hub,
	}

	_, err := service.Register(ClusterRegisterRequest{
		HubURL:  "https://hub.example.com",
		Domain:  "edge.example.com",
		Token:   "cluster-token",
		BaseURL: "https://panel.example.com/app/",
	})
	if err != nil {
		t.Fatalf("register cluster domain: %v", err)
	}
	if hub.lastRegisterRequest.Member.BaseURL != "https://panel.example.com/app/" {
		t.Fatalf("expected register request base URL to be forwarded, got %q", hub.lastRegisterRequest.Member.BaseURL)
	}
	if hub.lastRegisterRequest.Member.Address != "https://panel.example.com/app/" {
		t.Fatalf("expected register request address to be forwarded, got %q", hub.lastRegisterRequest.Member.Address)
	}
	if len(store.savedDomains) < 1 {
		t.Fatalf("expected saved domain state, got %d", len(store.savedDomains))
	}
	lastDomain := store.savedDomains[len(store.savedDomains)-1]
	if lastDomain.HubURL != "https://hub.example.com" {
		t.Fatalf("expected persisted hub URL, got %q", lastDomain.HubURL)
	}
	if lastDomain.CommunicationEndpointPath != "/_cluster" {
		t.Fatalf("expected fixed communication endpoint path, got %q", lastDomain.CommunicationEndpointPath)
	}
	if lastDomain.CommunicationProtocolVersion != "v1" {
		t.Fatalf("expected fixed communication protocol version, got %q", lastDomain.CommunicationProtocolVersion)
	}
	if len(store.replacedMembers) != 1 || len(store.replacedMembers[0]) != 1 {
		t.Fatalf("expected one replaced member, got %#v", store.replacedMembers)
	}
	if store.replacedMembers[0][0].DomainID != lastDomain.Id {
		t.Fatalf("expected member domain id %d, got %d", lastDomain.Id, store.replacedMembers[0][0].DomainID)
	}
}

func TestClusterServiceRegisterParsesJoinURI(t *testing.T) {
	store := &stubClusterServiceStore{}
	hub := &stubClusterHubClient{
		registerResponse: &ClusterHubOperationResponse{OperationID: "op-register", Status: "completed"},
		snapshotResponse: &ClusterHubSnapshotResponse{Version: 4, Members: []ClusterHubMemberResponse{{NodeID: "node-a", BaseURL: "https://node-a.example.com", PeerToken: "peer-token-a"}}},
	}
	service := &ClusterService{
		secretProvider: stubClusterSecretProvider{secret: []byte("panel-secret-for-cluster-tests")},
		localIdentity:  ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{}},
		store:          store,
		hubClient:      hub,
	}

	_, err := service.Register(ClusterRegisterRequest{
		JoinURI: "buihub://hub.example.com/domain/edge.example.com?domain_token=cluster-token&hub_protocol=https",
		BaseURL: "https://panel.example.com/app/",
	})
	if err != nil {
		t.Fatalf("register cluster domain from join URI: %v", err)
	}
	if hub.lastHubURL != "https://hub.example.com" {
		t.Fatalf("expected parsed hub URL, got %q", hub.lastHubURL)
	}
	if hub.lastRegisterRequest.DomainID != "edge.example.com" {
		t.Fatalf("expected parsed domain, got %q", hub.lastRegisterRequest.DomainID)
	}
	if hub.lastRegisterRequest.DomainToken != "cluster-token" {
		t.Fatalf("expected parsed token, got %q", hub.lastRegisterRequest.DomainToken)
	}
}

func TestClusterServiceRegisterDefaultsDisplayNameFromBaseURL(t *testing.T) {
	store := &stubClusterServiceStore{}
	hub := &stubClusterHubClient{
		registerResponse: &ClusterHubOperationResponse{OperationID: "op-register", Status: "completed"},
		snapshotResponse: &ClusterHubSnapshotResponse{Version: 4, Members: []ClusterHubMemberResponse{{NodeID: "node-a", BaseURL: "https://node-a.example.com", PeerToken: "peer-token-a"}}},
	}
	service := &ClusterService{
		secretProvider: stubClusterSecretProvider{secret: []byte("panel-secret-for-cluster-tests")},
		localIdentity:  ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{}},
		store:          store,
		hubClient:      hub,
	}

	_, err := service.Register(ClusterRegisterRequest{
		HubURL:  "https://hub.example.com",
		Domain:  "edge.example.com",
		Token:   "cluster-token",
		BaseURL: "https://jp.whoisbean.com:10443/beanui/",
	})
	if err != nil {
		t.Fatalf("register cluster domain: %v", err)
	}
	if hub.lastRegisterRequest.Member.DisplayName != "jp.whoisbean.com" {
		t.Fatalf("expected display name from base URL host, got %q", hub.lastRegisterRequest.Member.DisplayName)
	}
}

func TestClusterServiceRegisterMapsExistingBaseURLToLocalIdentity(t *testing.T) {
	store := &stubClusterServiceStore{}
	local := newTestClusterLocalNode(t, "node-local")
	hub := &stubClusterHubClient{
		registerResponse: &ClusterHubOperationResponse{OperationID: "op-register", Status: "completed"},
		snapshotResponse: &ClusterHubSnapshotResponse{Version: 4, Members: []ClusterHubMemberResponse{
			{NodeID: "node-existing", BaseURL: "https://JP.whoisbean.com:10443/beanui", PeerToken: "peer-token-a"},
			{NodeID: "node-local", BaseURL: "https://jp.whoisbean.com:10443/beanui/", PublicKey: local.PublicKey, PeerToken: "peer-token-a"},
		}},
	}
	service := &ClusterService{
		secretProvider: stubClusterSecretProvider{secret: []byte("panel-secret-for-cluster-tests")},
		localIdentity:  ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{node: local}},
		store:          store,
		hubClient:      hub,
	}

	_, err := service.Register(ClusterRegisterRequest{
		HubURL:  "https://hub.example.com",
		Domain:  "edge.example.com",
		Token:   "cluster-token",
		BaseURL: "https://jp.whoisbean.com:10443/beanui/",
	})
	if err != nil {
		t.Fatalf("register cluster domain: %v", err)
	}
	if hub.registerCalls != 1 {
		t.Fatalf("expected hub register call to map existing URL to local node, got %d", hub.registerCalls)
	}
	if hub.snapshotCalls != 1 {
		t.Fatalf("expected one snapshot refresh, got %d", hub.snapshotCalls)
	}
	if len(store.replacedMembers) != 1 || len(store.replacedMembers[0]) != 1 {
		t.Fatalf("expected local mirror to load one de-duplicated member, got %#v", store.replacedMembers)
	}
	if store.replacedMembers[0][0].NodeID != "node-local" {
		t.Fatalf("expected duplicate base URL to map to local node, got %q", store.replacedMembers[0][0].NodeID)
	}
}

func TestClusterServiceGetMemberConnectionDecryptsPeerTokenByNodeID(t *testing.T) {
	secret := []byte("panel-secret-for-cluster-tests")
	store := &stubClusterServiceStore{
		members: map[string]*model.ClusterMember{
			serviceMemberKey(1, "node-a"): {
				NodeID:             "node-a",
				Name:               "alpha",
				DisplayName:        "Alpha",
				BaseURL:            "https://node-a.example.com/beanui/",
				DomainID:           1,
				PeerTokenEncrypted: mustEncryptClusterToken(t, string(secret), "peer-token-a"),
			},
		},
	}
	service := &ClusterService{
		secretProvider: stubClusterSecretProvider{secret: secret},
		store:          store,
	}

	connection, err := service.GetMemberConnection("node-a")
	if err != nil {
		t.Fatalf("get member connection: %v", err)
	}
	if connection.BaseURL != "https://node-a.example.com/beanui/" {
		t.Fatalf("expected base URL, got %q", connection.BaseURL)
	}
	if connection.Token != "peer-token-a" {
		t.Fatalf("expected decrypted peer token, got %q", connection.Token)
	}
}

func TestClusterServiceRegisterDoesNotPersistDomainWhenHubRegistrationFails(t *testing.T) {
	store := &stubClusterServiceStore{}
	hub := &stubClusterHubClient{registerErr: errTestClusterHubFailure}
	service := &ClusterService{
		secretProvider: stubClusterSecretProvider{secret: []byte("panel-secret-for-cluster-tests")},
		localIdentity:  ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{}},
		store:          store,
		hubClient:      hub,
	}

	_, err := service.Register(ClusterRegisterRequest{
		HubURL:  "https://hub.example.com",
		Domain:  "edge.example.com",
		Token:   "cluster-token",
		BaseURL: "https://panel.example.com/app/",
	})
	if err == nil {
		t.Fatal("expected hub registration failure")
	}
	if len(store.savedDomains) != 0 {
		t.Fatalf("expected no persisted domains on failed registration, got %d", len(store.savedDomains))
	}
}

func TestClusterServiceRegisterRequiresHubURL(t *testing.T) {
	store := &stubClusterServiceStore{}
	hub := &stubClusterHubClient{}
	service := &ClusterService{
		secretProvider: stubClusterSecretProvider{secret: []byte("panel-secret-for-cluster-tests")},
		localIdentity:  ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{}},
		store:          store,
		hubClient:      hub,
	}

	_, err := service.Register(ClusterRegisterRequest{
		Domain:  "edge.example.com",
		Token:   "cluster-token",
		BaseURL: "https://panel.example.com/app/",
	})
	if err == nil {
		t.Fatal("expected missing hub URL to fail")
	}
	if hub.registerCalls != 0 {
		t.Fatalf("expected no hub register calls, got %d", hub.registerCalls)
	}
	if len(store.savedDomains) != 0 {
		t.Fatalf("expected no persisted domains, got %d", len(store.savedDomains))
	}
}

func TestClusterServiceRegisterRequiresBaseURL(t *testing.T) {
	store := &stubClusterServiceStore{}
	hub := &stubClusterHubClient{}
	service := &ClusterService{
		secretProvider: stubClusterSecretProvider{secret: []byte("panel-secret-for-cluster-tests")},
		localIdentity:  ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{}},
		store:          store,
		hubClient:      hub,
	}

	_, err := service.Register(ClusterRegisterRequest{
		HubURL: "https://hub.example.com",
		Domain: "edge.example.com",
		Token:  "cluster-token",
	})
	if err == nil {
		t.Fatal("expected missing base URL to fail")
	}
	if hub.registerCalls != 0 {
		t.Fatalf("expected no hub register calls, got %d", hub.registerCalls)
	}
	if len(store.savedDomains) != 0 {
		t.Fatalf("expected no persisted domains, got %d", len(store.savedDomains))
	}
}

func TestClusterServiceDeleteMemberDeletesHubMembershipBeforeLocalMirror(t *testing.T) {
	secret := []byte("panel-secret-for-cluster-tests")
	store := &stubClusterServiceStore{
		domains: map[string]*model.ClusterDomain{
			"edge.example.com": {Id: 1, Domain: "edge.example.com", HubURL: "https://hub.example.com", TokenEncrypted: mustEncryptClusterToken(t, string(secret), "domain-token")},
		},
		members: map[string]*model.ClusterMember{
			serviceMemberKey(1, "node-a"): {Id: 7, NodeID: "node-a", DomainID: 1},
		},
	}
	hub := &stubClusterHubClient{}
	service := &ClusterService{
		secretProvider: stubClusterSecretProvider{secret: secret},
		store:          store,
		hubClient:      hub,
	}

	if err := service.DeleteMember(7); err != nil {
		t.Fatalf("delete member: %v", err)
	}
	if store.deletedMemberID != 7 {
		t.Fatalf("expected local member delete for id 7, got %d", store.deletedMemberID)
	}
}

func TestClusterServiceLeaveDomainDeletesLocalNodeFromHubAndRemovesDomainMirror(t *testing.T) {
	secret := []byte("panel-secret-for-cluster-tests")
	store := &stubClusterServiceStore{
		domains: map[string]*model.ClusterDomain{
			"edge.example.com": {Id: 1, Domain: "edge.example.com", HubURL: "https://hub.example.com", TokenEncrypted: mustEncryptClusterToken(t, string(secret), "domain-token")},
		},
		members: map[string]*model.ClusterMember{
			serviceMemberKey(1, "node-local"): {Id: 7, NodeID: "node-local", DomainID: 1},
			serviceMemberKey(1, "node-peer"):  {Id: 8, NodeID: "node-peer", DomainID: 1},
		},
	}
	hub := &stubClusterHubClient{}
	service := &ClusterService{
		secretProvider: stubClusterSecretProvider{secret: secret},
		localIdentity:  ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{node: &model.ClusterLocalNode{NodeID: "node-local"}}},
		store:          store,
		hubClient:      hub,
	}

	if err := service.LeaveDomain(1); err != nil {
		t.Fatalf("leave domain: %v", err)
	}
	if hub.lastDeleteMemberID != "node-local" {
		t.Fatalf("expected hub delete for local node, got %q", hub.lastDeleteMemberID)
	}
	if store.deletedDomainID != 1 {
		t.Fatalf("expected local domain mirror delete for id 1, got %d", store.deletedDomainID)
	}
}

func TestClusterServiceListMembersMarksLocalNode(t *testing.T) {
	store := &stubClusterServiceStore{
		members: map[string]*model.ClusterMember{
			serviceMemberKey(1, "node-local"): {Id: 7, NodeID: "node-local", DomainID: 1},
			serviceMemberKey(1, "node-peer"):  {Id: 8, NodeID: "node-peer", DomainID: 1},
		},
	}
	service := &ClusterService{
		localIdentity: ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{node: &model.ClusterLocalNode{NodeID: "node-local"}}},
		store:         store,
	}

	members, err := service.ListMembers()
	if err != nil {
		t.Fatalf("list cluster members: %v", err)
	}

	var localSeen bool
	var peerSeen bool
	for _, member := range members {
		switch member.NodeID {
		case "node-local":
			localSeen = true
			if !member.IsLocal {
				t.Fatalf("expected local member to be marked isLocal")
			}
		case "node-peer":
			peerSeen = true
			if member.IsLocal {
				t.Fatalf("expected peer member not to be marked isLocal")
			}
		}
	}
	if !localSeen || !peerSeen {
		t.Fatalf("expected both local and peer members, got %#v", members)
	}
}

func TestClusterServiceListDomainsIncludesSupportedActions(t *testing.T) {
	store := &stubClusterServiceStore{
		domains: map[string]*model.ClusterDomain{
			"edge.example.com": {
				Id:                           1,
				Domain:                       "edge.example.com",
				HubURL:                       "https://hub.example.com",
				CommunicationEndpointPath:    "/_cluster",
				CommunicationProtocolVersion: "v1",
				LastVersion:                  4,
			},
		},
	}
	service := &ClusterService{store: store}

	domains, err := service.ListDomains()
	if err != nil {
		t.Fatalf("list cluster domains: %v", err)
	}
	if len(domains) != 1 {
		t.Fatalf("expected one domain, got %#v", domains)
	}
	if domains[0].CommunicationEndpointPath != "/_cluster" {
		t.Fatalf("expected endpoint path, got %q", domains[0].CommunicationEndpointPath)
	}
	if domains[0].CommunicationProtocolVersion != "v1" {
		t.Fatalf("expected protocol version, got %q", domains[0].CommunicationProtocolVersion)
	}
	expectedActions := []string{"domain.cluster.changed", "events", "heartbeat", "ping", "info", "action", "domain.panel.update.available"}
	if !reflect.DeepEqual(domains[0].SupportedActions, expectedActions) {
		t.Fatalf("expected supported actions %#v, got %#v", expectedActions, domains[0].SupportedActions)
	}
}

func TestClusterServiceReceiveMessageRejectsWrongPeerTokenEvenWhenDomainTokenMatches(t *testing.T) {
	secret := []byte("panel-secret-for-cluster-tests")
	sourcePublicKey, sourcePrivateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate source keypair: %v", err)
	}
	store := &stubClusterServiceStore{
		domains: map[string]*model.ClusterDomain{
			"edge.example.com": {Id: 1, Domain: "edge.example.com", TokenEncrypted: mustEncryptClusterToken(t, string(secret), "domain-token")},
		},
		members: map[string]*model.ClusterMember{
			serviceMemberKey(1, "node-local"):  {NodeID: "node-local", DomainID: 1, PeerTokenEncrypted: mustEncryptClusterToken(t, string(secret), "peer-token-local")},
			serviceMemberKey(1, "node-source"): {NodeID: "node-source", DomainID: 1, PublicKey: base64.StdEncoding.EncodeToString(sourcePublicKey)},
		},
	}
	service := &ClusterService{
		secretProvider: stubClusterSecretProvider{secret: secret},
		localIdentity:  ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{node: &model.ClusterLocalNode{NodeID: "node-local"}}},
		store:          store,
	}
	envelope, err := SignClusterNotifyVersionEnvelope(&model.ClusterLocalNode{NodeID: "node-source", PrivateKey: base64.StdEncoding.EncodeToString(sourcePrivateKey)}, "edge.example.com", 9, 1700000000)
	if err != nil {
		t.Fatalf("sign envelope: %v", err)
	}

	err = service.ReceiveMessage(envelope, "domain-token")
	if err == nil {
		t.Fatal("expected wrong peer token to be rejected")
	}
}

func TestClusterServiceReceiveMessageAcceptsLocalMemberPeerTokenForCorrectDomain(t *testing.T) {
	secret := []byte("panel-secret-for-cluster-tests")
	sourcePublicKey, sourcePrivateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate source keypair: %v", err)
	}
	store := &stubClusterServiceStore{
		domains: map[string]*model.ClusterDomain{
			"edge.example.com": {Id: 1, Domain: "edge.example.com", TokenEncrypted: mustEncryptClusterToken(t, string(secret), "domain-token")},
		},
		members: map[string]*model.ClusterMember{
			serviceMemberKey(1, "node-local"):  {NodeID: "node-local", DomainID: 1, PeerTokenEncrypted: mustEncryptClusterToken(t, string(secret), "peer-token-local")},
			serviceMemberKey(1, "node-source"): {NodeID: "node-source", DomainID: 1, PublicKey: base64.StdEncoding.EncodeToString(sourcePublicKey)},
		},
	}
	service := &ClusterService{
		secretProvider: stubClusterSecretProvider{secret: secret},
		localIdentity:  ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{node: &model.ClusterLocalNode{NodeID: "node-local"}}},
		store:          store,
	}
	envelope, err := SignClusterNotifyVersionEnvelope(&model.ClusterLocalNode{NodeID: "node-source", PrivateKey: base64.StdEncoding.EncodeToString(sourcePrivateKey)}, "edge.example.com", 9, 1700000000)
	if err != nil {
		t.Fatalf("sign envelope: %v", err)
	}

	if err := service.ReceiveMessage(envelope, "peer-token-local"); err != nil {
		t.Fatalf("expected correct peer token to be accepted, got %v", err)
	}
}

func TestClusterServiceReceivePeerMessageInitializesInjectedSyncDependencies(t *testing.T) {
	secret := []byte("panel-secret-for-cluster-tests")
	sourcePublicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate source keypair: %v", err)
	}
	sourcePublicKeyEncoded := base64.StdEncoding.EncodeToString(sourcePublicKey)
	store := &stubClusterServiceStore{
		domains: map[string]*model.ClusterDomain{
			"edge.example.com": {Id: 1, Domain: "edge.example.com", HubURL: "https://hub.example.com", TokenEncrypted: mustEncryptClusterToken(t, string(secret), "domain-token")},
		},
		members: map[string]*model.ClusterMember{
			serviceMemberKey(1, "node-local"):  {NodeID: "node-local", DomainID: 1, PeerTokenEncrypted: mustEncryptClusterToken(t, string(secret), "peer-token-local")},
			serviceMemberKey(1, "node-source"): {NodeID: "node-source", DomainID: 1, PublicKey: sourcePublicKeyEncoded, LastVersion: 1},
		},
	}
	hub := &stubClusterHubClient{
		snapshotResponse: &ClusterHubSnapshotResponse{
			Version: 9,
			Members: []ClusterHubMemberResponse{
				{NodeID: "node-local", BaseURL: "https://local.example.com", PeerToken: "peer-token-local"},
				{NodeID: "node-source", BaseURL: "https://source.example.com", PublicKey: sourcePublicKeyEncoded},
			},
		},
	}
	service := &ClusterService{
		secretProvider: stubClusterSecretProvider{secret: secret},
		localIdentity:  ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{node: &model.ClusterLocalNode{NodeID: "node-local"}}},
		store:          store,
		hubClient:      hub,
		syncService: ClusterSyncService{
			store:     &clusterSyncStoreAdapter{store: store},
			hubSyncer: &ClusterHubSyncer{client: hub, store: store, secretProvider: stubClusterSecretProvider{secret: secret}, reachability: newTestClusterReachabilityService(20)},
		},
	}

	syncService := service.peerSyncService()
	if _, err := syncService.HandleIncomingNotifyVersion(context.Background(), 1, "node-source", 9); err != nil {
		t.Fatalf("handle incoming notify version: %v", err)
	}
	if len(store.savedMembers) == 0 {
		t.Fatal("expected injected store to save source member version")
	}
	if got := store.savedMembers[0].LastVersion; got != 9 {
		t.Fatalf("expected saved source member version 9, got %d", got)
	}
	if hub.snapshotCalls != 1 {
		t.Fatalf("expected injected hub client snapshot call, got %d", hub.snapshotCalls)
	}
	if hub.lastSnapshotToken != "domain-token" {
		t.Fatalf("expected injected secret provider to decrypt domain token, got %q", hub.lastSnapshotToken)
	}
	if len(store.replacedMembers) != 1 {
		t.Fatalf("expected injected store to replace members from snapshot, got %d", len(store.replacedMembers))
	}
}

func TestClusterServiceReceivePeerMessageRefreshesUnknownSourceBeforeRejecting(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "receive-peer-refresh.db")); err != nil {
		if strings.Contains(err.Error(), "go-sqlite3 requires cgo") || strings.Contains(err.Error(), "C compiler") {
			t.Skipf("sqlite test database unavailable in this toolchain: %v", err)
		}
		t.Fatalf("init test db: %v", err)
	}

	secret := []byte("panel-secret-for-cluster-tests")
	source := newTestClusterLocalNode(t, "node-source")
	store := &stubClusterServiceStore{
		domains: map[string]*model.ClusterDomain{
			"edge.example.com": {Id: 1, Domain: "edge.example.com", HubURL: "https://hub.example.com", LastVersion: 1, TokenEncrypted: mustEncryptClusterToken(t, string(secret), "domain-token")},
		},
		members: map[string]*model.ClusterMember{
			serviceMemberKey(1, "node-local"): {NodeID: "node-local", DomainID: 1, PeerTokenEncrypted: mustEncryptClusterToken(t, string(secret), "peer-token-local")},
		},
	}
	hub := &stubClusterHubClient{
		snapshotResponse: &ClusterHubSnapshotResponse{
			Version: 9,
			Members: []ClusterHubMemberResponse{
				{NodeID: "node-local", BaseURL: "https://local.example.com", PeerToken: "peer-token-local"},
				{NodeID: "node-source", BaseURL: "https://source.example.com", PublicKey: source.PublicKey},
			},
		},
	}
	service := &ClusterService{
		secretProvider: stubClusterSecretProvider{secret: secret},
		localIdentity:  ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{node: &model.ClusterLocalNode{NodeID: "node-local"}}},
		store:          store,
		hubClient:      hub,
	}
	message, err := NewClusterPeerMessage("edge.example.com", 9, "node-source", 1, PeerCategoryEvent, "future.action", map[string]any{"version": float64(9)})
	if err != nil {
		t.Fatalf("new peer message: %v", err)
	}
	if err := SignClusterPeerMessage(source, message); err != nil {
		t.Fatalf("sign peer message: %v", err)
	}

	if err := service.ReceivePeerMessage(message, "peer-token-local"); err != nil {
		t.Fatalf("receive peer message after membership refresh: %v", err)
	}
	if hub.snapshotCalls != 1 {
		t.Fatalf("expected one snapshot refresh, got %d", hub.snapshotCalls)
	}
	if _, err := store.GetMemberByDomainNodeID(1, "node-source"); err != nil {
		t.Fatalf("expected source member to be loaded from snapshot: %v", err)
	}
}

func TestClusterServiceReceivePeerMessageRefreshesNewerMembershipBeforeSignatureCheck(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "receive-peer-newer-refresh.db")); err != nil {
		if strings.Contains(err.Error(), "go-sqlite3 requires cgo") || strings.Contains(err.Error(), "C compiler") {
			t.Skipf("sqlite test database unavailable in this toolchain: %v", err)
		}
		t.Fatalf("init test db: %v", err)
	}

	secret := []byte("panel-secret-for-cluster-tests")
	staleSource := newTestClusterLocalNode(t, "node-source")
	source := newTestClusterLocalNode(t, "node-source")
	store := &stubClusterServiceStore{
		domains: map[string]*model.ClusterDomain{
			"edge.example.com": {Id: 1, Domain: "edge.example.com", HubURL: "https://hub.example.com", LastVersion: 1, TokenEncrypted: mustEncryptClusterToken(t, string(secret), "domain-token")},
		},
		members: map[string]*model.ClusterMember{
			serviceMemberKey(1, "node-local"):  {NodeID: "node-local", DomainID: 1, PeerTokenEncrypted: mustEncryptClusterToken(t, string(secret), "peer-token-local")},
			serviceMemberKey(1, "node-source"): {NodeID: "node-source", DomainID: 1, PublicKey: staleSource.PublicKey, LastVersion: 1},
		},
	}
	hub := &stubClusterHubClient{
		snapshotResponse: &ClusterHubSnapshotResponse{
			Version: 9,
			Members: []ClusterHubMemberResponse{
				{NodeID: "node-local", BaseURL: "https://local.example.com", PeerToken: "peer-token-local"},
				{NodeID: "node-source", BaseURL: "https://source.example.com", PublicKey: source.PublicKey},
			},
		},
	}
	service := &ClusterService{
		secretProvider: stubClusterSecretProvider{secret: secret},
		localIdentity:  ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{node: &model.ClusterLocalNode{NodeID: "node-local"}}},
		store:          store,
		hubClient:      hub,
	}
	message, err := NewClusterPeerMessage("edge.example.com", 9, "node-source", 1, PeerCategoryEvent, "future.action", map[string]any{"version": float64(9)})
	if err != nil {
		t.Fatalf("new peer message: %v", err)
	}
	if err := SignClusterPeerMessage(source, message); err != nil {
		t.Fatalf("sign peer message: %v", err)
	}

	if err := service.ReceivePeerMessage(message, "peer-token-local"); err != nil {
		t.Fatalf("receive peer message after newer membership refresh: %v", err)
	}
	if hub.snapshotCalls != 1 {
		t.Fatalf("expected one snapshot refresh, got %d", hub.snapshotCalls)
	}
	refreshed, err := store.GetMemberByDomainNodeID(1, "node-source")
	if err != nil {
		t.Fatalf("expected refreshed source member: %v", err)
	}
	if refreshed.PublicKey != source.PublicKey {
		t.Fatal("expected refreshed source public key before signature verification")
	}
}

type stubClusterServiceStore struct {
	domains         map[string]*model.ClusterDomain
	domainsList     []model.ClusterDomain
	savedDomains    []*model.ClusterDomain
	members         map[string]*model.ClusterMember
	savedMembers    []*model.ClusterMember
	replacedMembers [][]model.ClusterMember
	deletedMemberID uint
	deletedDomainID uint
}

func (s *stubClusterServiceStore) GetDomainByName(name string) (*model.ClusterDomain, error) {
	if s.domains == nil || s.domains[name] == nil {
		return nil, errClusterDomainNotFound
	}
	copy := *s.domains[name]
	return &copy, nil
}

func (s *stubClusterServiceStore) SaveDomain(domain *model.ClusterDomain) error {
	if s.domains == nil {
		s.domains = map[string]*model.ClusterDomain{}
	}
	copy := *domain
	if copy.Id == 0 {
		copy.Id = uint(len(s.domains) + 1)
		domain.Id = copy.Id
	}
	s.domains[copy.Domain] = &copy
	s.savedDomains = append(s.savedDomains, &copy)
	return nil
}

func (s *stubClusterServiceStore) ListDomains() ([]model.ClusterDomain, error) {
	domains := make([]model.ClusterDomain, 0, len(s.domains))
	for _, domain := range s.domains {
		domains = append(domains, *domain)
	}
	return domains, nil
}
func (s *stubClusterServiceStore) GetDomain(id uint) (*model.ClusterDomain, error) {
	for _, domain := range s.domains {
		if domain.Id == id {
			copy := *domain
			return &copy, nil
		}
	}
	return nil, errClusterDomainNotFound
}
func (s *stubClusterServiceStore) GetMember(id uint) (*model.ClusterMember, error) {
	for _, member := range s.members {
		if member.Id == id {
			copy := *member
			return &copy, nil
		}
	}
	return nil, errClusterMemberNotFound
}
func (s *stubClusterServiceStore) GetMemberByNodeID(nodeID string) (*model.ClusterMember, error) {
	if s.members == nil {
		return nil, errClusterMemberNotFound
	}
	for _, member := range s.members {
		if member.NodeID == nodeID {
			copy := *member
			return &copy, nil
		}
	}
	return nil, errClusterMemberNotFound
}

func (s *stubClusterServiceStore) GetMemberByDomainNodeID(domainID uint, nodeID string) (*model.ClusterMember, error) {
	key := serviceMemberKey(domainID, nodeID)
	if s.members == nil || s.members[key] == nil {
		return nil, errClusterMemberNotFound
	}
	copy := *s.members[key]
	return &copy, nil
}
func (s *stubClusterServiceStore) SaveMember(member *model.ClusterMember) error {
	copy := *member
	s.savedMembers = append(s.savedMembers, &copy)
	if s.members == nil {
		s.members = map[string]*model.ClusterMember{}
	}
	s.members[serviceMemberKey(copy.DomainID, copy.NodeID)] = &copy
	return nil
}
func (s *stubClusterServiceStore) ListMembers() ([]model.ClusterMember, error) {
	members := make([]model.ClusterMember, 0, len(s.members))
	for _, member := range s.members {
		members = append(members, *member)
	}
	return members, nil
}
func (s *stubClusterServiceStore) DeleteMember(id uint) error {
	s.deletedMemberID = id
	return nil
}
func (s *stubClusterServiceStore) DeleteDomain(id uint) error {
	s.deletedDomainID = id
	return nil
}
func (s *stubClusterServiceStore) ReplaceDomainMembers(_ uint, members []model.ClusterMember) error {
	copyMembers := make([]model.ClusterMember, len(members))
	copy(copyMembers, members)
	s.replacedMembers = append(s.replacedMembers, copyMembers)
	if s.members == nil {
		s.members = map[string]*model.ClusterMember{}
	}
	for _, member := range copyMembers {
		copy := member
		s.members[serviceMemberKey(copy.DomainID, copy.NodeID)] = &copy
	}
	return nil
}

type stubClusterHubClient struct {
	registerResponse    *ClusterHubOperationResponse
	snapshotResponse    *ClusterHubSnapshotResponse
	deleteResponse      *ClusterHubOperationResponse
	registerErr         error
	deleteErr           error
	registerCalls       int
	snapshotCalls       int
	lastHubURL          string
	lastSnapshotHubURL  string
	lastSnapshotDomain  string
	lastSnapshotToken   string
	lastRegisterRequest   ClusterHubRegisterNodeRequest
	lastDeleteMemberID    string
	lastClaimTargetVersion string
	claimResponse         *ClusterHubClaimUpdateResponse
}

func (s *stubClusterHubClient) RegisterNode(_ context.Context, hubURL string, request ClusterHubRegisterNodeRequest) (*ClusterHubOperationResponse, error) {
	s.registerCalls++
	s.lastHubURL = hubURL
	s.lastRegisterRequest = request
	if s.registerErr != nil {
		return nil, s.registerErr
	}
	return s.registerResponse, nil
}

func (s *stubClusterHubClient) GetLatestVersion(context.Context, string, string, string) (*ClusterHubVersionResponse, error) {
	return nil, nil
}

func (s *stubClusterHubClient) GetSnapshot(_ context.Context, hubURL string, domain string, token string) (*ClusterHubSnapshotResponse, error) {
	s.snapshotCalls++
	s.lastSnapshotHubURL = hubURL
	s.lastSnapshotDomain = domain
	s.lastSnapshotToken = token
	return s.snapshotResponse, nil
}

func (s *stubClusterHubClient) DeleteMember(_ context.Context, _ string, _ string, _ string, memberID string) (*ClusterHubOperationResponse, error) {
	s.lastDeleteMemberID = memberID
	if s.deleteErr != nil {
		return nil, s.deleteErr
	}
	if s.deleteResponse != nil {
		return s.deleteResponse, nil
	}
	return &ClusterHubOperationResponse{OperationID: "op-delete", Status: "completed"}, nil
}

func (s *stubClusterHubClient) ClaimUpdate(_ context.Context, _ string, _ string, _ string, _ string, targetVersion string) (*ClusterHubClaimUpdateResponse, error) {
	s.lastClaimTargetVersion = targetVersion
	if s.claimResponse != nil {
		return s.claimResponse, nil
	}
	return &ClusterHubClaimUpdateResponse{Proceed: true, TargetVersion: targetVersion}, nil
}

func (s *stubClusterHubClient) SetMemberStatus(_ context.Context, _ string, _ string, _ string, _ string, _ string, _ string, _ string) (*ClusterHubMemberStatusResponse, error) {
	return &ClusterHubMemberStatusResponse{OK: true}, nil
}

type stubClusterSecretProvider struct{ secret []byte }

func (s stubClusterSecretProvider) GetSecret() ([]byte, error) { return s.secret, nil }

var errTestClusterHubFailure = context.Canceled

func serviceMemberKey(domainID uint, nodeID string) string {
	return fmt.Sprintf("%d:%s", domainID, nodeID)
}
