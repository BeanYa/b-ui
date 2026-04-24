# Peer Communication Protocol Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the peer communication protocol foundation for signed, logged, idempotent direct peer messages, route plans, chain state, acknowledgements, and local scheduled broadcasts.

**Architecture:** Keep `b-cluster-hub` out of peer message forwarding. In `b-ui`, replace the current narrow `ClusterEnvelope` path with a versioned peer message envelope while preserving the existing `sync.notify_version` behavior as the first supported action. Persist peer logs and states through GORM models, then add services for verification, routing, delivery, chain progress, and schedule materialization.

**Tech Stack:** Go, Gin, GORM, SQLite, Ed25519, existing `ClusterService` and `ClusterSyncService`.

---

## Scope Check

The approved spec includes a protocol foundation and future business synchronization such as panel information CRUD. This plan implements the protocol foundation plus migration of the current cluster version notification into the new envelope. Panel information CRUD business behavior should be planned separately after this lands, because it touches panel ownership rules, UI/API workflows, and domain services beyond transport.

## File Map

- Create: `src/backend/internal/domain/services/cluster_peer_message.go`
  - Owns `PeerMessage`, `RoutePlan`, schedule, delivery, envelope validation, canonical payload hash, signing, and verification.
- Create: `src/backend/internal/domain/services/cluster_peer_message_test.go`
  - Unit tests for hash, signature, expiry, unsupported protocol, and duplicate-safe message identity.
- Create: `src/backend/internal/domain/services/cluster_peer_store.go`
  - Owns peer persistence interfaces and DB-backed helpers for event log, event state, workflow state, ack state, and schedules.
- Create: `src/backend/internal/domain/services/cluster_peer_store_test.go`
  - Tests store state transitions with an in-memory stub and DB-backed behavior where feasible.
- Create: `src/backend/internal/domain/services/cluster_peer_dispatcher.go`
  - Owns inbound dispatch by `category` and `action`, starting with `domain.cluster.changed`.
- Create: `src/backend/internal/domain/services/cluster_peer_dispatcher_test.go`
  - Tests inbound state transitions, unsupported action behavior, and membership refresh decisions.
- Create: `src/backend/internal/domain/services/cluster_peer_delivery.go`
  - Owns recipient expansion and HTTP delivery for direct, multicast, and broadcast route modes.
- Create: `src/backend/internal/domain/services/cluster_peer_delivery_test.go`
  - Tests route expansion, selector filtering, HTTPS validation reuse, and ack state updates.
- Create: `src/backend/internal/domain/services/cluster_peer_workflow.go`
  - Owns chain step persistence and next-step forwarding decisions.
- Create: `src/backend/internal/domain/services/cluster_peer_workflow_test.go`
  - Tests chain step success, failure stop, and continue-on-failure.
- Create: `src/backend/internal/domain/services/cluster_peer_schedule.go`
  - Owns due schedule selection and materialization into ordinary broadcast messages.
- Create: `src/backend/internal/domain/services/cluster_peer_schedule_test.go`
  - Tests once and interval schedules.
- Modify: `src/backend/internal/infra/db/model/model.go`
  - Add GORM models for peer log, state, ack, workflow, and schedule.
- Modify: `src/backend/internal/infra/db/db.go`
  - AutoMigrate the new peer models.
- Modify: `src/backend/internal/domain/services/cluster_sync.go`
  - Keep `ClusterEnvelope` compatibility temporarily, add `domain.cluster.changed` dispatch through the peer dispatcher.
- Modify: `src/backend/internal/domain/services/cluster_runtime.go`
  - Replace `ClusterHTTPBroadcaster.BroadcastNotifyVersion` internals with peer message creation and broadcast delivery.
- Modify: `src/backend/internal/domain/services/cluster_service.go`
  - Route `ReceiveMessage` through the peer dispatcher and keep the old envelope accepted during migration.
- Modify: `src/backend/internal/http/api/cluster.go`
  - Bind the new `PeerMessage` at `/_cluster/v1/events` and preserve old `ClusterEnvelope` compatibility.
- Modify: `src/backend/internal/domain/jobs/clusterVersionPollJob.go`
  - Keep membership polling unchanged.
- Create: `src/backend/internal/domain/jobs/clusterPeerScheduleJob.go`
  - Run local due schedules.
- Modify: `src/backend/internal/domain/jobs/cronJob.go`
  - Register the schedule job on the existing cron runner.

## Task 1: Add Peer Message Envelope and Signing

**Files:**
- Create: `src/backend/internal/domain/services/cluster_peer_message.go`
- Create: `src/backend/internal/domain/services/cluster_peer_message_test.go`
- Test: `src/backend/internal/domain/services/cluster_peer_message_test.go`

- [ ] **Step 1: Write failing tests for canonical hash and signature verification**

Create `src/backend/internal/domain/services/cluster_peer_message_test.go`:

```go
package service

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"
	"time"

	"github.com/alireza0/s-ui/src/backend/internal/infra/db/model"
)

func TestPeerMessagePayloadHashIsStable(t *testing.T) {
	payloadA := map[string]any{"version": float64(7), "domain": "edge.example.com"}
	payloadB := map[string]any{"domain": "edge.example.com", "version": float64(7)}

	hashA, err := ClusterPeerPayloadHash(payloadA)
	if err != nil {
		t.Fatalf("hash A: %v", err)
	}
	hashB, err := ClusterPeerPayloadHash(payloadB)
	if err != nil {
		t.Fatalf("hash B: %v", err)
	}
	if hashA != hashB {
		t.Fatalf("expected stable payload hash, got %q and %q", hashA, hashB)
	}
}

func TestSignAndVerifyPeerMessage(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	local := &model.ClusterLocalNode{
		NodeID:     "node-a",
		PublicKey:  base64.StdEncoding.EncodeToString(publicKey),
		PrivateKey: base64.StdEncoding.EncodeToString(privateKey),
	}
	message := NewClusterPeerMessage("edge.example.com", 5, "node-a", 1, "event", "domain.cluster.changed", map[string]any{"version": float64(5)})
	message.Route = RoutePlan{Mode: RouteModeBroadcast}

	if err := SignClusterPeerMessage(local, message); err != nil {
		t.Fatalf("sign: %v", err)
	}
	if err := VerifyClusterPeerMessage(message, local.PublicKey, time.Now().Unix()); err != nil {
		t.Fatalf("verify: %v", err)
	}
	message.Payload["version"] = float64(9)
	if err := VerifyClusterPeerMessage(message, local.PublicKey, time.Now().Unix()); err == nil {
		t.Fatal("expected tampered payload to fail verification")
	}
}

func TestVerifyPeerMessageRejectsExpiredMessage(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	local := &model.ClusterLocalNode{
		NodeID:     "node-a",
		PublicKey:  base64.StdEncoding.EncodeToString(publicKey),
		PrivateKey: base64.StdEncoding.EncodeToString(privateKey),
	}
	message := NewClusterPeerMessage("edge.example.com", 5, "node-a", 1, "event", "domain.cluster.changed", map[string]any{"version": float64(5)})
	message.ExpiresAt = 100
	if err := SignClusterPeerMessage(local, message); err != nil {
		t.Fatalf("sign: %v", err)
	}
	if err := VerifyClusterPeerMessage(message, local.PublicKey, 101); err == nil || err.Error() != "message_expired" {
		t.Fatalf("expected message_expired, got %v", err)
	}
}
```

- [ ] **Step 2: Run the new tests and confirm they fail**

Run:

```bash
go test ./src/backend/internal/domain/services -run 'TestPeerMessagePayloadHashIsStable|TestSignAndVerifyPeerMessage|TestVerifyPeerMessageRejectsExpiredMessage' -count=1
```

Expected: FAIL because `ClusterPeerPayloadHash`, `NewClusterPeerMessage`, `RoutePlan`, and signing helpers do not exist.

- [ ] **Step 3: Implement the peer message types and crypto helpers**

Create `src/backend/internal/domain/services/cluster_peer_message.go`:

```go
package service

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/alireza0/s-ui/src/backend/internal/infra/db/model"
	"github.com/gofrs/uuid/v5"
)

const ClusterPeerProtocolVersion = "v1"

const (
	RouteModeDirect             = "direct"
	RouteModeMulticast          = "multicast"
	RouteModeBroadcast          = "broadcast"
	RouteModeChain              = "chain"
	RouteModeScheduledBroadcast = "scheduled_broadcast"
)

type PeerMessage struct {
	MessageID         string                 `json:"messageId"`
	WorkflowID        string                 `json:"workflowId,omitempty"`
	StepID            string                 `json:"stepId,omitempty"`
	DomainID          string                 `json:"domainId"`
	MembershipVersion int64                  `json:"membershipVersion"`
	SourceNodeID      string                 `json:"sourceNodeId"`
	SourceSeq         int64                  `json:"sourceSeq"`
	Category          string                 `json:"category"`
	Action            string                 `json:"action"`
	ProtocolVersion   string                 `json:"protocolVersion"`
	SchemaVersion     int                    `json:"schemaVersion"`
	Route             RoutePlan              `json:"route"`
	IdempotencyKey    string                 `json:"idempotencyKey,omitempty"`
	CausationID       string                 `json:"causationId,omitempty"`
	CorrelationID     string                 `json:"correlationId,omitempty"`
	CreatedAt         int64                  `json:"createdAt"`
	ExpiresAt         int64                  `json:"expiresAt,omitempty"`
	PayloadHash       string                 `json:"payloadHash"`
	Payload           map[string]interface{} `json:"payload"`
	Signature         string                 `json:"signature"`
}

type RoutePlan struct {
	Mode     string         `json:"mode"`
	Targets  []string       `json:"targets,omitempty"`
	Selector TargetSelector `json:"selector,omitempty"`
	Chain    []RouteStep    `json:"chain,omitempty"`
	Delivery DeliveryPolicy `json:"delivery,omitempty"`
	Schedule SchedulePolicy `json:"schedule,omitempty"`
}

type TargetSelector struct {
	Include            []string `json:"include,omitempty"`
	Exclude            []string `json:"exclude,omitempty"`
	CapabilityRequired []string `json:"capabilityRequired,omitempty"`
}

type RouteStep struct {
	StepID            string                 `json:"stepId"`
	NodeID            string                 `json:"nodeId"`
	Action            string                 `json:"action,omitempty"`
	PayloadOverride   map[string]interface{} `json:"payloadOverride,omitempty"`
	ContinueOnFailure bool                   `json:"continueOnFailure,omitempty"`
}

type DeliveryPolicy struct {
	Ack       string      `json:"ack,omitempty"`
	TimeoutMS int64      `json:"timeoutMs,omitempty"`
	Retry     RetryPolicy `json:"retry,omitempty"`
	MaxHops   int         `json:"maxHops,omitempty"`
}

type RetryPolicy struct {
	MaxAttempts int   `json:"maxAttempts,omitempty"`
	BackoffMS   int64 `json:"backoffMs,omitempty"`
}

type SchedulePolicy struct {
	Kind       string `json:"kind,omitempty"`
	RunAt      int64  `json:"runAt,omitempty"`
	IntervalMS int64  `json:"intervalMs,omitempty"`
	Cron       string `json:"cron,omitempty"`
	MaxRuns    int    `json:"maxRuns,omitempty"`
	ExpiresAt  int64  `json:"expiresAt,omitempty"`
}

func NewClusterPeerMessage(domain string, membershipVersion int64, sourceNodeID string, sourceSeq int64, category string, action string, payload map[string]interface{}) *PeerMessage {
	id, _ := uuid.NewV4()
	now := time.Now().Unix()
	return &PeerMessage{
		MessageID:         id.String(),
		DomainID:          domain,
		MembershipVersion: membershipVersion,
		SourceNodeID:      sourceNodeID,
		SourceSeq:         sourceSeq,
		Category:          category,
		Action:            action,
		ProtocolVersion:   ClusterPeerProtocolVersion,
		SchemaVersion:     1,
		CreatedAt:         now,
		Payload:           payload,
	}
}

func ClusterPeerPayloadHash(payload map[string]interface{}) (string, error) {
	canonical, err := canonicalJSON(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

func SignClusterPeerMessage(local *model.ClusterLocalNode, message *PeerMessage) error {
	if message.ProtocolVersion == "" {
		message.ProtocolVersion = ClusterPeerProtocolVersion
	}
	hash, err := ClusterPeerPayloadHash(message.Payload)
	if err != nil {
		return err
	}
	message.PayloadHash = hash
	privateKeyRaw, err := base64.StdEncoding.DecodeString(local.PrivateKey)
	if err != nil {
		return err
	}
	payload, err := clusterPeerSigningPayload(message)
	if err != nil {
		return err
	}
	message.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(ed25519.PrivateKey(privateKeyRaw), payload))
	return nil
}

func VerifyClusterPeerMessage(message *PeerMessage, publicKey string, now int64) error {
	if message.ProtocolVersion != ClusterPeerProtocolVersion {
		return errors.New("unsupported_protocol_version")
	}
	if message.ExpiresAt > 0 && now > message.ExpiresAt {
		return errors.New("message_expired")
	}
	hash, err := ClusterPeerPayloadHash(message.Payload)
	if err != nil {
		return err
	}
	if hash != message.PayloadHash {
		return errors.New("payload_hash_mismatch")
	}
	publicKeyRaw, err := base64.StdEncoding.DecodeString(publicKey)
	if err != nil {
		return err
	}
	signatureRaw, err := base64.StdEncoding.DecodeString(message.Signature)
	if err != nil {
		return err
	}
	payload, err := clusterPeerSigningPayload(message)
	if err != nil {
		return err
	}
	if !ed25519.Verify(ed25519.PublicKey(publicKeyRaw), payload, signatureRaw) {
		return errors.New("invalid_signature")
	}
	return nil
}

func clusterPeerSigningPayload(message *PeerMessage) ([]byte, error) {
	unsigned := *message
	unsigned.Signature = ""
	return canonicalJSON(unsigned)
}

func canonicalJSON(value interface{}) ([]byte, error) {
	normalized := normalizeJSONValue(value)
	return json.Marshal(normalized)
}

func normalizeJSONValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		ordered := make(map[string]interface{}, len(typed))
		for _, key := range keys {
			ordered[key] = normalizeJSONValue(typed[key])
		}
		return ordered
	case []interface{}:
		items := make([]interface{}, len(typed))
		for i := range typed {
			items[i] = normalizeJSONValue(typed[i])
		}
		return items
	default:
		return typed
	}
}
```

- [ ] **Step 4: Run the focused tests and commit**

Run:

```bash
go test ./src/backend/internal/domain/services -run 'TestPeerMessagePayloadHashIsStable|TestSignAndVerifyPeerMessage|TestVerifyPeerMessageRejectsExpiredMessage' -count=1
```

Expected: PASS.

Commit:

```bash
git add src/backend/internal/domain/services/cluster_peer_message.go src/backend/internal/domain/services/cluster_peer_message_test.go
git commit -m "feat: add peer message envelope"
```

## Task 2: Add Peer Persistence Models and Store

**Files:**
- Modify: `src/backend/internal/infra/db/model/model.go`
- Modify: `src/backend/internal/infra/db/db.go`
- Create: `src/backend/internal/domain/services/cluster_peer_store.go`
- Create: `src/backend/internal/domain/services/cluster_peer_store_test.go`
- Test: `src/backend/internal/domain/services/cluster_peer_store_test.go`

- [ ] **Step 1: Write failing tests for event state transitions**

Create `src/backend/internal/domain/services/cluster_peer_store_test.go`:

```go
package service

import "testing"

func TestMemoryPeerStoreRejectsDifferentPayloadForSameMessage(t *testing.T) {
	store := newMemoryPeerStore()
	first := &PeerMessage{MessageID: "msg-1", DomainID: "edge.example.com", PayloadHash: "hash-a", Action: "domain.cluster.changed"}
	second := &PeerMessage{MessageID: "msg-1", DomainID: "edge.example.com", PayloadHash: "hash-b", Action: "domain.cluster.changed"}

	state, err := store.RecordReceived(first)
	if err != nil {
		t.Fatalf("record first: %v", err)
	}
	if state.Status != PeerEventStatusReceived {
		t.Fatalf("expected received, got %q", state.Status)
	}
	if _, err := store.RecordReceived(second); err == nil || err.Error() != "payload_hash_mismatch" {
		t.Fatalf("expected payload_hash_mismatch, got %v", err)
	}
}

func TestMemoryPeerStoreReturnsExistingSucceededDuplicate(t *testing.T) {
	store := newMemoryPeerStore()
	message := &PeerMessage{MessageID: "msg-1", DomainID: "edge.example.com", PayloadHash: "hash-a", Action: "domain.cluster.changed"}
	if _, err := store.RecordReceived(message); err != nil {
		t.Fatalf("record: %v", err)
	}
	if err := store.MarkEventState("msg-1", PeerEventStatusSucceeded, ""); err != nil {
		t.Fatalf("mark succeeded: %v", err)
	}
	state, err := store.RecordReceived(message)
	if err != nil {
		t.Fatalf("record duplicate: %v", err)
	}
	if state.Status != PeerEventStatusSucceeded {
		t.Fatalf("expected succeeded duplicate, got %q", state.Status)
	}
}
```

- [ ] **Step 2: Run the store tests and confirm they fail**

Run:

```bash
go test ./src/backend/internal/domain/services -run 'TestMemoryPeerStore' -count=1
```

Expected: FAIL because peer store types do not exist.

- [ ] **Step 3: Add GORM models**

Append to `src/backend/internal/infra/db/model/model.go`:

```go
type ClusterPeerEventLog struct {
	Id          uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	MessageID   string `json:"messageId" gorm:"uniqueIndex"`
	DomainID    string `json:"domainId" gorm:"index"`
	Direction   string `json:"direction"`
	SourceNode  string `json:"sourceNode" gorm:"index"`
	Action      string `json:"action" gorm:"index"`
	Envelope    string `json:"envelope"`
	PayloadHash string `json:"payloadHash"`
	Signature   string `json:"signature"`
	CreatedAt   int64  `json:"createdAt" gorm:"index"`
}

type ClusterPeerEventState struct {
	Id             uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	MessageID      string `json:"messageId" gorm:"uniqueIndex"`
	IdempotencyKey string `json:"idempotencyKey" gorm:"index"`
	SourceNode     string `json:"sourceNode" gorm:"index"`
	SourceSeq      int64  `json:"sourceSeq" gorm:"index"`
	DomainID       string `json:"domainId" gorm:"index"`
	Action         string `json:"action" gorm:"index"`
	PayloadHash    string `json:"payloadHash"`
	Status         string `json:"status" gorm:"index"`
	Error          string `json:"error"`
	CreatedAt      int64  `json:"createdAt"`
	UpdatedAt      int64  `json:"updatedAt"`
}

type ClusterPeerAckState struct {
	Id          uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	MessageID   string `json:"messageId" gorm:"index"`
	TargetNode  string `json:"targetNode" gorm:"index"`
	Status      string `json:"status" gorm:"index"`
	Attempts    int    `json:"attempts"`
	NextRetryAt int64  `json:"nextRetryAt" gorm:"index"`
	Error       string `json:"error"`
	UpdatedAt   int64  `json:"updatedAt"`
}

type ClusterPeerWorkflowState struct {
	Id          uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	WorkflowID  string `json:"workflowId" gorm:"index"`
	StepID      string `json:"stepId" gorm:"index"`
	DomainID    string `json:"domainId" gorm:"index"`
	NodeID      string `json:"nodeId" gorm:"index"`
	Status      string `json:"status" gorm:"index"`
	ResultHash  string `json:"resultHash"`
	Error       string `json:"error"`
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`
}

type ClusterPeerSchedule struct {
	Id          uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	ScheduleID  string `json:"scheduleId" gorm:"uniqueIndex"`
	DomainID    string `json:"domainId" gorm:"index"`
	OwnerNodeID string `json:"ownerNodeId" gorm:"index"`
	Action      string `json:"action" gorm:"index"`
	RouteJSON   string `json:"routeJson"`
	PayloadJSON string `json:"payloadJson"`
	NextRunAt   int64  `json:"nextRunAt" gorm:"index"`
	LastRunAt   int64  `json:"lastRunAt"`
	RunCount    int    `json:"runCount"`
	MaxRuns     int    `json:"maxRuns"`
	Enabled     bool   `json:"enabled" gorm:"index"`
}
```

- [ ] **Step 4: AutoMigrate the new models**

Update `src/backend/internal/infra/db/db.go` by adding these entries to `db.AutoMigrate(...)` after `ClusterMember`:

```go
&model.ClusterPeerEventLog{},
&model.ClusterPeerEventState{},
&model.ClusterPeerAckState{},
&model.ClusterPeerWorkflowState{},
&model.ClusterPeerSchedule{},
```

- [ ] **Step 5: Implement the store interface and memory store used by tests**

Create `src/backend/internal/domain/services/cluster_peer_store.go`:

```go
package service

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	database "github.com/alireza0/s-ui/src/backend/internal/infra/db"
	"github.com/alireza0/s-ui/src/backend/internal/infra/db/model"
)

const (
	PeerEventStatusReceived    = "received"
	PeerEventStatusProcessing  = "processing"
	PeerEventStatusSucceeded   = "succeeded"
	PeerEventStatusFailed      = "failed"
	PeerEventStatusIgnored     = "ignored"
	PeerEventStatusUnsupported = "unsupported"
	PeerEventStatusDead        = "dead"
)

type PeerEventState struct {
	MessageID   string
	PayloadHash string
	Status      string
}

type clusterPeerStore interface {
	RecordReceived(*PeerMessage) (*PeerEventState, error)
	MarkEventState(messageID string, status string, errorMessage string) error
}

type dbClusterPeerStore struct{}

func (s *dbClusterPeerStore) RecordReceived(message *PeerMessage) (*PeerEventState, error) {
	now := time.Now().Unix()
	var existing model.ClusterPeerEventState
	err := database.GetDB().Where("message_id = ?", message.MessageID).First(&existing).Error
	if err == nil {
		if existing.PayloadHash != message.PayloadHash {
			return nil, errors.New("payload_hash_mismatch")
		}
		return &PeerEventState{MessageID: existing.MessageID, PayloadHash: existing.PayloadHash, Status: existing.Status}, nil
	}
	if !database.IsNotFound(err) {
		return nil, err
	}
	envelope, err := json.Marshal(message)
	if err != nil {
		return nil, err
	}
	tx := database.GetDB().Begin()
	state := model.ClusterPeerEventState{
		MessageID:      message.MessageID,
		IdempotencyKey: message.IdempotencyKey,
		SourceNode:     message.SourceNodeID,
		SourceSeq:      message.SourceSeq,
		DomainID:       message.DomainID,
		Action:         message.Action,
		PayloadHash:    message.PayloadHash,
		Status:         PeerEventStatusReceived,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := tx.Create(&state).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	log := model.ClusterPeerEventLog{
		MessageID:   message.MessageID,
		DomainID:    message.DomainID,
		Direction:   "inbound",
		SourceNode:  message.SourceNodeID,
		Action:      message.Action,
		Envelope:    string(envelope),
		PayloadHash: message.PayloadHash,
		Signature:   message.Signature,
		CreatedAt:   now,
	}
	if err := tx.Create(&log).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	return &PeerEventState{MessageID: state.MessageID, PayloadHash: state.PayloadHash, Status: state.Status}, nil
}

func (s *dbClusterPeerStore) MarkEventState(messageID string, status string, errorMessage string) error {
	return database.GetDB().Model(&model.ClusterPeerEventState{}).
		Where("message_id = ?", messageID).
		Updates(map[string]interface{}{"status": status, "error": errorMessage, "updated_at": time.Now().Unix()}).Error
}

type memoryPeerStore struct {
	mu     sync.Mutex
	states map[string]*PeerEventState
}

func newMemoryPeerStore() *memoryPeerStore {
	return &memoryPeerStore{states: map[string]*PeerEventState{}}
}

func (s *memoryPeerStore) RecordReceived(message *PeerMessage) (*PeerEventState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.states[message.MessageID]; ok {
		if existing.PayloadHash != message.PayloadHash {
			return nil, errors.New("payload_hash_mismatch")
		}
		copy := *existing
		return &copy, nil
	}
	state := &PeerEventState{MessageID: message.MessageID, PayloadHash: message.PayloadHash, Status: PeerEventStatusReceived}
	s.states[message.MessageID] = state
	copy := *state
	return &copy, nil
}

func (s *memoryPeerStore) MarkEventState(messageID string, status string, errorMessage string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.states[messageID]
	if !ok {
		return errors.New("peer event state not found")
	}
	state.Status = status
	return nil
}
```

- [ ] **Step 6: Run tests and commit**

Run:

```bash
go test ./src/backend/internal/domain/services -run 'TestMemoryPeerStore' -count=1
```

Expected: PASS.

Commit:

```bash
git add src/backend/internal/infra/db/model/model.go src/backend/internal/infra/db/db.go src/backend/internal/domain/services/cluster_peer_store.go src/backend/internal/domain/services/cluster_peer_store_test.go
git commit -m "feat: persist peer message state"
```

## Task 3: Dispatch Incoming Peer Messages

**Files:**
- Create: `src/backend/internal/domain/services/cluster_peer_dispatcher.go`
- Create: `src/backend/internal/domain/services/cluster_peer_dispatcher_test.go`
- Modify: `src/backend/internal/domain/services/cluster_service.go`
- Test: `src/backend/internal/domain/services/cluster_peer_dispatcher_test.go`

- [ ] **Step 1: Write failing dispatcher tests**

Create `src/backend/internal/domain/services/cluster_peer_dispatcher_test.go`:

```go
package service

import (
	"context"
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
```

Add local stubs in the same file:

```go
type stubPeerSyncer struct{}

func (s *stubPeerSyncer) LatestVersion(context.Context, *model.ClusterDomain) (int64, error) {
	return 9, nil
}

func (s *stubPeerSyncer) SyncDomain(context.Context, *model.ClusterDomain, int64) error {
	return nil
}
```

Add this local `clusterSyncStore` stub in the same test file:

```go
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
```

- [ ] **Step 2: Run dispatcher tests and confirm they fail**

Run:

```bash
go test ./src/backend/internal/domain/services -run 'TestPeerDispatcher' -count=1
```

Expected: FAIL because `ClusterPeerDispatcher` does not exist.

- [ ] **Step 3: Implement dispatcher**

Create `src/backend/internal/domain/services/cluster_peer_dispatcher.go`:

```go
package service

import (
	"context"
	"errors"

	"github.com/alireza0/s-ui/src/backend/internal/infra/db/model"
)

const (
	PeerCategoryCommand  = "command"
	PeerCategoryEvent    = "event"
	PeerCategoryQuery    = "query"
	PeerCategoryResponse = "response"
)

const PeerActionDomainClusterChanged = "domain.cluster.changed"

type ClusterPeerDispatcher struct {
	eventStore  clusterPeerStore
	syncService *ClusterSyncService
}

func (d *ClusterPeerDispatcher) Dispatch(ctx context.Context, domain *model.ClusterDomain, source *model.ClusterMember, message *PeerMessage) error {
	store := d.getStore()
	state, err := store.RecordReceived(message)
	if err != nil {
		return err
	}
	if state.Status == PeerEventStatusSucceeded || state.Status == PeerEventStatusUnsupported || state.Status == PeerEventStatusIgnored {
		return nil
	}
	if err := store.MarkEventState(message.MessageID, PeerEventStatusProcessing, ""); err != nil {
		return err
	}
	switch message.Action {
	case PeerActionDomainClusterChanged:
		err = d.handleDomainClusterChanged(ctx, domain, source, message)
	default:
		if message.Category == PeerCategoryEvent {
			return store.MarkEventState(message.MessageID, PeerEventStatusUnsupported, "")
		}
		err = errors.New("unsupported_action")
	}
	if err != nil {
		_ = store.MarkEventState(message.MessageID, PeerEventStatusFailed, err.Error())
		return err
	}
	return store.MarkEventState(message.MessageID, PeerEventStatusSucceeded, "")
}

func (d *ClusterPeerDispatcher) handleDomainClusterChanged(ctx context.Context, domain *model.ClusterDomain, source *model.ClusterMember, message *PeerMessage) error {
	versionValue, ok := message.Payload["version"].(float64)
	if !ok {
		return errors.New("invalid_domain_cluster_changed_payload")
	}
	syncService := d.syncService
	if syncService == nil {
		runtime := NewRuntimeClusterSyncService()
		syncService = &runtime
	}
	_, err := syncService.HandleIncomingNotifyVersion(ctx, domain.Id, source.NodeID, int64(versionValue))
	return err
}

func (d *ClusterPeerDispatcher) getStore() clusterPeerStore {
	if d.eventStore != nil {
		return d.eventStore
	}
	return &dbClusterPeerStore{}
}
```

- [ ] **Step 4: Add a new receive method while keeping old compatibility**

In `src/backend/internal/domain/services/cluster_service.go`, add:

```go
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
	dispatcher := ClusterPeerDispatcher{syncService: &s.syncService}
	return dispatcher.Dispatch(context.Background(), domain, member, message)
}
```

Add `time` to the existing import block in `src/backend/internal/domain/services/cluster_service.go`.

- [ ] **Step 5: Run tests and commit**

Run:

```bash
go test ./src/backend/internal/domain/services -run 'TestPeerDispatcher|TestCluster' -count=1
```

Expected: PASS for peer dispatcher and existing cluster service tests.

Commit:

```bash
git add src/backend/internal/domain/services/cluster_peer_dispatcher.go src/backend/internal/domain/services/cluster_peer_dispatcher_test.go src/backend/internal/domain/services/cluster_service.go
git commit -m "feat: dispatch peer messages"
```

## Task 4: Accept New Peer Messages on the Cluster HTTP Endpoint

**Files:**
- Modify: `src/backend/internal/http/api/cluster.go`
- Modify: `src/backend/internal/domain/services/cluster_service.go`
- Modify: `src/backend/internal/domain/services/cluster_http_test.go`
- Test: `src/backend/internal/domain/services/cluster_http_test.go`

- [ ] **Step 1: Add failing HTTP route test for new envelope**

In `src/backend/internal/domain/services/cluster_http_test.go`, add a test that posts a `PeerMessage` to the route helper already used by existing cluster HTTP tests:

```go
func TestClusterMessageRouteAcceptsPeerMessage(t *testing.T) {
	service := &stubClusterAPIService{}
	router := gin.New()
	api.RegisterClusterMessageRoute(router, service)

	body := `{
	  "messageId":"msg-1",
	  "domainId":"edge.example.com",
	  "membershipVersion":3,
	  "sourceNodeId":"node-a",
	  "sourceSeq":1,
	  "category":"event",
	  "action":"domain.cluster.changed",
	  "protocolVersion":"v1",
	  "schemaVersion":1,
	  "route":{"mode":"broadcast"},
	  "payloadHash":"hash",
	  "payload":{"version":3},
	  "signature":"sig"
	}`
	request := httptest.NewRequest(http.MethodPost, api.ClusterMessagePath("/"), strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Cluster-Token", "peer-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if service.lastPeerMessage == nil || service.lastPeerMessage.Action != "domain.cluster.changed" {
		t.Fatalf("expected peer message to be passed to service")
	}
}
```

Extend the local HTTP test stub with:

```go
lastPeerMessage *PeerMessage

func (s *stubClusterAPIService) ReceivePeerMessage(message *PeerMessage, token string) error {
	s.lastPeerMessage = message
	return nil
}
```

- [ ] **Step 2: Run the HTTP test and confirm it fails**

Run:

```bash
go test ./src/backend/internal/domain/services ./src/backend/internal/http/api -run 'TestClusterMessageRouteAcceptsPeerMessage' -count=1
```

Expected: FAIL because `clusterAPIService` does not expose `ReceivePeerMessage` and route binding only uses `ClusterEnvelope`.

- [ ] **Step 3: Update the API service interface and route binding**

In `src/backend/internal/http/api/cluster.go`, change `clusterAPIService`:

```go
ReceivePeerMessage(*service.PeerMessage, string) error
ReceiveMessage(*service.ClusterEnvelope, string) error
```

Update `RegisterClusterMessageRoute`:

```go
router.POST(ClusterMessagePath("/"), func(c *gin.Context) {
	var raw map[string]interface{}
	if err := c.ShouldBindJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, Msg{Success: false, Msg: "cluster message: " + err.Error()})
		return
	}
	encoded, _ := json.Marshal(raw)
	if _, ok := raw["protocolVersion"]; ok {
		var message service.PeerMessage
		if err := json.Unmarshal(encoded, &message); err != nil {
			c.JSON(http.StatusBadRequest, Msg{Success: false, Msg: "cluster message: " + err.Error()})
			return
		}
		err := clusterService.ReceivePeerMessage(&message, c.GetHeader("X-Cluster-Token"))
		if err != nil {
			c.JSON(http.StatusUnauthorized, Msg{Success: false, Msg: clusterMessage(err)})
			return
		}
		c.JSON(http.StatusOK, Msg{Success: true, Msg: clusterMessage(nil)})
		return
	}
	var envelope service.ClusterEnvelope
	if err := json.Unmarshal(encoded, &envelope); err != nil {
		c.JSON(http.StatusBadRequest, Msg{Success: false, Msg: "cluster message: " + err.Error()})
		return
	}
	err := clusterService.ReceiveMessage(&envelope, c.GetHeader("X-Cluster-Token"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, Msg{Success: false, Msg: clusterMessage(err)})
		return
	}
	c.JSON(http.StatusOK, Msg{Success: true, Msg: clusterMessage(nil)})
})
```

Add `encoding/json` to imports.

- [ ] **Step 4: Run tests and commit**

Run:

```bash
go test ./src/backend/internal/domain/services ./src/backend/internal/http/api -run 'TestClusterMessageRouteAcceptsPeerMessage|TestCluster' -count=1
```

Expected: PASS.

Commit:

```bash
git add src/backend/internal/http/api/cluster.go src/backend/internal/domain/services/cluster_service.go src/backend/internal/domain/services/cluster_http_test.go
git commit -m "feat: accept peer message envelope"
```

## Task 5: Deliver Direct, Multicast, and Broadcast Messages

**Files:**
- Create: `src/backend/internal/domain/services/cluster_peer_delivery.go`
- Create: `src/backend/internal/domain/services/cluster_peer_delivery_test.go`
- Modify: `src/backend/internal/domain/services/cluster_runtime.go`
- Test: `src/backend/internal/domain/services/cluster_peer_delivery_test.go`

- [ ] **Step 1: Write failing route expansion tests**

Create `src/backend/internal/domain/services/cluster_peer_delivery_test.go`:

```go
package service

import (
	"testing"

	"github.com/alireza0/s-ui/src/backend/internal/infra/db/model"
)

func TestExpandPeerRouteBroadcastSkipsSourceAndExcludedNodes(t *testing.T) {
	members := []model.ClusterMember{
		{NodeID: "node-a", BaseURL: "https://node-a.example.com"},
		{NodeID: "node-b", BaseURL: "https://node-b.example.com"},
		{NodeID: "node-c", BaseURL: "https://node-c.example.com"},
	}
	targets := ExpandClusterPeerRoute(RoutePlan{
		Mode: RouteModeBroadcast,
		Selector: TargetSelector{
			Exclude: []string{"node-c"},
		},
	}, members, "node-a")
	if len(targets) != 1 || targets[0].NodeID != "node-b" {
		t.Fatalf("expected only node-b, got %#v", targets)
	}
}

func TestExpandPeerRouteMulticastUsesFixedTargets(t *testing.T) {
	members := []model.ClusterMember{
		{NodeID: "node-a", BaseURL: "https://node-a.example.com"},
		{NodeID: "node-b", BaseURL: "https://node-b.example.com"},
		{NodeID: "node-c", BaseURL: "https://node-c.example.com"},
	}
	targets := ExpandClusterPeerRoute(RoutePlan{Mode: RouteModeMulticast, Targets: []string{"node-c", "node-b"}}, members, "node-a")
	if len(targets) != 2 || targets[0].NodeID != "node-c" || targets[1].NodeID != "node-b" {
		t.Fatalf("expected fixed multicast order c,b, got %#v", targets)
	}
}
```

- [ ] **Step 2: Run the route expansion tests and confirm they fail**

Run:

```bash
go test ./src/backend/internal/domain/services -run 'TestExpandPeerRoute' -count=1
```

Expected: FAIL because `ExpandClusterPeerRoute` does not exist.

- [ ] **Step 3: Implement route expansion and delivery skeleton**

Create `src/backend/internal/domain/services/cluster_peer_delivery.go`:

```go
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"time"

	"github.com/alireza0/s-ui/src/backend/internal/infra/db/model"
)

type ClusterPeerDeliveryService struct {
	HTTPClient *http.Client
}

func ExpandClusterPeerRoute(route RoutePlan, members []model.ClusterMember, sourceNodeID string) []model.ClusterMember {
	byNode := map[string]model.ClusterMember{}
	for _, member := range members {
		if member.NodeID == "" || member.BaseURL == "" || member.NodeID == sourceNodeID {
			continue
		}
		if len(route.Selector.Include) > 0 && !slices.Contains(route.Selector.Include, member.NodeID) {
			continue
		}
		if slices.Contains(route.Selector.Exclude, member.NodeID) {
			continue
		}
		byNode[member.NodeID] = member
	}
	switch route.Mode {
	case RouteModeDirect, RouteModeMulticast:
		targets := make([]model.ClusterMember, 0, len(route.Targets))
		for _, nodeID := range route.Targets {
			if member, ok := byNode[nodeID]; ok {
				targets = append(targets, member)
			}
		}
		return targets
	case RouteModeBroadcast, RouteModeScheduledBroadcast:
		targets := make([]model.ClusterMember, 0, len(byNode))
		for _, member := range members {
			if selected, ok := byNode[member.NodeID]; ok {
				targets = append(targets, selected)
			}
		}
		return targets
	default:
		return nil
	}
}

func (s *ClusterPeerDeliveryService) Send(ctx context.Context, message *PeerMessage, member model.ClusterMember, token string) error {
	body, err := json.Marshal(message)
	if err != nil {
		return err
	}
	messageURL, err := clusterPeerMessageURL(member.BaseURL)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, messageURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Cluster-Token", token)
	response, err := s.httpClient().Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if err := requireHTTPSuccess(response, "cluster peer message"); err != nil {
		return err
	}
	return requireClusterPeerSuccess(response)
}

func (s *ClusterPeerDeliveryService) httpClient() *http.Client {
	if s.HTTPClient != nil {
		return s.HTTPClient
	}
	return &http.Client{Timeout: 10 * time.Second}
}
```

- [ ] **Step 4: Migrate `BroadcastNotifyVersion` to produce peer messages**

In `src/backend/internal/domain/services/cluster_runtime.go`, update `ClusterCommunicationSupportedActions`:

```go
return []string{"domain.cluster.changed", "events", "heartbeat", "ping"}
```

Inside `BroadcastNotifyVersion`, replace the old `SignClusterNotifyVersionEnvelope` call with:

```go
message := NewClusterPeerMessage(member.Domain.Domain, version, identity.NodeID, time.Now().UnixNano(), PeerCategoryEvent, PeerActionDomainClusterChanged, map[string]interface{}{"version": float64(version)})
message.Route = RoutePlan{Mode: RouteModeBroadcast, Delivery: DeliveryPolicy{Ack: "node", TimeoutMS: 10000, Retry: RetryPolicy{MaxAttempts: 3, BackoffMS: 1000}}}
if err := SignClusterPeerMessage(identity, message); err != nil {
	return err
}
body, err := json.Marshal(message)
```

Keep `clusterPeerMessageURL`, token decrypt, request construction, and response handling unchanged.

- [ ] **Step 5: Run tests and commit**

Run:

```bash
go test ./src/backend/internal/domain/services -run 'TestExpandPeerRoute|TestClusterHTTPBroadcaster|TestCluster' -count=1
```

Expected: PASS.

Commit:

```bash
git add src/backend/internal/domain/services/cluster_peer_delivery.go src/backend/internal/domain/services/cluster_peer_delivery_test.go src/backend/internal/domain/services/cluster_runtime.go
git commit -m "feat: deliver peer messages by route"
```

## Task 6: Add Chain Workflow State

**Files:**
- Create: `src/backend/internal/domain/services/cluster_peer_workflow.go`
- Create: `src/backend/internal/domain/services/cluster_peer_workflow_test.go`
- Test: `src/backend/internal/domain/services/cluster_peer_workflow_test.go`

- [ ] **Step 1: Write failing chain decision tests**

Create `src/backend/internal/domain/services/cluster_peer_workflow_test.go`:

```go
package service

import "testing"

func TestNextChainStepStopsOnFailureByDefault(t *testing.T) {
	route := RoutePlan{Mode: RouteModeChain, Chain: []RouteStep{
		{StepID: "step-a", NodeID: "node-a"},
		{StepID: "step-b", NodeID: "node-b"},
	}}
	next, ok := NextClusterPeerChainStep(route, "step-a", false)
	if ok || next.StepID != "" {
		t.Fatalf("expected chain to stop on failure, got %#v %v", next, ok)
	}
}

func TestNextChainStepContinuesOnSuccess(t *testing.T) {
	route := RoutePlan{Mode: RouteModeChain, Chain: []RouteStep{
		{StepID: "step-a", NodeID: "node-a"},
		{StepID: "step-b", NodeID: "node-b"},
	}}
	next, ok := NextClusterPeerChainStep(route, "step-a", true)
	if !ok || next.StepID != "step-b" {
		t.Fatalf("expected step-b, got %#v %v", next, ok)
	}
}
```

- [ ] **Step 2: Run chain tests and confirm they fail**

Run:

```bash
go test ./src/backend/internal/domain/services -run 'TestNextChainStep' -count=1
```

Expected: FAIL because `NextClusterPeerChainStep` does not exist.

- [ ] **Step 3: Implement chain next-step decision**

Create `src/backend/internal/domain/services/cluster_peer_workflow.go`:

```go
package service

func NextClusterPeerChainStep(route RoutePlan, currentStepID string, currentSucceeded bool) (RouteStep, bool) {
	if route.Mode != RouteModeChain {
		return RouteStep{}, false
	}
	for index, step := range route.Chain {
		if step.StepID != currentStepID {
			continue
		}
		if !currentSucceeded && !step.ContinueOnFailure {
			return RouteStep{}, false
		}
		nextIndex := index + 1
		if nextIndex >= len(route.Chain) {
			return RouteStep{}, false
		}
		return route.Chain[nextIndex], true
	}
	return RouteStep{}, false
}
```

- [ ] **Step 4: Add workflow persistence in `cluster_peer_store.go`**

Add a method to persist workflow step status:

```go
func SaveClusterPeerWorkflowStep(workflowID string, stepID string, domainID string, nodeID string, status string, resultHash string, errorMessage string) error {
	now := time.Now().Unix()
	return database.GetDB().Where(model.ClusterPeerWorkflowState{WorkflowID: workflowID, StepID: stepID}).
		Assign(model.ClusterPeerWorkflowState{
			DomainID:   domainID,
			NodeID:     nodeID,
			Status:     status,
			ResultHash: resultHash,
			Error:      errorMessage,
			UpdatedAt:  now,
		}).
		FirstOrCreate(&model.ClusterPeerWorkflowState{
			WorkflowID: workflowID,
			StepID:     stepID,
			DomainID:   domainID,
			NodeID:     nodeID,
			Status:     status,
			ResultHash: resultHash,
			Error:      errorMessage,
			CreatedAt:  now,
			UpdatedAt:  now,
		}).Error
}
```

- [ ] **Step 5: Run tests and commit**

Run:

```bash
go test ./src/backend/internal/domain/services -run 'TestNextChainStep|TestMemoryPeerStore' -count=1
```

Expected: PASS.

Commit:

```bash
git add src/backend/internal/domain/services/cluster_peer_workflow.go src/backend/internal/domain/services/cluster_peer_workflow_test.go src/backend/internal/domain/services/cluster_peer_store.go
git commit -m "feat: track peer chain workflows"
```

## Task 7: Add Local Scheduled Broadcast Materialization

**Files:**
- Create: `src/backend/internal/domain/services/cluster_peer_schedule.go`
- Create: `src/backend/internal/domain/services/cluster_peer_schedule_test.go`
- Create: `src/backend/internal/domain/jobs/clusterPeerScheduleJob.go`
- Modify: `src/backend/internal/domain/jobs/cronJob.go`
- Test: `src/backend/internal/domain/services/cluster_peer_schedule_test.go`

- [ ] **Step 1: Write failing schedule materialization tests**

Create `src/backend/internal/domain/services/cluster_peer_schedule_test.go`:

```go
package service

import "testing"

func TestNextScheduleRunDisablesOnceSchedule(t *testing.T) {
	schedule := PeerScheduleState{Kind: "once", RunCount: 0, MaxRuns: 1, IntervalMS: 0, NextRunAt: 100}
	next, enabled := NextPeerScheduleRun(schedule, 100)
	if enabled || next != 0 {
		t.Fatalf("expected disabled once schedule, got next=%d enabled=%v", next, enabled)
	}
}

func TestNextScheduleRunAdvancesInterval(t *testing.T) {
	schedule := PeerScheduleState{Kind: "interval", RunCount: 2, MaxRuns: 5, IntervalMS: 1000, NextRunAt: 100}
	next, enabled := NextPeerScheduleRun(schedule, 100)
	if !enabled || next != 1100 {
		t.Fatalf("expected next 1100 enabled, got next=%d enabled=%v", next, enabled)
	}
}
```

- [ ] **Step 2: Run schedule tests and confirm they fail**

Run:

```bash
go test ./src/backend/internal/domain/services -run 'TestNextScheduleRun' -count=1
```

Expected: FAIL because schedule helpers do not exist.

- [ ] **Step 3: Implement schedule helper**

Create `src/backend/internal/domain/services/cluster_peer_schedule.go`:

```go
package service

type PeerScheduleState struct {
	Kind       string
	RunCount   int
	MaxRuns    int
	IntervalMS int64
	NextRunAt  int64
	ExpiresAt  int64
}

func NextPeerScheduleRun(schedule PeerScheduleState, now int64) (int64, bool) {
	if schedule.ExpiresAt > 0 && now >= schedule.ExpiresAt {
		return 0, false
	}
	nextRunCount := schedule.RunCount + 1
	if schedule.MaxRuns > 0 && nextRunCount >= schedule.MaxRuns {
		return 0, false
	}
	switch schedule.Kind {
	case "once":
		return 0, false
	case "interval":
		return now + schedule.IntervalMS, true
	default:
		return 0, false
	}
}
```

- [ ] **Step 4: Add schedule job skeleton**

Create `src/backend/internal/domain/jobs/clusterPeerScheduleJob.go`:

```go
package cronjob

import service "github.com/alireza0/s-ui/src/backend/internal/domain/services"

type ClusterPeerScheduleJob struct {
	service service.ClusterPeerScheduleService
}

func NewClusterPeerScheduleJob() *ClusterPeerScheduleJob {
	return &ClusterPeerScheduleJob{service: service.ClusterPeerScheduleService{}}
}

func (j *ClusterPeerScheduleJob) Run() {
	_ = j.service.RunDueSchedules()
}
```

Add this service type to `cluster_peer_schedule.go`:

```go
type ClusterPeerScheduleService struct{}

func (s ClusterPeerScheduleService) RunDueSchedules() error {
	return nil
}
```

This first implementation intentionally does not create schedules automatically. It provides the job boundary and a no-op service so a future panel sync plan can register schedules without changing cron wiring.

- [ ] **Step 5: Wire the job into `cronJob.go`**

In `src/backend/internal/domain/jobs/cronJob.go`, add this line after the existing cluster version polling job:

```go
// cluster peer scheduled broadcasts
c.cron.AddJob("@every 30m", NewClusterPeerScheduleJob())
```

- [ ] **Step 6: Run tests and commit**

Run:

```bash
go test ./src/backend/internal/domain/services ./src/backend/internal/domain/jobs -run 'TestNextScheduleRun' -count=1
```

Expected: PASS.

Commit:

```bash
git add src/backend/internal/domain/services/cluster_peer_schedule.go src/backend/internal/domain/services/cluster_peer_schedule_test.go src/backend/internal/domain/jobs/clusterPeerScheduleJob.go src/backend/internal/domain/jobs/cronJob.go
git commit -m "feat: add peer schedule foundation"
```

## Task 8: Full Verification and Compatibility Cleanup

**Files:**
- Modify: `src/backend/internal/domain/services/cluster_sync.go`
- Modify: `src/backend/internal/domain/services/cluster_runtime.go`
- Modify: `src/backend/internal/http/api/cluster.go`
- Test: all touched Go packages

- [ ] **Step 1: Add compatibility test for old `ClusterEnvelope`**

Add or keep a test that posts the old shape:

```json
{
  "schemaVersion": 1,
  "messageType": "sync.notify_version",
  "sourceNodeId": "node-a",
  "domain": "edge.example.com",
  "sentAt": 100,
  "version": 7,
  "signature": "..."
}
```

Expected: the route still calls `ReceiveMessage`, not `ReceivePeerMessage`.

- [ ] **Step 2: Add verification for supported actions**

Update the existing `ListDomains` tests to expect:

```go
[]string{"domain.cluster.changed", "events", "heartbeat", "ping"}
```

This preserves existing endpoint names while advertising the new action name.

- [ ] **Step 3: Run targeted Go tests**

Run:

```bash
go test ./src/backend/internal/domain/services ./src/backend/internal/http/api ./src/backend/internal/domain/jobs -count=1
```

Expected: PASS.

- [ ] **Step 4: Run full backend tests**

Run:

```bash
go test ./...
```

Expected: PASS. If unrelated packages fail because of pre-existing dirty worktree changes, capture the failing package and error in the final implementation report before proceeding.

- [ ] **Step 5: Run frontend cluster tests**

Run:

```bash
cd src/frontend
npm test -- ClusterCenter
```

Expected: PASS.

- [ ] **Step 6: Commit verification cleanup**

Commit only files changed by this task:

```bash
git add src/backend/internal/domain/services src/backend/internal/http/api src/backend/internal/domain/jobs src/backend/internal/infra/db
git commit -m "test: verify peer protocol compatibility"
```

## Self-Review Notes

Spec coverage:

- Hub does not relay peer messages: covered by preserving direct endpoint delivery in Tasks 4 and 5.
- Single versioned envelope: Task 1.
- Direct, multicast, broadcast route plan: Task 5.
- Chain route foundation: Task 6.
- Scheduled broadcast foundation: Task 7.
- Local log and idempotency state: Task 2.
- Unsupported action behavior: Task 3.
- Membership table and signature verification: Task 3 uses existing membership validation in `ClusterService.ReceivePeerMessage`.
- Existing `sync.notify_version` behavior preserved under `domain.cluster.changed`: Tasks 3, 4, 5, and 8.

Intentional follow-up:

- Panel information CRUD owner model is not implemented in this plan. It should be a separate plan after the peer protocol foundation lands, because it requires panel data ownership rules and domain service integration.

Verification:

- Each implementation task includes a focused failing test, implementation step, focused passing test, and commit.
- Final verification runs targeted Go packages and full `go test ./...`.
