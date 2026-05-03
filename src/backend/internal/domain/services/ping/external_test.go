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

func TestZStaticTargetsUsePublishedPortsOnly(t *testing.T) {
	targets := zstaticTargets()
	if len(targets) == 0 {
		t.Fatal("expected ZStatic target catalogue")
	}
	for _, target := range targets {
		if target.Provider != "zstatic_cdn" {
			t.Fatalf("expected provider zstatic_cdn, got %q", target.Provider)
		}
		if target.Host == "" {
			t.Fatalf("expected host for %#v", target)
		}
		if target.Port <= 0 {
			t.Fatalf("expected published port for %#v", target)
		}
		if !methodAllowed(MethodTCP, target.Methods) {
			t.Fatalf("expected TCP method for %#v", target)
		}
	}
}

func TestRunOutboundUsesCurrentNodeOnceWithoutMembers(t *testing.T) {
	svc := NewExternalService(newStoreWithDir(t.TempDir()))
	svc.probeEndpoint = func(ctx context.Context, endpoint ExternalEndpoint, methods []string) (string, float64, error) {
		return MethodTCP, 14.2, nil
	}

	data, err := svc.Run(context.Background(), ExternalRunRequest{
		Direction: DirectionOutbound,
		SourceIDs: []string{"public_dns"},
		Methods:   []string{MethodTCP},
	}, nil)
	if err != nil {
		t.Fatalf("Run outbound: %v", err)
	}
	if len(data.Results) == 0 {
		t.Fatal("expected outbound results")
	}
	for _, result := range data.Results {
		if result.Direction != DirectionOutbound {
			t.Fatalf("expected outbound direction, got %q", result.Direction)
		}
		if result.Source.ID != "current-panel" {
			t.Fatalf("expected current-panel source, got %#v", result.Source)
		}
		if result.SourceMemberID != "current-panel" {
			t.Fatalf("expected legacy current-panel source id, got %q", result.SourceMemberID)
		}
	}
}

func TestRunOutboundIgnoresDeprecatedTargetNodeIDs(t *testing.T) {
	svc := NewExternalService(newStoreWithDir(t.TempDir()))
	var probed int
	svc.probeEndpoint = func(ctx context.Context, endpoint ExternalEndpoint, methods []string) (string, float64, error) {
		probed++
		return MethodTCP, 9, nil
	}

	data, err := svc.Run(context.Background(), ExternalRunRequest{
		SourceIDs:     []string{"public_dns"},
		TargetNodeIDs: []string{"node-a", "node-b"},
		Direction:     DirectionOutbound,
		Methods:       []string{MethodTCP},
	}, []MeshMember{
		{MemberID: "node-a", NodeID: "node-a", Name: "Node A"},
		{MemberID: "node-b", NodeID: "node-b", Name: "Node B"},
	})
	if err != nil {
		t.Fatalf("Run outbound: %v", err)
	}

	if len(data.Results) != len(publicDNSTargets()) {
		t.Fatalf("expected one result per public DNS target, got %d", len(data.Results))
	}
	if probed != len(publicDNSTargets()) {
		t.Fatalf("expected %d probes, got %d", len(publicDNSTargets()), probed)
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

func TestNormalizeExternalConfigPreservesLegacyRunnableSources(t *testing.T) {
	config := normalizeExternalConfig(&ExternalConfig{Sources: []ExternalSource{
		{ID: "he_lg", Name: "Hurricane Electric LG", Type: "web_scrape", Direction: DirectionInbound, Enabled: true, APIKey: "he-key"},
	}})

	var he ExternalSource
	for _, src := range config.Sources {
		if src.ID == "he_lg" {
			he = src
			break
		}
	}

	if he.ID == "" {
		t.Fatal("expected legacy he_lg source after normalization")
	}
	if he.Direction != DirectionInbound {
		t.Fatalf("expected he_lg direction inbound, got %q", he.Direction)
	}
	if !he.Enabled {
		t.Fatal("expected normalization to preserve he_lg enabled flag")
	}
	if he.APIKey != "he-key" {
		t.Fatalf("expected normalization to preserve he_lg api key, got %q", he.APIKey)
	}
}

func TestEnabledExternalSourcesDeduplicatesRequestIDs(t *testing.T) {
	sources := enabledExternalSources([]string{"public_dns", "public_dns"}, defaultExternalConfig(), DirectionOutbound)

	if len(sources) != 1 {
		t.Fatalf("expected duplicate request IDs to resolve once, got %d", len(sources))
	}
	if sources[0].ID != "public_dns" {
		t.Fatalf("expected public_dns source, got %q", sources[0].ID)
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
	store := newStoreWithDir(t.TempDir())
	if err := store.SaveExternalConfig(&ExternalConfig{Sources: []ExternalSource{
		{ID: "unknown_out", Name: "Unknown Out", Type: "target_catalog", Direction: DirectionOutbound, Enabled: true},
	}}); err != nil {
		t.Fatalf("SaveExternalConfig: %v", err)
	}
	svc := NewExternalService(store)

	data, err := svc.RunOutbound(context.Background(), []string{"unknown_out"}, []MeshMember{
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
	if result.Source.ID != "unknown_out" || result.SourceLabel != "Unknown Out" {
		t.Fatalf("expected source populated from provider config, got source=%#v source_label=%q", result.Source, result.SourceLabel)
	}
	if result.Error == nil || *result.Error == "" {
		t.Fatal("expected unimplemented outbound provider error message")
	}
}

func TestRunInboundLegacyProviderPopulatesEndpointMetadata(t *testing.T) {
	store := newStoreWithDir(t.TempDir())
	if err := store.SaveExternalConfig(&ExternalConfig{Sources: []ExternalSource{
		{ID: "linode_lg", Name: "Linode Looking Glass", Type: "web_scrape", Direction: DirectionInbound, Enabled: true},
	}}); err != nil {
		t.Fatalf("SaveExternalConfig: %v", err)
	}
	svc := NewExternalService(store)
	svc.meshSvc.icmpPinger = func(context.Context, string) (float64, error) {
		return 12.5, nil
	}

	data, err := svc.RunInbound(context.Background(), []string{"linode_lg"}, []MeshMember{
		{MemberID: "node-a", NodeID: "node-a", Name: "Node A", Address: "node-a.example"},
	})
	if err != nil {
		t.Fatalf("RunInbound: %v", err)
	}
	if len(data.Results) == 0 {
		t.Fatal("expected legacy inbound results")
	}
	result := data.Results[0]
	if result.Source.ID == "" {
		t.Fatalf("expected source endpoint id, got %#v", result.Source)
	}
	if result.Source.Provider != "linode_lg" {
		t.Fatalf("expected source provider linode_lg, got %q", result.Source.Provider)
	}
	if result.Target.ID != "node-a" {
		t.Fatalf("expected target endpoint node-a, got %q", result.Target.ID)
	}
	if result.SourceLabel != result.Source.Label {
		t.Fatalf("expected legacy source label to align with endpoint label, got %q and %q", result.SourceLabel, result.Source.Label)
	}
	if result.TargetMemberID != result.Target.ID {
		t.Fatalf("expected legacy target member id to align with endpoint id, got %q and %q", result.TargetMemberID, result.Target.ID)
	}
}

func TestRunRIPEAtlasPopulatesEndpointMetadata(t *testing.T) {
	svc := NewExternalService(newStoreWithDir(t.TempDir()))

	data, err := svc.RunRIPEAtlas(context.Background(), "api-key", []MeshMember{
		{MemberID: "node-a", NodeID: "node-a", Name: "Node A", Address: "node-a.example"},
	})
	if err != nil {
		t.Fatalf("RunRIPEAtlas: %v", err)
	}
	if len(data.Results) != 1 {
		t.Fatalf("expected one RIPE Atlas result, got %d", len(data.Results))
	}
	result := data.Results[0]
	if result.Source.ID != "ripe_atlas" {
		t.Fatalf("expected source endpoint ripe_atlas, got %q", result.Source.ID)
	}
	if result.Source.Label != "RIPE Atlas" {
		t.Fatalf("expected source label RIPE Atlas, got %q", result.Source.Label)
	}
	if result.Target.ID != "node-a" {
		t.Fatalf("expected target endpoint node-a, got %q", result.Target.ID)
	}
	if result.TargetMemberID != result.Target.ID {
		t.Fatalf("expected legacy target member id to align with endpoint id, got %q and %q", result.TargetMemberID, result.Target.ID)
	}
}

func TestRunExternal_NoSourcesEnabled(t *testing.T) {
	svc := NewExternalService(newStoreWithDir(t.TempDir()))
	_, err := svc.Run(context.Background(), ExternalRunRequest{SourceIDs: []string{}}, nil)
	if err == nil {
		t.Fatal("expected error for no enabled sources")
	}
}

func TestRunExternalOutboundIgnoresTargetNodeIDs(t *testing.T) {
	svc := NewExternalService(newStoreWithDir(t.TempDir()))
	svc.probeEndpoint = func(ctx context.Context, endpoint ExternalEndpoint, methods []string) (string, float64, error) {
		return MethodTCP, 12.5, nil
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
	if len(data.Results) != len(publicDNSTargets()) {
		t.Fatalf("expected one result per public DNS target, got %d", len(data.Results))
	}
	for _, result := range data.Results {
		if result.SourceMemberID != "current-panel" {
			t.Fatalf("expected outbound source to be current panel, got %q", result.SourceMemberID)
		}
		if result.Source.ID == "" {
			t.Fatalf("expected outbound source endpoint id to be populated, got %#v", result)
		}
		if result.Target.ID == "" {
			t.Fatalf("expected outbound target endpoint id to be populated, got %#v", result)
		}
	}
}
