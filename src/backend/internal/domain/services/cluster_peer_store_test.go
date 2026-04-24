package service

import (
	"errors"
	"testing"

	"github.com/alireza0/s-ui/src/backend/internal/infra/db/model"
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
		func(messageID string) (*model.ClusterPeerEventState, error) {
			if messageID != message.MessageID {
				t.Fatalf("expected lookup for %q, got %q", message.MessageID, messageID)
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
		func(string) (*model.ClusterPeerEventState, error) {
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
