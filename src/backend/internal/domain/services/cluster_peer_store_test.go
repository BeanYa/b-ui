package service

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	database "github.com/alireza0/b-ui/src/backend/internal/infra/db"
	"github.com/alireza0/b-ui/src/backend/internal/infra/db/model"
)

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

func TestMemoryPeerStoreUsesNonEmptyIdempotencyKey(t *testing.T) {
	store := newMemoryPeerStore()
	first := &PeerMessage{MessageID: "msg-1", DomainID: "edge.example.com", IdempotencyKey: "idem-1", PayloadHash: "hash-a", Action: "domain.cluster.changed"}
	second := &PeerMessage{MessageID: "msg-2", DomainID: "edge.example.com", IdempotencyKey: "idem-1", PayloadHash: "hash-a", Action: "domain.cluster.changed"}

	if _, err := store.RecordReceived(first); err != nil {
		t.Fatalf("record first: %v", err)
	}
	state, err := store.RecordReceived(second)
	if err != nil {
		t.Fatalf("record duplicate: %v", err)
	}
	if state.MessageID != "msg-1" {
		t.Fatalf("expected duplicate idempotency key to return first message state, got %q", state.MessageID)
	}
}

func TestMemoryPeerStoreUsesSourceNodeAndSequence(t *testing.T) {
	store := newMemoryPeerStore()
	first := &PeerMessage{MessageID: "msg-1", DomainID: "edge.example.com", SourceNodeID: "node-a", SourceSeq: 7, PayloadHash: "hash-a", Action: "domain.cluster.changed"}
	second := &PeerMessage{MessageID: "msg-2", DomainID: "edge.example.com", SourceNodeID: "node-a", SourceSeq: 7, PayloadHash: "hash-a", Action: "domain.cluster.changed"}

	if _, err := store.RecordReceived(first); err != nil {
		t.Fatalf("record first: %v", err)
	}
	state, err := store.RecordReceived(second)
	if err != nil {
		t.Fatalf("record duplicate: %v", err)
	}
	if state.MessageID != "msg-1" {
		t.Fatalf("expected duplicate source sequence to return first message state, got %q", state.MessageID)
	}
}

func TestMemoryPeerStoreClaimProcessingIsAtomic(t *testing.T) {
	store := newMemoryPeerStore()
	message := &PeerMessage{MessageID: "msg-1", DomainID: "edge.example.com", PayloadHash: "hash-a", Action: "domain.cluster.changed"}
	if _, err := store.RecordReceived(message); err != nil {
		t.Fatalf("record: %v", err)
	}
	claimed, err := store.ClaimProcessing(message.MessageID)
	if err != nil {
		t.Fatalf("claim first: %v", err)
	}
	if !claimed {
		t.Fatal("expected first claim to transition to processing")
	}
	claimed, err = store.ClaimProcessing(message.MessageID)
	if err != nil {
		t.Fatalf("claim second: %v", err)
	}
	if claimed {
		t.Fatal("expected second claim not to transition while processing")
	}
}

func TestDBClusterPeerStoreUsesIdempotencySourceSequenceAndAtomicClaim(t *testing.T) {
	if err := database.InitDB(filepath.Join(t.TempDir(), "peer-store.db")); err != nil {
		if strings.Contains(err.Error(), "go-sqlite3 requires cgo") || strings.Contains(err.Error(), "C compiler") {
			t.Skipf("sqlite test database unavailable in this toolchain: %v", err)
		}
		t.Fatalf("init test db: %v", err)
	}
	store := &dbClusterPeerStore{}
	first := &PeerMessage{MessageID: "msg-1", DomainID: "edge.example.com", IdempotencyKey: "idem-1", SourceNodeID: "node-a", SourceSeq: 7, PayloadHash: "hash-a", Action: "domain.cluster.changed"}
	second := &PeerMessage{MessageID: "msg-2", DomainID: "edge.example.com", IdempotencyKey: "idem-1", PayloadHash: "hash-a", Action: "domain.cluster.changed"}
	third := &PeerMessage{MessageID: "msg-3", DomainID: "edge.example.com", SourceNodeID: "node-a", SourceSeq: 7, PayloadHash: "hash-a", Action: "domain.cluster.changed"}

	if _, err := store.RecordReceived(first); err != nil {
		t.Fatalf("record first: %v", err)
	}
	state, err := store.RecordReceived(second)
	if err != nil {
		t.Fatalf("record idempotency duplicate: %v", err)
	}
	if state.MessageID != first.MessageID {
		t.Fatalf("expected idempotency duplicate to return first state, got %q", state.MessageID)
	}
	state, err = store.RecordReceived(third)
	if err != nil {
		t.Fatalf("record source sequence duplicate: %v", err)
	}
	if state.MessageID != first.MessageID {
		t.Fatalf("expected source sequence duplicate to return first state, got %q", state.MessageID)
	}
	claimed, err := store.ClaimProcessing(first.MessageID)
	if err != nil {
		t.Fatalf("claim first: %v", err)
	}
	if !claimed {
		t.Fatal("expected first claim to transition")
	}
	claimed, err = store.ClaimProcessing(first.MessageID)
	if err != nil {
		t.Fatalf("claim second: %v", err)
	}
	if claimed {
		t.Fatal("expected second claim not to transition")
	}
}

func TestDBClusterPeerStoreReloadsExistingStateAfterCreateConflict(t *testing.T) {
	message := &PeerMessage{
		MessageID:    "msg-1",
		DomainID:     "edge.example.com",
		SourceNodeID: "node-a",
		PayloadHash:  "hash-a",
		Action:       "domain.cluster.changed",
		CreatedAt:    1700000000,
	}
	existing := &model.ClusterPeerEventState{
		MessageID:   message.MessageID,
		PayloadHash: message.PayloadHash,
		Status:      PeerEventStatusSucceeded,
	}

	state, handled, err := resolvePeerEventStateCreateConflict(
		errors.New("UNIQUE constraint failed: cluster_peer_event_states.message_id"),
		message,
		func(lookupMessage *PeerMessage) (*model.ClusterPeerEventState, error) {
			if lookupMessage.MessageID != message.MessageID {
				t.Fatalf("expected lookup for %q, got %q", message.MessageID, lookupMessage.MessageID)
			}
			return existing, nil
		},
	)
	if err != nil {
		t.Fatalf("resolve create conflict: %v", err)
	}
	if !handled {
		t.Fatal("expected unique conflict to be handled")
	}
	if state.Status != PeerEventStatusSucceeded {
		t.Fatalf("expected existing succeeded state, got %q", state.Status)
	}
	if state.PayloadHash != message.PayloadHash {
		t.Fatalf("expected payload hash %q, got %q", message.PayloadHash, state.PayloadHash)
	}
}

func TestDBClusterPeerStoreCreateConflictRejectsDifferentPayload(t *testing.T) {
	message := &PeerMessage{MessageID: "msg-1", DomainID: "edge.example.com", PayloadHash: "hash-a", Action: "domain.cluster.changed"}
	existing := &model.ClusterPeerEventState{
		MessageID:   message.MessageID,
		PayloadHash: "hash-b",
		Status:      PeerEventStatusReceived,
	}

	_, handled, err := resolvePeerEventStateCreateConflict(
		errors.New("UNIQUE constraint failed: cluster_peer_event_states.message_id"),
		message,
		func(*PeerMessage) (*model.ClusterPeerEventState, error) {
			return existing, nil
		},
	)
	if !handled {
		t.Fatal("expected unique conflict to be handled")
	}
	if err == nil || err.Error() != "payload_hash_mismatch" {
		t.Fatalf("expected payload_hash_mismatch, got %v", err)
	}
}
