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
		{ID: "zstatic_cdn", Name: "Zstatic CDN", Type: "cdn_ping", Direction: "inbound", Enabled: false, APIKey: "legacy-key", WorkerURL: "https://worker.example"},
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
	if zstatic.Enabled {
		t.Fatal("expected normalization to preserve disabled enabled flag")
	}
	if zstatic.APIKey != "legacy-key" {
		t.Fatalf("expected normalization to preserve api key, got %q", zstatic.APIKey)
	}
	if zstatic.WorkerURL != "https://worker.example" {
		t.Fatalf("expected normalization to preserve worker url, got %q", zstatic.WorkerURL)
	}
}

func TestRunInboundUnknownEnabledProviderReturnsErrorResult(t *testing.T) {
	svc := NewExternalService(newStoreWithDir(t.TempDir()))

	data, err := svc.RunInbound(context.Background(), []string{"check_host"}, []MeshMember{
		{MemberID: "node-a", NodeID: "node-a", Name: "Node A", Address: "node-a.example"},
	})
	if err != nil {
		t.Fatalf("RunInbound: %v", err)
	}
	if len(data.Results) != 1 {
		t.Fatalf("expected one error result for unimplemented inbound provider, got %d", len(data.Results))
	}
	result := data.Results[0]
	if result.Success {
		t.Fatal("expected unimplemented inbound provider result to fail")
	}
	if result.Direction != "inbound" {
		t.Fatalf("expected inbound direction, got %q", result.Direction)
	}
	if result.Source.ID != "check_host" || result.SourceLabel != "Check-Host" {
		t.Fatalf("expected source populated from provider config, got source=%#v source_label=%q", result.Source, result.SourceLabel)
	}
	if result.Error == nil || *result.Error == "" {
		t.Fatal("expected unimplemented inbound provider error message")
	}
}

func TestRunOutboundUnknownEnabledProviderReturnsErrorResult(t *testing.T) {
	svc := NewExternalService(newStoreWithDir(t.TempDir()))

	data, err := svc.RunOutbound(context.Background(), []string{"zstatic_cdn"}, []MeshMember{
		{MemberID: "node-a", NodeID: "node-a", Name: "Node A", Address: "node-a.example"},
	})
	if err != nil {
		t.Fatalf("RunOutbound: %v", err)
	}
	if len(data.Results) != 1 {
		t.Fatalf("expected one error result for unimplemented outbound provider, got %d", len(data.Results))
	}
	result := data.Results[0]
	if result.Success {
		t.Fatal("expected unimplemented outbound provider result to fail")
	}
	if result.Direction != "outbound" {
		t.Fatalf("expected outbound direction, got %q", result.Direction)
	}
	if result.Source.ID != "zstatic_cdn" || result.SourceLabel != "ZStaticCDN" {
		t.Fatalf("expected source populated from provider config, got source=%#v source_label=%q", result.Source, result.SourceLabel)
	}
	if result.Error == nil || *result.Error == "" {
		t.Fatal("expected unimplemented outbound provider error message")
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
		if result.Source.ID == "" {
			t.Fatalf("expected legacy outbound source endpoint id to be populated, got %#v", result)
		}
		if result.Target.ID == "" {
			t.Fatalf("expected legacy outbound target endpoint id to be populated, got %#v", result)
		}
	}
}
