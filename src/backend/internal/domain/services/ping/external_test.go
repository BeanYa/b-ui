package ping

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type rewriteHostTransport struct {
	target *url.URL
	seen   *int
}

func (t rewriteHostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	(*t.seen)++
	rewritten := req.Clone(req.Context())
	rewritten.URL.Scheme = t.target.Scheme
	rewritten.URL.Host = t.target.Host
	return http.DefaultTransport.RoundTrip(rewritten)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

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

func TestProbeEndpointWithMethodsHTTPUsesExternalEndpointRoot(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"processed","code":"ok"}`)
	}))
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse server URL: %v", err)
	}
	var requests int
	svc := NewExternalService(newStoreWithDir(t.TempDir()))
	client := &http.Client{Transport: rewriteHostTransport{target: serverURL, seen: &requests}}
	svc.httpClient = client
	svc.meshSvc.httpClient = client

	method, latency, err := svc.probeEndpointWithMethods(context.Background(), ExternalEndpoint{
		ID:      "external-http",
		Host:    "example.test",
		Port:    80,
		Methods: []string{MethodHTTP},
	}, []string{MethodHTTP})
	if err != nil {
		t.Fatalf("probeEndpointWithMethods: %v", err)
	}
	if method != MethodHTTP {
		t.Fatalf("expected HTTP method, got %q", method)
	}
	if latency < 0 {
		t.Fatalf("expected non-negative latency, got %f", latency)
	}
	if requests == 0 {
		t.Fatal("expected HTTP request")
	}
	if gotPath != "/" {
		t.Fatalf("expected external HTTP probe to request root path, got %q", gotPath)
	}
}

func TestProbeEndpointWithMethodsHTTPFallsBackToGETAfterHead405(t *testing.T) {
	methodsSeen := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodsSeen = append(methodsSeen, r.Method)
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse server URL: %v", err)
	}
	var requests int
	svc := NewExternalService(newStoreWithDir(t.TempDir()))
	svc.httpClient = &http.Client{Transport: rewriteHostTransport{target: serverURL, seen: &requests}}

	method, latency, err := svc.probeEndpointWithMethods(context.Background(), ExternalEndpoint{
		ID:      "external-http",
		Host:    "example.test",
		Port:    80,
		Methods: []string{MethodHTTP},
	}, []string{MethodHTTP})
	if err != nil {
		t.Fatalf("probeEndpointWithMethods: %v", err)
	}
	if method != MethodHTTP {
		t.Fatalf("expected HTTP method, got %q", method)
	}
	if latency < 0 {
		t.Fatalf("expected non-negative latency, got %f", latency)
	}
	if strings.Join(methodsSeen, ",") != "HEAD,GET" {
		t.Fatalf("expected HEAD then GET, got %v", methodsSeen)
	}
}

func TestProbeEndpointWithMethodsHTTPFailsWhenGetFallbackRejected(t *testing.T) {
	methodsSeen := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodsSeen = append(methodsSeen, r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Parse server URL: %v", err)
	}
	var requests int
	svc := NewExternalService(newStoreWithDir(t.TempDir()))
	svc.httpClient = &http.Client{Transport: rewriteHostTransport{target: serverURL, seen: &requests}}

	_, _, err = svc.probeEndpointWithMethods(context.Background(), ExternalEndpoint{
		ID:      "external-http",
		Host:    "example.test",
		Port:    80,
		Methods: []string{MethodHTTP},
	}, []string{MethodHTTP})
	if err == nil {
		t.Fatal("expected GET fallback 405 to fail")
	}
	if strings.Join(methodsSeen, ",") != "HEAD,GET" {
		t.Fatalf("expected HEAD then GET, got %v", methodsSeen)
	}
}

func TestProbeEndpointWithMethodsUnsupportedRequestedMethodDoesNotProbe(t *testing.T) {
	var requests int
	svc := NewExternalService(newStoreWithDir(t.TempDir()))
	svc.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		return nil, fmt.Errorf("unexpected HTTP request")
	})}

	_, _, err := svc.probeEndpointWithMethods(context.Background(), ExternalEndpoint{
		ID:      "tcp-only",
		Host:    "example.test",
		Port:    80,
		Methods: []string{MethodTCP},
	}, []string{MethodHTTP})
	if err == nil {
		t.Fatal("expected unsupported method error")
	}
	if !strings.Contains(err.Error(), "no supported methods") {
		t.Fatalf("expected no supported methods error, got %v", err)
	}
	if requests != 0 {
		t.Fatalf("expected no HTTP requests, got %d", requests)
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

func TestRunInboundUsesExplicitTargetInStandaloneMode(t *testing.T) {
	svc := NewExternalService(newStoreWithDir(t.TempDir()))
	svc.runInboundProvider = func(ctx context.Context, sourceID string, target ExternalEndpoint) []ExternalTestResult {
		return []ExternalTestResult{{
			Direction: DirectionInbound,
			Source:    ExternalEndpoint{ID: sourceID},
			Target:    target,
			Success:   true,
		}}
	}

	data, err := svc.Run(context.Background(), ExternalRunRequest{
		Direction: DirectionInbound,
		SourceIDs: []string{"check_host"},
		Target: &ExternalTargetRequest{
			Host:  "panel.example.com",
			Port:  443,
			Label: "Panel",
		},
	}, nil)
	if err != nil {
		t.Fatalf("Run inbound: %v", err)
	}
	if len(data.Results) != 1 {
		t.Fatalf("expected one inbound result, got %d", len(data.Results))
	}
	result := data.Results[0]
	if result.Target.Host != "panel.example.com" {
		t.Fatalf("expected explicit target host, got %q", result.Target.Host)
	}
	if result.Target.Port != 443 {
		t.Fatalf("expected explicit target port, got %d", result.Target.Port)
	}
}

func TestRunInboundRequiresExplicitTarget(t *testing.T) {
	svc := NewExternalService(newStoreWithDir(t.TempDir()))

	_, err := svc.Run(context.Background(), ExternalRunRequest{
		Direction: DirectionInbound,
		SourceIDs: []string{"check_host"},
	}, nil)

	if err == nil {
		t.Fatal("expected missing inbound target to fail")
	}
	if !strings.Contains(err.Error(), "target") {
		t.Fatalf("expected target error, got %v", err)
	}
}

func TestTargetFromRequestDefaultsLabelAndPreservesPort(t *testing.T) {
	target, err := targetFromRequest(&ExternalTargetRequest{
		Host: " panel.example.com ",
		Port: 0,
	})
	if err != nil {
		t.Fatalf("targetFromRequest: %v", err)
	}
	if target.ID != "current-target" {
		t.Fatalf("expected current-target id, got %q", target.ID)
	}
	if target.Provider != "panel" {
		t.Fatalf("expected panel provider, got %q", target.Provider)
	}
	if target.Host != "panel.example.com" {
		t.Fatalf("expected trimmed host, got %q", target.Host)
	}
	if target.Label != "panel.example.com" {
		t.Fatalf("expected label to default to host, got %q", target.Label)
	}
	if target.Port != 0 {
		t.Fatalf("expected port to be preserved exactly, got %d", target.Port)
	}
}

func TestRunCheckHostTCPParsesNodeMetadataAndLatency(t *testing.T) {
	var startSeen int
	var resultSeen int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/json" {
			t.Fatalf("expected JSON accept header, got %q", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/check-tcp":
			startSeen++
			if r.URL.Query().Get("host") != "panel.example.com:443" {
				t.Fatalf("expected explicit target address in query, got %q", r.URL.Query().Get("host"))
			}
			if r.URL.Query().Get("max_nodes") != "12" {
				t.Fatalf("expected max_nodes=12, got %q", r.URL.Query().Get("max_nodes"))
			}
			fmt.Fprint(w, `{"ok":1,"request_id":"req-1","nodes":{"us1.node.check-host.net":["us","USA","Los Angeles","5.253.30.82","AS18978"],"de1.node.check-host.net":["de","Germany","Frankfurt","46.4.143.48","AS24940"]}}`)
		case "/check-result/req-1":
			resultSeen++
			fmt.Fprint(w, `{"us1.node.check-host.net":[{"time":0.031,"address":"203.0.113.10"}],"de1.node.check-host.net":[[1,"0.045","OK"]]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc := NewExternalService(newStoreWithDir(t.TempDir()))
	svc.checkHostBaseURL = server.URL
	svc.checkHostPollDelay = 0
	svc.checkHostPollAttempts = 1

	target := ExternalEndpoint{ID: "current-target", Label: "Panel", Provider: "panel", Host: "panel.example.com", Port: 443}
	results := svc.runCheckHostTCP(context.Background(), target)

	if startSeen != 1 {
		t.Fatalf("expected one Check-Host start request, got %d", startSeen)
	}
	if resultSeen != 1 {
		t.Fatalf("expected one Check-Host result request, got %d", resultSeen)
	}
	if len(results) != 2 {
		t.Fatalf("expected two Check-Host results, got %d", len(results))
	}

	byNode := make(map[string]ExternalTestResult, len(results))
	for _, result := range results {
		byNode[result.Source.ID] = result
		if !result.Success {
			t.Fatalf("expected successful result for %s: %#v", result.Source.ID, result)
		}
		if result.Direction != DirectionInbound {
			t.Fatalf("expected inbound direction, got %q", result.Direction)
		}
		if result.Target.ID != target.ID || result.Target.Host != target.Host || result.Target.Port != target.Port || result.Target.Label != target.Label {
			t.Fatalf("expected explicit target, got %#v", result.Target)
		}
		if result.Method == nil || *result.Method != MethodTCP {
			t.Fatalf("expected tcp method, got %#v", result.Method)
		}
	}

	us := byNode["us1.node.check-host.net"]
	if us.Source.Provider != "check_host" {
		t.Fatalf("expected check_host source provider, got %q", us.Source.Provider)
	}
	if us.Source.Country != "USA" || us.Source.City != "Los Angeles" || us.Source.Network != "AS18978" {
		t.Fatalf("expected node metadata from Check-Host nodes map, got %#v", us.Source)
	}
	if us.LatencyMs == nil || math.Abs(*us.LatencyMs-31) > 0.001 {
		t.Fatalf("expected 31ms latency, got %#v", us.LatencyMs)
	}

	de := byNode["de1.node.check-host.net"]
	if de.LatencyMs == nil || math.Abs(*de.LatencyMs-45) > 0.001 {
		t.Fatalf("expected nested string latency converted to 45ms, got %#v", de.LatencyMs)
	}
}

func TestRunCheckHostTCPTreatsFailureTupleAsFailedResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/check-tcp":
			fmt.Fprint(w, `{"ok":1,"request_id":"req-fail","nodes":{"hk1.node.check-host.net":["hk","Hong Kong","Hong Kong","203.0.113.45","AS64500"]}}`)
		case "/check-result/req-fail":
			fmt.Fprint(w, `{"hk1.node.check-host.net":[[0,"Connection refused"]]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc := NewExternalService(newStoreWithDir(t.TempDir()))
	svc.checkHostBaseURL = server.URL
	svc.checkHostPollDelay = 0
	svc.checkHostPollAttempts = 1

	results := svc.runCheckHostTCP(context.Background(), ExternalEndpoint{
		ID:    "current-target",
		Label: "Panel",
		Host:  "panel.example.com",
		Port:  443,
	})
	if len(results) != 1 {
		t.Fatalf("expected one failed node result, got %d", len(results))
	}
	result := results[0]
	if result.Success {
		t.Fatalf("expected failure tuple to produce failed result, got %#v", result)
	}
	if result.Error == nil || !strings.Contains(*result.Error, "Connection refused") {
		t.Fatalf("expected connection error message, got %#v", result.Error)
	}
	if result.Method != nil || result.LatencyMs != nil {
		t.Fatalf("expected no success metadata, got method=%#v latency=%#v", result.Method, result.LatencyMs)
	}
}

func TestRunInboundProviderDefaultUnknownProviderReturnsFailedResult(t *testing.T) {
	svc := NewExternalService(newStoreWithDir(t.TempDir()))
	target := ExternalEndpoint{ID: "current-target", Label: "Panel", Host: "panel.example.com", Port: 443}

	results := svc.runInboundProviderDefault(context.Background(), "globalping", target)

	if len(results) != 1 {
		t.Fatalf("expected one failed provider result, got %d", len(results))
	}
	result := results[0]
	if result.Success {
		t.Fatalf("expected unknown provider to fail, got %#v", result)
	}
	if result.Direction != DirectionInbound {
		t.Fatalf("expected inbound direction, got %q", result.Direction)
	}
	if result.Target.Host != target.Host || result.Target.Port != target.Port {
		t.Fatalf("expected explicit target, got %#v", result.Target)
	}
	if result.Error == nil || !strings.Contains(*result.Error, "not implemented") {
		t.Fatalf("expected clear not implemented error, got %#v", result.Error)
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
