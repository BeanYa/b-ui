package ping

import (
	"context"
	"testing"
)

func TestDefaultExternalConfig(t *testing.T) {
	config := defaultExternalConfig()
	if len(config.Sources) != 9 {
		t.Fatalf("expected 9 default sources, got %d", len(config.Sources))
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
	if inCount != 4 {
		t.Fatalf("expected 4 inbound sources, got %d", inCount)
	}
	if outCount != 5 {
		t.Fatalf("expected 5 outbound sources, got %d", outCount)
	}
}

func TestDefaultExternalConfigUsesCurrentNodeProviderModel(t *testing.T) {
	config := defaultExternalConfig()

	byID := map[string]ExternalSource{}
	for _, src := range config.Sources {
		byID[src.ID] = src
	}

	if byID["check_host"].Direction != "inbound" {
		t.Fatalf("expected check_host inbound, got %#v", byID["check_host"])
	}
	if byID["zstatic_cdn"].Direction != "outbound" {
		t.Fatalf("expected zstatic_cdn outbound target provider, got %#v", byID["zstatic_cdn"])
	}
	if byID["linode_speedtest"].Direction != "outbound" {
		t.Fatalf("expected linode_speedtest outbound, got %#v", byID["linode_speedtest"])
	}
}

func TestNormalizeExternalConfigCorrectsLegacyZStaticDirection(t *testing.T) {
	config := normalizeExternalConfig(&ExternalConfig{Sources: []ExternalSource{
		{ID: "zstatic_cdn", Name: "Zstatic CDN", Type: "cdn_ping", Direction: "inbound", Enabled: true},
	}})

	var zstatic ExternalSource
	for _, src := range config.Sources {
		if src.ID == "zstatic_cdn" {
			zstatic = src
			break
		}
	}

	if zstatic.ID == "" {
		t.Fatal("expected zstatic_cdn source after normalization")
	}
	if zstatic.Direction != "outbound" {
		t.Fatalf("expected normalized zstatic_cdn direction outbound, got %q", zstatic.Direction)
	}
	if !zstatic.Enabled {
		t.Fatal("expected normalization to preserve enabled flag")
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
