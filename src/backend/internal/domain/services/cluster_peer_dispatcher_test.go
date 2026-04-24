package service

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alireza0/s-ui/src/backend/internal/infra/db/model"
)

func TestPeerDispatcherMarksUnsupportedEventWithoutError(t *testing.T) {
	store := newMemoryPeerStore()
	dispatcher := ClusterPeerDispatcher{eventStore: store}
	message := &PeerMessage{
		MessageID:   "msg-unsupported",
		DomainID:    "edge.example.com",
		PayloadHash: "hash",
		Category:    "event",
		Action:      "future.action",
	}
	if err := dispatcher.Dispatch(context.Background(), &model.ClusterDomain{Id: 1, Domain: "edge.example.com"}, &model.ClusterMember{NodeID: "node-a"}, message); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	state, err := store.RecordReceived(message)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	if state.Status != PeerEventStatusUnsupported {
		t.Fatalf("expected unsupported, got %q", state.Status)
	}
}

func TestPeerDispatcherHandlesDomainClusterChanged(t *testing.T) {
	store := newMemoryPeerStore()
	syncer := &stubPeerSyncer{}
	dispatcher := ClusterPeerDispatcher{eventStore: store, syncService: &ClusterSyncService{hubSyncer: syncer, store: &stubPeerDispatcherSyncStore{domain: &model.ClusterDomain{Id: 1, Domain: "edge.example.com", HubURL: "https://hub.example.com"}}}}
	message := &PeerMessage{
		MessageID:   "msg-change",
		DomainID:    "edge.example.com",
		PayloadHash: "hash",
		Category:    "event",
		Action:      "domain.cluster.changed",
		Payload:     map[string]interface{}{"version": float64(9)},
	}
	if err := dispatcher.Dispatch(context.Background(), &model.ClusterDomain{Id: 1, Domain: "edge.example.com", HubURL: "https://hub.example.com"}, &model.ClusterMember{NodeID: "node-a", LastVersion: 1}, message); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	state, err := store.RecordReceived(message)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	if state.Status != PeerEventStatusSucceeded {
		t.Fatalf("expected succeeded, got %q", state.Status)
	}
}

func TestPeerDispatcherSavesSuccessfulChainStepAndSendsNextStep(t *testing.T) {
	secret := []byte("panel-secret-for-cluster-tests")
	local := newTestClusterLocalNode(t, "node-local")
	peerToken, err := EncryptClusterDomainToken(secret, "peer-token-b")
	if err != nil {
		t.Fatalf("encrypt peer token: %v", err)
	}

	var forwarded PeerMessage
	var gotToken string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("X-Cluster-Token")
		if err := json.NewDecoder(r.Body).Decode(&forwarded); err != nil {
			t.Errorf("decode forwarded message: %v", err)
		}
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	var saved []savedPeerWorkflowStep
	dispatcher := ClusterPeerDispatcher{
		eventStore: newMemoryPeerStore(),
		syncService: &ClusterSyncService{
			hubSyncer: &stubPeerSyncer{},
			store: &stubPeerDispatcherSyncStore{
				domain: &model.ClusterDomain{Id: 1, Domain: "edge.example.com"},
				member: &model.ClusterMember{NodeID: "node-b", DomainID: 1, BaseURL: server.URL, PeerTokenEncrypted: peerToken},
			},
		},
		identity:       ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{node: local}},
		secretProvider: stubClusterSecretProvider{secret: secret},
		delivery:       &ClusterPeerDeliveryService{HTTPClient: server.Client()},
		saveWorkflowStep: func(workflowID string, stepID string, domainID string, nodeID string, status string, resultHash string, errorMessage string) error {
			saved = append(saved, savedPeerWorkflowStep{workflowID: workflowID, stepID: stepID, domainID: domainID, nodeID: nodeID, status: status, resultHash: resultHash, errorMessage: errorMessage})
			return nil
		},
	}

	message := &PeerMessage{
		MessageID:     "msg-chain-success",
		WorkflowID:    "workflow-1",
		StepID:        "step-a",
		DomainID:      "edge.example.com",
		SourceNodeID:  "node-source",
		SourceSeq:     41,
		PayloadHash:   "hash-current",
		Category:      PeerCategoryEvent,
		Action:        PeerActionDomainClusterChanged,
		CorrelationID: "corr-1",
		CausationID:   "cause-1",
		Payload:       map[string]interface{}{"version": float64(9)},
		Route: RoutePlan{Mode: RouteModeChain, Chain: []RouteStep{
			{StepID: "step-a", NodeID: "node-local"},
			{StepID: "step-b", NodeID: "node-b", Action: "next.action", PayloadOverride: map[string]interface{}{"next": "payload"}},
		}},
	}

	if err := dispatcher.Dispatch(context.Background(), &model.ClusterDomain{Id: 1, Domain: "edge.example.com"}, &model.ClusterMember{NodeID: "node-source"}, message); err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	if len(saved) != 1 {
		t.Fatalf("expected one saved workflow step, got %d", len(saved))
	}
	if saved[0].workflowID != "workflow-1" || saved[0].stepID != "step-a" || saved[0].status != PeerEventStatusSucceeded {
		t.Fatalf("unexpected saved workflow step: %#v", saved[0])
	}
	if saved[0].domainID != "edge.example.com" || saved[0].nodeID != "node-local" || saved[0].resultHash != "hash-current" {
		t.Fatalf("unexpected saved workflow metadata: %#v", saved[0])
	}

	if gotToken != "peer-token-b" {
		t.Fatalf("expected decrypted peer token, got %q", gotToken)
	}
	if forwarded.WorkflowID != "workflow-1" || forwarded.StepID != "step-b" {
		t.Fatalf("unexpected forwarded workflow identifiers: %#v", forwarded)
	}
	if forwarded.SourceNodeID != "node-local" {
		t.Fatalf("expected local source node, got %q", forwarded.SourceNodeID)
	}
	if forwarded.Action != "next.action" {
		t.Fatalf("expected route step action override, got %q", forwarded.Action)
	}
	if forwarded.Payload["next"] != "payload" {
		t.Fatalf("expected route step payload override, got %#v", forwarded.Payload)
	}
	if forwarded.CorrelationID != "corr-1" || forwarded.CausationID != "cause-1" {
		t.Fatalf("expected correlation and causation to be preserved, got %q/%q", forwarded.CorrelationID, forwarded.CausationID)
	}
	if forwarded.Signature == "" || forwarded.PayloadHash == "" {
		t.Fatalf("expected forwarded message to be signed, got signature=%q hash=%q", forwarded.Signature, forwarded.PayloadHash)
	}
}

func TestPeerDispatcherRetriesSuccessfulChainStepWhenNextStepDeliveryFails(t *testing.T) {
	secret := []byte("panel-secret-for-cluster-tests")
	local := newTestClusterLocalNode(t, "node-local")
	peerToken, err := EncryptClusterDomainToken(secret, "peer-token-b")
	if err != nil {
		t.Fatalf("encrypt peer token: %v", err)
	}

	attempts := 0
	var forwarded []PeerMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		var message PeerMessage
		if err := json.NewDecoder(r.Body).Decode(&message); err != nil {
			t.Errorf("decode forwarded message: %v", err)
		}
		forwarded = append(forwarded, message)
		if attempts == 1 {
			http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	store := newMemoryPeerStore()
	var saved []savedPeerWorkflowStep
	dispatcher := ClusterPeerDispatcher{
		eventStore: store,
		syncService: &ClusterSyncService{
			hubSyncer: &stubPeerSyncer{},
			store: &stubPeerDispatcherSyncStore{
				domain: &model.ClusterDomain{Id: 1, Domain: "edge.example.com"},
				member: &model.ClusterMember{NodeID: "node-b", DomainID: 1, BaseURL: server.URL, PeerTokenEncrypted: peerToken},
			},
		},
		identity:       ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{node: local}},
		secretProvider: stubClusterSecretProvider{secret: secret},
		delivery:       &ClusterPeerDeliveryService{HTTPClient: server.Client()},
		saveWorkflowStep: func(workflowID string, stepID string, domainID string, nodeID string, status string, resultHash string, errorMessage string) error {
			saved = append(saved, savedPeerWorkflowStep{workflowID: workflowID, stepID: stepID, domainID: domainID, nodeID: nodeID, status: status, resultHash: resultHash, errorMessage: errorMessage})
			return nil
		},
	}

	message := successfulChainMessage("msg-chain-success-retry")
	if err := dispatcher.Dispatch(context.Background(), &model.ClusterDomain{Id: 1, Domain: "edge.example.com"}, &model.ClusterMember{NodeID: "node-source"}, message); err == nil {
		t.Fatal("expected first dispatch to return delivery error")
	}
	if attempts != 1 {
		t.Fatalf("expected one delivery attempt after first dispatch, got %d", attempts)
	}

	if err := dispatcher.Dispatch(context.Background(), &model.ClusterDomain{Id: 1, Domain: "edge.example.com"}, &model.ClusterMember{NodeID: "node-source"}, message); err != nil {
		t.Fatalf("retry dispatch: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected retry to attempt delivery again, got %d attempts", attempts)
	}
	if len(forwarded) != 2 {
		t.Fatalf("expected two forwarded messages, got %d", len(forwarded))
	}
	if forwarded[0].MessageID == "" || forwarded[1].MessageID == "" {
		t.Fatalf("expected forwarded message IDs to be set: %#v", forwarded)
	}
	if forwarded[0].MessageID != forwarded[1].MessageID {
		t.Fatalf("expected stable forwarded message ID across retry, got %q then %q", forwarded[0].MessageID, forwarded[1].MessageID)
	}
	if forwarded[0].IdempotencyKey == "" || forwarded[0].IdempotencyKey != forwarded[1].IdempotencyKey {
		t.Fatalf("expected stable forwarded idempotency key across retry, got %q then %q", forwarded[0].IdempotencyKey, forwarded[1].IdempotencyKey)
	}
	if len(saved) != 2 {
		t.Fatalf("expected workflow step to be saved on both attempts, got %d", len(saved))
	}
	state, err := store.RecordReceived(message)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	if state.Status != PeerEventStatusSucceeded {
		t.Fatalf("expected succeeded after retry, got %q", state.Status)
	}
}

func TestPeerDispatcherChainFailureContinuationRequiresContinueOnFailure(t *testing.T) {
	tests := []struct {
		name              string
		continueOnFailure bool
		wantForwarded     bool
	}{
		{name: "stops by default"},
		{name: "continues when allowed", continueOnFailure: true, wantForwarded: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secret := []byte("panel-secret-for-cluster-tests")
			local := newTestClusterLocalNode(t, "node-local")
			peerToken, err := EncryptClusterDomainToken(secret, "peer-token-b")
			if err != nil {
				t.Fatalf("encrypt peer token: %v", err)
			}

			forwardedCount := 0
			var forwarded PeerMessage
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				forwardedCount++
				if err := json.NewDecoder(r.Body).Decode(&forwarded); err != nil {
					t.Errorf("decode forwarded message: %v", err)
				}
				_, _ = w.Write([]byte(`{"success":true}`))
			}))
			defer server.Close()

			var saved []savedPeerWorkflowStep
			dispatcher := ClusterPeerDispatcher{
				eventStore: newMemoryPeerStore(),
				syncService: &ClusterSyncService{
					hubSyncer: &stubPeerSyncer{},
					store: &stubPeerDispatcherSyncStore{
						domain: &model.ClusterDomain{Id: 1, Domain: "edge.example.com"},
						member: &model.ClusterMember{NodeID: "node-b", DomainID: 1, BaseURL: server.URL, PeerTokenEncrypted: peerToken},
					},
				},
				identity:       ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{node: local}},
				secretProvider: stubClusterSecretProvider{secret: secret},
				delivery:       &ClusterPeerDeliveryService{HTTPClient: server.Client()},
				saveWorkflowStep: func(workflowID string, stepID string, domainID string, nodeID string, status string, resultHash string, errorMessage string) error {
					saved = append(saved, savedPeerWorkflowStep{workflowID: workflowID, stepID: stepID, domainID: domainID, nodeID: nodeID, status: status, resultHash: resultHash, errorMessage: errorMessage})
					return nil
				},
			}

			message := &PeerMessage{
				MessageID:    "msg-chain-failure-" + tt.name,
				WorkflowID:   "workflow-1",
				StepID:       "step-a",
				DomainID:     "edge.example.com",
				SourceNodeID: "node-source",
				PayloadHash:  "hash-current",
				Category:     PeerCategoryEvent,
				Action:       PeerActionDomainClusterChanged,
				Payload:      map[string]interface{}{"invalid": true},
				Route: RoutePlan{Mode: RouteModeChain, Chain: []RouteStep{
					{StepID: "step-a", NodeID: "node-local", ContinueOnFailure: tt.continueOnFailure},
					{StepID: "step-b", NodeID: "node-b"},
				}},
			}

			err = dispatcher.Dispatch(context.Background(), &model.ClusterDomain{Id: 1, Domain: "edge.example.com"}, &model.ClusterMember{NodeID: "node-source"}, message)
			if tt.wantForwarded {
				if err != nil {
					t.Fatalf("expected successful ack after continuation, got %v", err)
				}
			} else if err == nil {
				t.Fatal("expected dispatch to return the step failure")
			}

			if len(saved) != 1 || saved[0].status != PeerEventStatusFailed || saved[0].errorMessage == "" {
				t.Fatalf("expected failed workflow step to be saved, got %#v", saved)
			}
			if tt.wantForwarded && forwardedCount != 1 {
				t.Fatalf("expected next step to be forwarded once, got %d", forwardedCount)
			}
			if !tt.wantForwarded && forwardedCount != 0 {
				t.Fatalf("expected no forwarded next step, got %d", forwardedCount)
			}
			if tt.wantForwarded {
				if forwarded.Action != PeerActionDomainClusterChanged {
					t.Fatalf("expected current action to be preserved, got %q", forwarded.Action)
				}
				if forwarded.Payload["invalid"] != true {
					t.Fatalf("expected current payload to be preserved, got %#v", forwarded.Payload)
				}
			}
		})
	}
}

func TestPeerDispatcherDoesNotDuplicateFailureContinuationOnRetry(t *testing.T) {
	secret := []byte("panel-secret-for-cluster-tests")
	local := newTestClusterLocalNode(t, "node-local")
	peerToken, err := EncryptClusterDomainToken(secret, "peer-token-b")
	if err != nil {
		t.Fatalf("encrypt peer token: %v", err)
	}

	forwardedCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forwardedCount++
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	store := newMemoryPeerStore()
	var saved []savedPeerWorkflowStep
	dispatcher := ClusterPeerDispatcher{
		eventStore: store,
		syncService: &ClusterSyncService{
			hubSyncer: &stubPeerSyncer{},
			store: &stubPeerDispatcherSyncStore{
				domain: &model.ClusterDomain{Id: 1, Domain: "edge.example.com"},
				member: &model.ClusterMember{NodeID: "node-b", DomainID: 1, BaseURL: server.URL, PeerTokenEncrypted: peerToken},
			},
		},
		identity:       ClusterLocalIdentityService{store: &stubClusterLocalNodeStore{node: local}},
		secretProvider: stubClusterSecretProvider{secret: secret},
		delivery:       &ClusterPeerDeliveryService{HTTPClient: server.Client()},
		saveWorkflowStep: func(workflowID string, stepID string, domainID string, nodeID string, status string, resultHash string, errorMessage string) error {
			saved = append(saved, savedPeerWorkflowStep{workflowID: workflowID, stepID: stepID, domainID: domainID, nodeID: nodeID, status: status, resultHash: resultHash, errorMessage: errorMessage})
			return nil
		},
	}

	message := failedContinueChainMessage("msg-chain-failure-idempotent")
	err = dispatcher.Dispatch(context.Background(), &model.ClusterDomain{Id: 1, Domain: "edge.example.com"}, &model.ClusterMember{NodeID: "node-source"}, message)
	if err != nil {
		t.Fatalf("expected successful ack after continuation, got %v", err)
	}
	if forwardedCount != 1 {
		t.Fatalf("expected next step to be forwarded once, got %d", forwardedCount)
	}
	if len(saved) != 1 || saved[0].status != PeerEventStatusFailed || saved[0].errorMessage != "invalid_payload_version" {
		t.Fatalf("expected failed workflow step to be saved, got %#v", saved)
	}

	err = dispatcher.Dispatch(context.Background(), &model.ClusterDomain{Id: 1, Domain: "edge.example.com"}, &model.ClusterMember{NodeID: "node-source"}, message)
	if err != nil {
		t.Fatalf("expected retry to be idempotent, got %v", err)
	}
	if forwardedCount != 1 {
		t.Fatalf("expected retry not to forward again, got %d forwards", forwardedCount)
	}
	if len(saved) != 1 {
		t.Fatalf("expected workflow step to be saved once, got %d", len(saved))
	}
	state, err := store.RecordReceived(message)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	if state.Status != PeerEventStatusSucceeded {
		t.Fatalf("expected terminal succeeded state after continuation, got %q", state.Status)
	}
}

type savedPeerWorkflowStep struct {
	workflowID   string
	stepID       string
	domainID     string
	nodeID       string
	status       string
	resultHash   string
	errorMessage string
}

func successfulChainMessage(messageID string) *PeerMessage {
	return &PeerMessage{
		MessageID:    messageID,
		WorkflowID:   "workflow-1",
		StepID:       "step-a",
		DomainID:     "edge.example.com",
		SourceNodeID: "node-source",
		SourceSeq:    41,
		PayloadHash:  "hash-current",
		Category:     PeerCategoryEvent,
		Action:       PeerActionDomainClusterChanged,
		Payload:      map[string]interface{}{"version": float64(9)},
		Route: RoutePlan{Mode: RouteModeChain, Chain: []RouteStep{
			{StepID: "step-a", NodeID: "node-local"},
			{StepID: "step-b", NodeID: "node-b", Action: "next.action", PayloadOverride: map[string]interface{}{"next": "payload"}},
		}},
	}
}

func failedContinueChainMessage(messageID string) *PeerMessage {
	return &PeerMessage{
		MessageID:    messageID,
		WorkflowID:   "workflow-1",
		StepID:       "step-a",
		DomainID:     "edge.example.com",
		SourceNodeID: "node-source",
		PayloadHash:  "hash-current",
		Category:     PeerCategoryEvent,
		Action:       PeerActionDomainClusterChanged,
		Payload:      map[string]interface{}{"invalid": true},
		Route: RoutePlan{Mode: RouteModeChain, Chain: []RouteStep{
			{StepID: "step-a", NodeID: "node-local", ContinueOnFailure: true},
			{StepID: "step-b", NodeID: "node-b"},
		}},
	}
}

type stubPeerSyncer struct{}

func (s *stubPeerSyncer) LatestVersion(context.Context, *model.ClusterDomain) (int64, error) {
	return 9, nil
}

func (s *stubPeerSyncer) SyncDomain(context.Context, *model.ClusterDomain, int64) error {
	return nil
}

type stubPeerDispatcherSyncStore struct {
	domain *model.ClusterDomain
	member *model.ClusterMember
}

func (s *stubPeerDispatcherSyncStore) GetMember(uint, string) (*model.ClusterMember, error) {
	if s.member != nil {
		return s.member, nil
	}
	return &model.ClusterMember{NodeID: "node-a", LastVersion: 1}, nil
}

func (s *stubPeerDispatcherSyncStore) SaveMember(member *model.ClusterMember) error {
	s.member = member
	return nil
}

func (s *stubPeerDispatcherSyncStore) ListMembers() ([]model.ClusterMember, error) {
	if s.member == nil {
		return nil, nil
	}
	return []model.ClusterMember{*s.member}, nil
}

func (s *stubPeerDispatcherSyncStore) GetDomain(uint) (*model.ClusterDomain, error) {
	return s.domain, nil
}

func (s *stubPeerDispatcherSyncStore) SaveDomain(domain *model.ClusterDomain) error {
	s.domain = domain
	return nil
}

func (s *stubPeerDispatcherSyncStore) ListDomains() ([]model.ClusterDomain, error) {
	if s.domain == nil {
		return nil, nil
	}
	return []model.ClusterDomain{*s.domain}, nil
}

func newTestClusterLocalNode(t *testing.T, nodeID string) *model.ClusterLocalNode {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate local key: %v", err)
	}
	return &model.ClusterLocalNode{
		NodeID:     nodeID,
		PublicKey:  base64.StdEncoding.EncodeToString(publicKey),
		PrivateKey: base64.StdEncoding.EncodeToString(privateKey),
	}
}
