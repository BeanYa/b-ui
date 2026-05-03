package ping

import (
	"context"
	"testing"
)

func TestDefaultExternalConfig(t *testing.T) {
	config := defaultExternalConfig()
	if len(config.Sources) != 10 {
		t.Fatalf("expected 10 default sources, got %d", len(config.Sources))
	}
	inCount := 0
	outCount := 0
	for _, src := range config.Sources {
		if src.Direction == "inbound" {
			inCount++
		} else if src.Direction == "outbound" {
			outCount++
		}
	}
	if inCount != 5 {
		t.Fatalf("expected 5 inbound sources, got %d", inCount)
	}
	if outCount != 5 {
		t.Fatalf("expected 5 outbound sources, got %d", outCount)
	}
}

func TestRunExternal_NoSourcesEnabled(t *testing.T) {
	svc := NewExternalService(newStoreWithDir(t.TempDir()))
	_, err := svc.Run(context.Background(), ExternalRunRequest{SourceIDs: []string{}}, nil)
	if err == nil {
		t.Fatal("expected error for no enabled sources")
	}
}

func TestRunExternal_TargetNodeIDsFiltersMembers(t *testing.T) {
	svc := NewExternalService(newStoreWithDir(t.TempDir()))
	svc.meshSvc.icmpPinger = func(context.Context, string) (float64, error) {
		return 12.5, nil
	}

	data, err := svc.Run(context.Background(), ExternalRunRequest{
		SourceIDs:     []string{"public_dns"},
		TargetNodeIDs: []string{"node-a"},
	}, []MeshMember{
		{MemberID: "node-a", NodeID: "node-a", Name: "Node A", Address: "node-a.example"},
		{MemberID: "node-b", NodeID: "node-b", Name: "Node B", Address: "node-b.example"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(data.Results) != len(outboundTargets["public_dns"]) {
		t.Fatalf("expected only public DNS results for node-a, got %d", len(data.Results))
	}
	for _, result := range data.Results {
		if result.SourceMemberID != "node-a" {
			t.Fatalf("expected target filter to exclude node-b, got result from %q", result.SourceMemberID)
		}
	}
}
