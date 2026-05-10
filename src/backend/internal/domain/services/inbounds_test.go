package service

import (
	"encoding/json"
	"testing"
)

func TestInboundServiceGetByIdReturnsEmptySliceWhenMissing(t *testing.T) {
	initClusterInboundTestDB(t)
	svc := &InboundService{}

	got, err := svc.Get("404")
	if err != nil {
		t.Fatalf("get missing inbound: %v", err)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal missing inbound result: %v", err)
	}
	if string(encoded) != "[]" {
		t.Fatalf("expected empty JSON array for missing inbound, got %s", encoded)
	}
}
