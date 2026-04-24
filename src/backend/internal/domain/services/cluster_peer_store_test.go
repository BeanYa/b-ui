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
