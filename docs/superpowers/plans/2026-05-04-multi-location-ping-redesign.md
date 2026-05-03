# Multi-Location Ping Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild Multi-Location Ping so inbound tests use public probes against the current server, outbound tests use the current server against public multi-location targets, and standalone mode works without any cluster domain.

**Architecture:** Add explicit endpoint metadata and provider abstractions to `internal/domain/services/ping`. Keep the existing `/api/ping/external` route but change it to resolve a current-node target for inbound and avoid cluster-member fan-out for outbound. Update the frontend store and page to show actual probe/target nodes instead of aggregate labels.

**Tech Stack:** Go, Gin, Pinia, Vue 3, Vitest, existing shell-based ping/TCP/HTTP latency helpers.

---

## File Structure

- Modify `src/backend/internal/domain/services/ping/types.go`
  - Add endpoint metadata and extend external request/result types.
- Create `src/backend/internal/domain/services/ping/providers.go`
  - Define provider interfaces, method helpers, selected-method logic, endpoint host formatting, and config normalization helpers.
- Create `src/backend/internal/domain/services/ping/target_providers.go`
  - Define outbound target catalogues for ZStaticCDN, Linode/Akamai, DNS, CDN, and cloud endpoints.
- Create `src/backend/internal/domain/services/ping/probe_checkhost.go`
  - Implement inbound public probe support through Check-Host TCP checks.
- Modify `src/backend/internal/domain/services/ping/external.go`
  - Replace member fan-out with current-node orchestration and provider execution.
- Modify `src/backend/internal/domain/services/ping/storage.go`
  - Normalize saved or legacy external config against new defaults.
- Modify `src/backend/internal/http/api/ping.go`
  - Resolve inbound target from request body or current request host, and remove required cluster-member loading for external tests.
- Modify `src/frontend/src/types/ping.ts`
  - Mirror endpoint/request/result type changes.
- Modify `src/frontend/src/store/modules/ping.ts`
  - Send direction, target, and method fields.
- Modify `src/frontend/src/views/MultiLocationPing.vue`
  - Add target inputs, current-node scoped outbound tables, and endpoint metadata display.
- Modify `src/frontend/src/views/ClusterNodeDetail.vue`
  - Stop passing `target_node_ids` and avoid presenting remote-node external latency as if it came from the remote node.
- Add and update tests in the same backend/frontend test directories.

## Task 1: Backend Types And Config Normalization

**Files:**
- Modify: `src/backend/internal/domain/services/ping/types.go`
- Modify: `src/backend/internal/domain/services/ping/storage.go`
- Modify: `src/backend/internal/domain/services/ping/external_test.go`

- [ ] **Step 1: Write failing tests for new defaults and legacy ZStatic direction**

Add these tests to `src/backend/internal/domain/services/ping/external_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
rtk go test ./src/backend/internal/domain/services/ping -run 'TestDefaultExternalConfigUsesCurrentNodeProviderModel|TestNormalizeExternalConfigCorrectsLegacyZStaticDirection' -count=1
```

Expected: FAIL because `check_host`, `linode_speedtest`, and `normalizeExternalConfig` do not exist yet.

- [ ] **Step 3: Extend Go types**

Update `src/backend/internal/domain/services/ping/types.go`:

```go
type ExternalEndpoint struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	Provider string   `json:"provider"`
	Region   string   `json:"region,omitempty"`
	Country  string   `json:"country,omitempty"`
	City     string   `json:"city,omitempty"`
	Network  string   `json:"network,omitempty"`
	Host     string   `json:"host,omitempty"`
	Port     int      `json:"port,omitempty"`
	Methods  []string `json:"methods,omitempty"`
}

type ExternalTargetRequest struct {
	Host  string `json:"host"`
	Port  int    `json:"port,omitempty"`
	Label string `json:"label,omitempty"`
}
```

Replace `ExternalTestResult` with this compatible shape:

```go
type ExternalTestResult struct {
	SourceMemberID string           `json:"source_member_id"`
	SourceLabel    string           `json:"source_label"`
	Direction      string           `json:"direction"`
	TargetMemberID string           `json:"target_member_id"`
	TargetName     string           `json:"target_name"`
	Source         ExternalEndpoint `json:"source"`
	Target         ExternalEndpoint `json:"target"`
	Method         *string          `json:"method"`
	LatencyMs      *float64         `json:"latency_ms"`
	Success        bool             `json:"success"`
	Error          *string          `json:"error"`
}
```

Replace `ExternalRunRequest` with:

```go
type ExternalRunRequest struct {
	SourceIDs     []string               `json:"source_ids"`
	TargetNodeIDs []string               `json:"target_node_ids,omitempty"`
	Direction     string                 `json:"direction,omitempty"`
	Target        *ExternalTargetRequest `json:"target,omitempty"`
	Methods       []string               `json:"methods,omitempty"`
}
```

- [ ] **Step 4: Add provider defaults and config normalization**

Create `src/backend/internal/domain/services/ping/providers.go` with:

```go
package ping

import (
	"net"
	"strconv"
	"strings"
)

const (
	DirectionInbound  = "inbound"
	DirectionOutbound = "outbound"
	MethodICMP        = "icmp"
	MethodTCP         = "tcp"
	MethodHTTP        = "http"
)

func defaultExternalSources() []ExternalSource {
	return []ExternalSource{
		{ID: "check_host", Name: "Check-Host", Type: "public_probe", Direction: DirectionInbound, Enabled: true},
		{ID: "globalping", Name: "Globalping", Type: "public_probe", Direction: DirectionInbound, Enabled: false},
		{ID: "ripe_atlas", Name: "RIPE Atlas", Type: "rest_api", Direction: DirectionInbound, Enabled: false},
		{ID: "cloudflare_workers", Name: "Cloudflare Workers", Type: "self_hosted", Direction: DirectionInbound, Enabled: false},
		{ID: "zstatic_cdn", Name: "ZStaticCDN", Type: "target_catalog", Direction: DirectionOutbound, Enabled: true},
		{ID: "linode_speedtest", Name: "Linode/Akamai Speed Test", Type: "target_catalog", Direction: DirectionOutbound, Enabled: true},
		{ID: "public_dns", Name: "Public DNS", Type: "target_catalog", Direction: DirectionOutbound, Enabled: true},
		{ID: "cdn_edges", Name: "CDN Edge Nodes", Type: "target_catalog", Direction: DirectionOutbound, Enabled: true},
		{ID: "cloud_test_ips", Name: "Cloud Provider Test IPs", Type: "target_catalog", Direction: DirectionOutbound, Enabled: true},
	}
}

func normalizeExternalConfig(config *ExternalConfig) *ExternalConfig {
	defaults := defaultExternalSources()
	if config == nil {
		return &ExternalConfig{Sources: defaults}
	}
	legacy := make(map[string]ExternalSource, len(config.Sources))
	for _, src := range config.Sources {
		legacy[src.ID] = src
	}
	merged := make([]ExternalSource, 0, len(defaults))
	for _, def := range defaults {
		if old, ok := legacy[def.ID]; ok {
			def.Enabled = old.Enabled
			def.APIKey = old.APIKey
			def.WorkerURL = old.WorkerURL
		}
		merged = append(merged, def)
	}
	return &ExternalConfig{Sources: merged}
}

func endpointAddress(endpoint ExternalEndpoint) string {
	host := strings.TrimSpace(endpoint.Host)
	if host == "" || endpoint.Port <= 0 {
		return host
	}
	return net.JoinHostPort(host, strconv.Itoa(endpoint.Port))
}

func methodAllowed(method string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, item := range allowed {
		if item == method {
			return true
		}
	}
	return false
}
```

Modify `defaultExternalConfig()` in `storage.go`:

```go
func defaultExternalConfig() *ExternalConfig {
	return normalizeExternalConfig(nil)
}
```

Modify `LoadExternalConfigOrDefault()` in `storage.go`:

```go
func (s *Store) LoadExternalConfigOrDefault() *ExternalConfig {
	config, err := s.LoadExternalConfig()
	if err != nil {
		return defaultExternalConfig()
	}
	return normalizeExternalConfig(config)
}
```

- [ ] **Step 5: Run tests to verify Task 1 passes**

Run:

```bash
rtk go test ./src/backend/internal/domain/services/ping -run 'TestDefaultExternalConfigUsesCurrentNodeProviderModel|TestNormalizeExternalConfigCorrectsLegacyZStaticDirection' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit Task 1**

Run:

```bash
rtk git add src/backend/internal/domain/services/ping/types.go src/backend/internal/domain/services/ping/storage.go src/backend/internal/domain/services/ping/providers.go src/backend/internal/domain/services/ping/external_test.go
rtk git commit -m "refactor(ping): define external endpoint model"
```

## Task 2: Outbound Target Catalogues And Method Runner

**Files:**
- Create: `src/backend/internal/domain/services/ping/target_providers.go`
- Modify: `src/backend/internal/domain/services/ping/external.go`
- Modify: `src/backend/internal/domain/services/ping/external_test.go`

- [ ] **Step 1: Write failing tests for ZStatic port catalogue and outbound current-node scope**

Add these tests to `external_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
rtk go test ./src/backend/internal/domain/services/ping -run 'TestZStaticTargetsUsePublishedPortsOnly|TestRunOutboundUsesCurrentNodeOnceWithoutMembers' -count=1
```

Expected: FAIL because `zstaticTargets` and `probeEndpoint` do not exist.

- [ ] **Step 3: Add target catalogues**

Create `target_providers.go`:

```go
package ping

func zstaticTargets() []ExternalEndpoint {
	return []ExternalEndpoint{
		{ID: "he-cm-v4", Label: "Hebei Mobile", Provider: "zstatic_cdn", Region: "Hebei", Country: "CN", Network: "China Mobile", Host: "he-cm-v4.ip.zstaticcdn.com", Port: 80, Methods: []string{MethodTCP}},
		{ID: "he-cu-v4", Label: "Hebei Unicom", Provider: "zstatic_cdn", Region: "Hebei", Country: "CN", Network: "China Unicom", Host: "he-cu-v4.ip.zstaticcdn.com", Port: 80, Methods: []string{MethodTCP}},
		{ID: "he-ct-v4", Label: "Hebei Telecom", Provider: "zstatic_cdn", Region: "Hebei", Country: "CN", Network: "China Telecom", Host: "he-ct-v4.ip.zstaticcdn.com", Port: 80, Methods: []string{MethodTCP}},
		{ID: "sx-cm-v4", Label: "Shanxi Mobile", Provider: "zstatic_cdn", Region: "Shanxi", Country: "CN", Network: "China Mobile", Host: "sx-cm-v4.ip.zstaticcdn.com", Port: 80, Methods: []string{MethodTCP}},
		{ID: "sx-cu-v4", Label: "Shanxi Unicom", Provider: "zstatic_cdn", Region: "Shanxi", Country: "CN", Network: "China Unicom", Host: "sx-cu-v4.ip.zstaticcdn.com", Port: 80, Methods: []string{MethodTCP}},
		{ID: "sx-ct-v4", Label: "Shanxi Telecom", Provider: "zstatic_cdn", Region: "Shanxi", Country: "CN", Network: "China Telecom", Host: "sx-ct-v4.ip.zstaticcdn.com", Port: 80, Methods: []string{MethodTCP}},
	}
}

func linodeSpeedtestTargets() []ExternalEndpoint {
	return []ExternalEndpoint{
		{ID: "linode-newark", Label: "Linode Newark", Provider: "linode_speedtest", Region: "New Jersey", Country: "US", Host: "speedtest.newark.linode.com", Port: 80, Methods: []string{MethodTCP, MethodHTTP, MethodICMP}},
		{ID: "linode-fremont", Label: "Linode Fremont", Provider: "linode_speedtest", Region: "California", Country: "US", Host: "speedtest.fremont.linode.com", Port: 80, Methods: []string{MethodTCP, MethodHTTP, MethodICMP}},
		{ID: "linode-frankfurt", Label: "Linode Frankfurt", Provider: "linode_speedtest", Region: "Frankfurt", Country: "DE", Host: "speedtest.frankfurt.linode.com", Port: 80, Methods: []string{MethodTCP, MethodHTTP, MethodICMP}},
		{ID: "linode-singapore", Label: "Linode Singapore", Provider: "linode_speedtest", Region: "Singapore", Country: "SG", Host: "speedtest.singapore.linode.com", Port: 80, Methods: []string{MethodTCP, MethodHTTP, MethodICMP}},
		{ID: "linode-tokyo", Label: "Linode Tokyo", Provider: "linode_speedtest", Region: "Tokyo", Country: "JP", Host: "speedtest.tokyo2.linode.com", Port: 80, Methods: []string{MethodTCP, MethodHTTP, MethodICMP}},
	}
}

func publicDNSTargets() []ExternalEndpoint {
	return []ExternalEndpoint{
		{ID: "cloudflare-dns", Label: "Cloudflare DNS", Provider: "public_dns", Country: "Global", Network: "Cloudflare", Host: "1.1.1.1", Port: 53, Methods: []string{MethodTCP, MethodICMP}},
		{ID: "google-dns", Label: "Google DNS", Provider: "public_dns", Country: "Global", Network: "Google", Host: "8.8.8.8", Port: 53, Methods: []string{MethodTCP, MethodICMP}},
		{ID: "quad9-dns", Label: "Quad9 DNS", Provider: "public_dns", Country: "Global", Network: "Quad9", Host: "9.9.9.9", Port: 53, Methods: []string{MethodTCP, MethodICMP}},
		{ID: "114-dns", Label: "114 DNS", Provider: "public_dns", Country: "CN", Network: "114DNS", Host: "114.114.114.114", Port: 53, Methods: []string{MethodTCP, MethodICMP}},
	}
}

func cdnEdgeTargets() []ExternalEndpoint {
	return []ExternalEndpoint{
		{ID: "cloudflare-edge", Label: "Cloudflare Edge", Provider: "cdn_edges", Country: "Global", Network: "Cloudflare", Host: "cloudflare.com", Port: 443, Methods: []string{MethodHTTP, MethodTCP}},
		{ID: "fastly-edge", Label: "Fastly Edge", Provider: "cdn_edges", Country: "Global", Network: "Fastly", Host: "www.fastly.com", Port: 443, Methods: []string{MethodHTTP, MethodTCP}},
		{ID: "akamai-edge", Label: "Akamai Edge", Provider: "cdn_edges", Country: "Global", Network: "Akamai", Host: "www.akamai.com", Port: 443, Methods: []string{MethodHTTP, MethodTCP}},
	}
}

func cloudTestTargets() []ExternalEndpoint {
	return []ExternalEndpoint{
		{ID: "aws-tokyo", Label: "AWS Tokyo", Provider: "cloud_test_ips", Region: "Tokyo", Country: "JP", Network: "AWS", Host: "ec2.ap-northeast-1.amazonaws.com", Port: 443, Methods: []string{MethodTCP, MethodHTTP}},
		{ID: "aws-singapore", Label: "AWS Singapore", Provider: "cloud_test_ips", Region: "Singapore", Country: "SG", Network: "AWS", Host: "ec2.ap-southeast-1.amazonaws.com", Port: 443, Methods: []string{MethodTCP, MethodHTTP}},
		{ID: "aws-virginia", Label: "AWS N. Virginia", Provider: "cloud_test_ips", Region: "Virginia", Country: "US", Network: "AWS", Host: "ec2.us-east-1.amazonaws.com", Port: 443, Methods: []string{MethodTCP, MethodHTTP}},
	}
}

func targetsForProvider(id string) []ExternalEndpoint {
	switch id {
	case "zstatic_cdn":
		return zstaticTargets()
	case "linode_speedtest":
		return linodeSpeedtestTargets()
	case "public_dns":
		return publicDNSTargets()
	case "cdn_edges":
		return cdnEdgeTargets()
	case "cloud_test_ips":
		return cloudTestTargets()
	default:
		return nil
	}
}
```

The ZStatic catalogue entries use each node's published port. Do not add code that assigns a default port to ZStatic entries.

- [ ] **Step 4: Add probe runner hooks to `ExternalService`**

In `external.go`, extend `ExternalService`:

```go
type externalEndpointProbe func(context.Context, ExternalEndpoint, []string) (string, float64, error)

type ExternalService struct {
	store         *Store
	meshSvc       *MeshService
	httpClient    *http.Client
	tcpDialer     *net.Dialer
	probeEndpoint externalEndpointProbe
}
```

Update `NewExternalService`:

```go
func NewExternalService(store *Store) *ExternalService {
	svc := &ExternalService{
		store:      store,
		meshSvc:    NewMeshService(),
		httpClient: &http.Client{Timeout: 10 * time.Second},
		tcpDialer:  &net.Dialer{Timeout: 5 * time.Second},
	}
	svc.probeEndpoint = svc.probeEndpointWithMethods
	return svc
}
```

Add:

```go
func currentPanelEndpoint() ExternalEndpoint {
	return ExternalEndpoint{
		ID:       "current-panel",
		Label:    "Current panel",
		Provider: "panel",
	}
}
```

- [ ] **Step 5: Run tests and commit Task 2**

Run:

```bash
rtk go test ./src/backend/internal/domain/services/ping -run 'TestZStaticTargetsUsePublishedPortsOnly|TestRunOutboundUsesCurrentNodeOnceWithoutMembers' -count=1
```

Expected: FAIL still if outbound orchestration has not changed. Commit only after Task 3 makes the outbound test pass.

## Task 3: Rewrite Outbound External Run Logic

**Files:**
- Modify: `src/backend/internal/domain/services/ping/external.go`
- Modify: `src/backend/internal/domain/services/ping/external_test.go`

- [ ] **Step 1: Add a failing test that deprecated target node IDs do not fan out**

Add to `external_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
rtk go test ./src/backend/internal/domain/services/ping -run TestRunOutboundIgnoresDeprecatedTargetNodeIDs -count=1
```

Expected: FAIL because current code fans out by member.

- [ ] **Step 3: Implement outbound orchestration**

In `external.go`, replace `RunOutbound` with:

```go
func (s *ExternalService) RunOutbound(ctx context.Context, sourceIDs []string, members []MeshMember) (*ExternalResultData, error) {
	config := s.store.LoadExternalConfigOrDefault()
	enabledSet := makeSourceSet(sourceIDs, config, DirectionOutbound)
	tasks := make([]func() ExternalTestResult, 0)
	source := currentPanelEndpoint()

	for sourceID := range enabledSet {
		for _, target := range targetsForProvider(sourceID) {
			target := target
			tasks = append(tasks, func() ExternalTestResult {
				return s.probeOutboundTarget(ctx, source, target, nil)
			})
		}
	}

	results := runExternalProbeTasks(DefaultExternalMaxConcurrent, tasks)
	data := &ExternalResultData{TestedAt: nowUnix(), Results: results}
	if err := s.store.SaveExternalResults(data); err != nil {
		return nil, err
	}
	return data, nil
}
```

Add:

```go
func (s *ExternalService) probeOutboundTarget(ctx context.Context, source ExternalEndpoint, target ExternalEndpoint, requestedMethods []string) ExternalTestResult {
	r := ExternalTestResult{
		SourceMemberID: source.ID,
		SourceLabel:    source.Label,
		Direction:      DirectionOutbound,
		TargetMemberID: target.ID,
		TargetName:     target.Host,
		Source:         source,
		Target:         target,
	}
	method, latency, err := s.probeEndpoint(ctx, target, requestedMethods)
	if err != nil {
		r.Success = false
		r.Error = errorPtr(err.Error())
		return r
	}
	r.Success = true
	r.Method = methodPtr(method)
	r.LatencyMs = latencyPtr(latency)
	return r
}

func (s *ExternalService) probeEndpointWithMethods(ctx context.Context, endpoint ExternalEndpoint, requestedMethods []string) (string, float64, error) {
	methods := requestedMethods
	if len(methods) == 0 {
		methods = endpoint.Methods
	}
	if len(methods) == 0 {
		methods = []string{MethodTCP}
	}

	var lastErr error
	for _, method := range methods {
		if !methodAllowed(method, endpoint.Methods) {
			continue
		}
		switch method {
		case MethodTCP:
			addr := endpointAddress(endpoint)
			latency, err := measureTCPConnectLatency(s.tcpDialer, addr)
			if err == nil {
				return MethodTCP, latency, nil
			}
			lastErr = err
		case MethodICMP:
			latency, err := s.meshSvc.icmpPing(ctx, endpoint.Host)
			if err == nil {
				return MethodICMP, latency, nil
			}
			lastErr = err
		case MethodHTTP:
			scheme := "https"
			if endpoint.Port == 80 {
				scheme = "http"
			}
			latency, err := s.meshSvc.httpPing(ctx, scheme+"://"+endpoint.Host, "")
			if err == nil {
				return MethodHTTP, latency, nil
			}
			lastErr = err
		}
	}
	if lastErr != nil {
		return "", 0, lastErr
	}
	return "", 0, fmt.Errorf("no supported methods for %s", endpoint.ID)
}
```

After replacing all references in `external.go`, delete the old `probeExternalTarget` helper and the old `inboundTargets` / `outboundTargets` maps. The new target catalogues in `target_providers.go` are the only outbound target source.

- [ ] **Step 4: Pass requested methods from `Run` into outbound**

Replace the outbound call in `Run` with a helper that accepts methods:

```go
if len(enabledOut) > 0 {
	outData, err := s.runOutboundWithMethods(ctx, enabledOut, req.Methods)
	if err != nil {
		return nil, err
	}
	allResults = append(allResults, outData.Results...)
}
```

Add:

```go
func (s *ExternalService) runOutboundWithMethods(ctx context.Context, sourceIDs []string, methods []string) (*ExternalResultData, error) {
	config := s.store.LoadExternalConfigOrDefault()
	enabledSet := makeSourceSet(sourceIDs, config, DirectionOutbound)
	source := currentPanelEndpoint()
	tasks := make([]func() ExternalTestResult, 0)
	for sourceID := range enabledSet {
		for _, target := range targetsForProvider(sourceID) {
			target := target
			tasks = append(tasks, func() ExternalTestResult {
				return s.probeOutboundTarget(ctx, source, target, methods)
			})
		}
	}
	results := runExternalProbeTasks(DefaultExternalMaxConcurrent, tasks)
	data := &ExternalResultData{TestedAt: nowUnix(), Results: results}
	if err := s.store.SaveExternalResults(data); err != nil {
		return nil, err
	}
	return data, nil
}
```

- [ ] **Step 5: Run outbound tests**

Run:

```bash
rtk go test ./src/backend/internal/domain/services/ping -run 'TestZStaticTargetsUsePublishedPortsOnly|TestRunOutboundUsesCurrentNodeOnceWithoutMembers|TestRunOutboundIgnoresDeprecatedTargetNodeIDs' -count=1
```

Expected: PASS.

- [ ] **Step 6: Run full ping package tests**

Run:

```bash
rtk go test ./src/backend/internal/domain/services/ping -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit Tasks 2 and 3**

Run:

```bash
rtk git add src/backend/internal/domain/services/ping/target_providers.go src/backend/internal/domain/services/ping/external.go src/backend/internal/domain/services/ping/external_test.go
rtk git commit -m "fix(ping): scope outbound tests to current panel"
```

## Task 4: Inbound Target Request And Check-Host Probe Provider

**Files:**
- Create: `src/backend/internal/domain/services/ping/probe_checkhost.go`
- Modify: `src/backend/internal/domain/services/ping/external.go`
- Modify: `src/backend/internal/domain/services/ping/external_test.go`

- [ ] **Step 1: Write failing test for explicit standalone inbound target**

Add to `external_test.go`:

```go
func TestRunInboundUsesExplicitTargetInStandaloneMode(t *testing.T) {
	svc := NewExternalService(newStoreWithDir(t.TempDir()))
	svc.runInboundProvider = func(ctx context.Context, sourceID string, target ExternalEndpoint) []ExternalTestResult {
		return []ExternalTestResult{{
			Direction: DirectionInbound,
			Source: ExternalEndpoint{ID: "probe-a", Label: "Probe A", Provider: sourceID, Country: "US"},
			Target: target,
			Method: methodPtr(MethodTCP),
			LatencyMs: latencyPtr(23),
			Success: true,
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
		t.Fatalf("expected 1 inbound result, got %d", len(data.Results))
	}
	if data.Results[0].Target.Host != "panel.example.com" || data.Results[0].Target.Port != 443 {
		t.Fatalf("expected explicit target, got %#v", data.Results[0].Target)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
rtk go test ./src/backend/internal/domain/services/ping -run TestRunInboundUsesExplicitTargetInStandaloneMode -count=1
```

Expected: FAIL because `runInboundProvider` and request target handling do not exist.

- [ ] **Step 3: Add inbound provider hook and target builder**

In `external.go`, extend `ExternalService`:

```go
type inboundProviderRunner func(context.Context, string, ExternalEndpoint) []ExternalTestResult

type ExternalService struct {
	store              *Store
	meshSvc            *MeshService
	httpClient         *http.Client
	tcpDialer          *net.Dialer
	probeEndpoint      externalEndpointProbe
	runInboundProvider inboundProviderRunner
}
```

Update `NewExternalService`:

```go
svc.runInboundProvider = svc.runInboundProviderDefault
```

Add:

```go
func targetFromRequest(target *ExternalTargetRequest) (ExternalEndpoint, error) {
	if target == nil || strings.TrimSpace(target.Host) == "" {
		return ExternalEndpoint{}, fmt.Errorf("inbound target host is required")
	}
	label := strings.TrimSpace(target.Label)
	if label == "" {
		label = strings.TrimSpace(target.Host)
	}
	return ExternalEndpoint{
		ID:       "current-target",
		Label:    label,
		Provider: "panel",
		Host:     strings.TrimSpace(target.Host),
		Port:     target.Port,
	}, nil
}
```

- [ ] **Step 4: Replace inbound orchestration**

Change `Run` inbound branch:

```go
if len(enabledIn) > 0 {
	target, err := targetFromRequest(req.Target)
	if err != nil {
		return nil, err
	}
	inData, err := s.runInboundWithTarget(ctx, enabledIn, target)
	if err != nil {
		return nil, err
	}
	allResults = append(allResults, inData.Results...)
}
```

Add:

```go
func (s *ExternalService) runInboundWithTarget(ctx context.Context, sourceIDs []string, target ExternalEndpoint) (*ExternalResultData, error) {
	results := make([]ExternalTestResult, 0)
	for _, sourceID := range sourceIDs {
		results = append(results, s.runInboundProvider(ctx, sourceID, target)...)
	}
	data := &ExternalResultData{TestedAt: nowUnix(), Results: results}
	if err := s.store.SaveExternalResults(data); err != nil {
		return nil, err
	}
	return data, nil
}
```

- [ ] **Step 5: Implement Check-Host provider**

Create `probe_checkhost.go`:

```go
package ping

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type checkHostStartResponse struct {
	OK        int                 `json:"ok"`
	RequestID string              `json:"request_id"`
	Nodes     map[string][]string `json:"nodes"`
}

func (s *ExternalService) runInboundProviderDefault(ctx context.Context, sourceID string, target ExternalEndpoint) []ExternalTestResult {
	switch sourceID {
	case "check_host":
		return s.runCheckHostTCP(ctx, target)
	default:
		return []ExternalTestResult{{
			Direction: DirectionInbound,
			Source: ExternalEndpoint{ID: sourceID, Label: sourceID, Provider: sourceID},
			Target: target,
			Success: false,
			Error: errorPtr("inbound provider is not enabled in this build"),
		}}
	}
}

func (s *ExternalService) runCheckHostTCP(ctx context.Context, target ExternalEndpoint) []ExternalTestResult {
	startURL := "https://check-host.net/check-tcp?host=" + url.QueryEscape(endpointAddress(target)) + "&max_nodes=12"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, startURL, nil)
	if err != nil {
		return inboundProviderError("check_host", target, err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return inboundProviderError("check_host", target, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return inboundProviderError("check_host", target, fmt.Errorf("check-host start returned %d", resp.StatusCode))
	}
	var started checkHostStartResponse
	if err := json.NewDecoder(resp.Body).Decode(&started); err != nil {
		return inboundProviderError("check_host", target, err)
	}
	if started.OK != 1 || started.RequestID == "" {
		return inboundProviderError("check_host", target, fmt.Errorf("check-host did not start measurement"))
	}
	time.Sleep(2 * time.Second)
	return s.fetchCheckHostTCPResults(ctx, started, target)
}

func inboundProviderError(provider string, target ExternalEndpoint, err error) []ExternalTestResult {
	return []ExternalTestResult{{
		Direction: DirectionInbound,
		Source: ExternalEndpoint{ID: provider, Label: provider, Provider: provider},
		Target: target,
		Success: false,
		Error: errorPtr(err.Error()),
	}}
}

func (s *ExternalService) fetchCheckHostTCPResults(ctx context.Context, started checkHostStartResponse, target ExternalEndpoint) []ExternalTestResult {
	resultURL := "https://check-host.net/check-result/" + url.PathEscape(started.RequestID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resultURL, nil)
	if err != nil {
		return inboundProviderError("check_host", target, err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return inboundProviderError("check_host", target, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return inboundProviderError("check_host", target, fmt.Errorf("check-host result returned %d", resp.StatusCode))
	}

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return inboundProviderError("check_host", target, err)
	}
	results := make([]ExternalTestResult, 0, len(raw))
	for nodeID, rawNode := range raw {
		source := checkHostEndpoint(nodeID, started.Nodes[nodeID])
		result := ExternalTestResult{
			SourceMemberID: source.ID,
			SourceLabel: source.Label,
			Direction: DirectionInbound,
			TargetMemberID: target.ID,
			TargetName: target.Label,
			Source: source,
			Target: target,
			Method: methodPtr(MethodTCP),
		}
		latency, ok := parseCheckHostTCPLatency(rawNode)
		if ok {
			result.Success = true
			result.LatencyMs = latencyPtr(latency)
		} else {
			result.Success = false
			result.Error = errorPtr("check-host node returned no TCP latency")
		}
		results = append(results, result)
	}
	return results
}

func checkHostEndpoint(nodeID string, meta []string) ExternalEndpoint {
	endpoint := ExternalEndpoint{ID: nodeID, Label: nodeID, Provider: "check_host"}
	if len(meta) > 0 {
		endpoint.Country = meta[0]
	}
	if len(meta) > 1 {
		endpoint.Region = meta[1]
	}
	if len(meta) > 2 {
		endpoint.City = meta[2]
	}
	if endpoint.City != "" {
		endpoint.Label = endpoint.City
	}
	return endpoint
}

func parseCheckHostTCPLatency(raw json.RawMessage) (float64, bool) {
	var data any
	if err := json.Unmarshal(raw, &data); err != nil {
		return 0, false
	}
	return findFirstNumericLatency(data)
}

func findFirstNumericLatency(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed * 1000, true
	case string:
		parsed, err := strconv.ParseFloat(typed, 64)
		if err == nil {
			return parsed * 1000, true
		}
	case []any:
		for _, item := range typed {
			if latency, ok := findFirstNumericLatency(item); ok {
				return latency, true
			}
		}
	}
	return 0, false
}
```

- [ ] **Step 6: Run inbound tests**

Run:

```bash
rtk go test ./src/backend/internal/domain/services/ping -run TestRunInboundUsesExplicitTargetInStandaloneMode -count=1
```

Expected: PASS.

- [ ] **Step 7: Run full ping package and commit**

Run:

```bash
rtk go test ./src/backend/internal/domain/services/ping -count=1
rtk git add src/backend/internal/domain/services/ping/external.go src/backend/internal/domain/services/ping/probe_checkhost.go src/backend/internal/domain/services/ping/external_test.go
rtk git commit -m "feat(ping): add current-node inbound probes"
```

## Task 5: API Standalone Target Resolution

**Files:**
- Modify: `src/backend/internal/http/api/ping.go`
- Modify: `src/backend/internal/http/api/ping_test.go`

- [ ] **Step 1: Write failing API tests**

Add to `ping_test.go`:

```go
type stubExternalPingService struct {
	lastReq ping.ExternalRunRequest
	result  *ping.ExternalResultData
	err     error
}

func (s *stubExternalPingService) Run(ctx context.Context, req ping.ExternalRunRequest, members []ping.MeshMember) (*ping.ExternalResultData, error) {
	s.lastReq = req
	if s.err != nil {
		return nil, s.err
	}
	if s.result != nil {
		return s.result, nil
	}
	return &ping.ExternalResultData{TestedAt: 1710000000, Results: []ping.ExternalTestResult{}}, nil
}

func TestTriggerExternalPingOutboundDoesNotRequireClusterMembers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(sessions.Sessions("b-ui", cookie.NewStore([]byte("test-secret"))))

	external := &stubExternalPingService{}
	handler := &pingAPIHandler{
		clusterService: &service.ClusterService{},
		externalSvc: external,
	}
	router.POST("/api/ping/external", handler.triggerExternalPing)
	router.GET("/__test/login/:username", func(c *gin.Context) {
		if err := SetLoginUser(c, c.Param("username"), 0); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	})

	body := strings.NewReader(`{"direction":"outbound","source_ids":["public_dns"],"methods":["tcp"]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/ping/external", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", loginCookie(t, router, "admin"))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	var response Msg
	decodeResponse(t, recorder, &response)
	if !response.Success {
		t.Fatalf("expected standalone outbound success, got %#v", response)
	}
	if external.lastReq.Direction != ping.DirectionOutbound {
		t.Fatalf("expected outbound request, got %#v", external.lastReq)
	}
	if len(external.lastReq.TargetNodeIDs) != 0 {
		t.Fatalf("expected no target node filters, got %#v", external.lastReq.TargetNodeIDs)
	}
}

func TestTriggerExternalPingInboundUsesExplicitTarget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(sessions.Sessions("b-ui", cookie.NewStore([]byte("test-secret"))))

	external := &stubExternalPingService{}
	handler := &pingAPIHandler{clusterService: &service.ClusterService{}, externalSvc: external}
	router.POST("/api/ping/external", handler.triggerExternalPing)
	router.GET("/__test/login/:username", func(c *gin.Context) {
		if err := SetLoginUser(c, c.Param("username"), 0); err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	})

	body := strings.NewReader(`{"direction":"inbound","source_ids":["check_host"],"target":{"host":"panel.example.com","port":443,"label":"Panel"}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/ping/external", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", loginCookie(t, router, "admin"))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	var response Msg
	decodeResponse(t, recorder, &response)
	if !response.Success {
		t.Fatalf("expected explicit inbound target success, got %#v", response)
	}
	if external.lastReq.Target == nil {
		t.Fatal("expected inbound target to pass through")
	}
	if external.lastReq.Target.Host != "panel.example.com" || external.lastReq.Target.Port != 443 {
		t.Fatalf("expected explicit target, got %#v", external.lastReq.Target)
	}
}
```

Update the `ping_test.go` imports to include `context`, `strings`, and the service package:

```go
import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	service "github.com/BeanYa/b-ui/src/backend/internal/domain/services"
	"github.com/BeanYa/b-ui/src/backend/internal/domain/services/ping"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
rtk go test ./src/backend/internal/http/api -run 'TestTriggerExternalPingOutboundDoesNotRequireClusterMembers|TestTriggerExternalPingInboundUsesExplicitTarget' -count=1
```

Expected: FAIL because API still calls `ListMembers()`.

- [ ] **Step 3: Remove cluster member dependency from external API**

In `ping.go`, add `context`, `net`, and `strconv` to the imports, then add a small interface above `pingAPIHandler`:

```go
type externalPingService interface {
	Run(context.Context, ping.ExternalRunRequest, []ping.MeshMember) (*ping.ExternalResultData, error)
}
```

Change `pingAPIHandler.externalSvc` to:

```go
externalSvc externalPingService
```

In `triggerExternalPing`, replace member loading with target resolution:

```go
if req.Direction == ping.DirectionInbound && req.Target == nil {
	target, err := externalTargetFromRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, Msg{Success: false, Msg: err.Error()})
		return
	}
	req.Target = target
}

data, err := h.externalSvc.Run(c.Request.Context(), req, nil)
if err != nil {
	jsonMsg(c, "external ping", err)
	return
}
jsonObj(c, data, nil)
```

Add:

```go
func externalTargetFromRequest(c *gin.Context) (*ping.ExternalTargetRequest, error) {
	host := strings.TrimSpace(c.Request.Host)
	if host == "" {
		return nil, fmt.Errorf("inbound target host is required")
	}
	if strings.Contains(host, ":") {
		hostname, port, err := net.SplitHostPort(host)
		if err == nil {
			parsedPort, _ := strconv.Atoi(port)
			return &ping.ExternalTargetRequest{Host: hostname, Port: parsedPort, Label: hostname}, nil
		}
	}
	return &ping.ExternalTargetRequest{Host: host, Label: host}, nil
}
```

Add `net` and `strconv` imports to `ping.go`.

- [ ] **Step 4: Run API tests**

Run:

```bash
rtk go test ./src/backend/internal/http/api -run 'TestTriggerExternalPingOutboundDoesNotRequireClusterMembers|TestTriggerExternalPingInboundUsesExplicitTarget' -count=1
```

Expected: PASS.

- [ ] **Step 5: Run ping and API packages, then commit**

Run:

```bash
rtk go test ./src/backend/internal/domain/services/ping ./src/backend/internal/http/api -count=1
rtk git add src/backend/internal/http/api/ping.go src/backend/internal/http/api/ping_test.go
rtk git commit -m "fix(ping): allow standalone external tests"
```

## Task 6: Frontend Types And Store Payload

**Files:**
- Modify: `src/frontend/src/types/ping.ts`
- Modify: `src/frontend/src/store/modules/ping.ts`
- Modify: `src/frontend/src/store/modules/ping.test.ts`

- [ ] **Step 1: Write failing store tests**

Update the existing `can scope external ping requests to one cluster node` test in `ping.test.ts` to:

```ts
it('sends current-node outbound ping requests without cluster node filters', async () => {
  const result: ExternalResultData = {
    tested_at: 1710000000,
    results: [],
  }
  vi.mocked(api.post).mockResolvedValueOnce({ data: { success: true, obj: result } })

  const { usePingStore } = await import('./ping')
  const store = usePingStore()

  await expect(store.triggerExternalPing({
    direction: 'outbound',
    source_ids: ['public_dns'],
    methods: ['tcp'],
  })).resolves.toEqual(result)

  expect(api.post).toHaveBeenCalledWith(
    'api/ping/external',
    { direction: 'outbound', source_ids: ['public_dns'], methods: ['tcp'] },
    { headers: { 'Content-Type': 'application/json' } },
  )
})

it('sends explicit inbound target details', async () => {
  const result: ExternalResultData = {
    tested_at: 1710000000,
    results: [],
  }
  vi.mocked(api.post).mockResolvedValueOnce({ data: { success: true, obj: result } })

  const { usePingStore } = await import('./ping')
  const store = usePingStore()

  await store.triggerExternalPing({
    direction: 'inbound',
    source_ids: ['check_host'],
    target: { host: 'panel.example.com', port: 443, label: 'Panel' },
  })

  expect(api.post).toHaveBeenCalledWith(
    'api/ping/external',
    {
      direction: 'inbound',
      source_ids: ['check_host'],
      target: { host: 'panel.example.com', port: 443, label: 'Panel' },
    },
    { headers: { 'Content-Type': 'application/json' } },
  )
})
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
cd src/frontend
npm test -- ping.test.ts
```

Expected: FAIL because `triggerExternalPing` still accepts positional args.

- [ ] **Step 3: Update frontend types**

In `src/frontend/src/types/ping.ts`, add:

```ts
export interface ExternalEndpoint {
  id: string
  label: string
  provider: string
  region?: string
  country?: string
  city?: string
  network?: string
  host?: string
  port?: number
  methods?: string[]
}

export interface ExternalTargetRequest {
  host: string
  port?: number
  label?: string
}
```

Extend `ExternalTestResult`:

```ts
  source: ExternalEndpoint
  target: ExternalEndpoint
```

Replace `ExternalRunRequest`:

```ts
export interface ExternalRunRequest {
  direction?: 'inbound' | 'outbound'
  source_ids: string[]
  target_node_ids?: string[]
  target?: ExternalTargetRequest
  methods?: string[]
}
```

- [ ] **Step 4: Update store signature**

In `src/frontend/src/store/modules/ping.ts`, replace `triggerExternalPing` with:

```ts
async function triggerExternalPing(request: ExternalRunRequest): Promise<ExternalResultData> {
  loading.value = true
  error.value = null
  try {
    const { data } = await api.post('api/ping/external', request, {
      headers: { 'Content-Type': 'application/json' },
    })
    if (data.success) {
      externalResults.value = data.obj
      return data.obj
    }
    throw new Error(data.msg)
  } catch (e: any) {
    error.value = e.message
    throw e
  } finally {
    loading.value = false
  }
}
```

- [ ] **Step 5: Run store tests and commit**

Run:

```bash
cd src/frontend
npm test -- ping.test.ts
rtk git add src/frontend/src/types/ping.ts src/frontend/src/store/modules/ping.ts src/frontend/src/store/modules/ping.test.ts
rtk git commit -m "refactor(ping): send explicit external ping requests"
```

Expected: PASS and commit succeeds from repo root if `rtk git` is run in `b-ui`.

## Task 7: MultiLocationPing Page UX

**Files:**
- Modify: `src/frontend/src/views/MultiLocationPing.vue`
- Add: `src/frontend/src/views/MultiLocationPing.test.ts`

- [ ] **Step 1: Write source-level failing tests**

Create `MultiLocationPing.test.ts`:

```ts
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

describe('MultiLocationPing view source', () => {
  const source = () => readFileSync(fileURLToPath(new URL('./MultiLocationPing.vue', import.meta.url)), 'utf8')

  it('sends inbound target host and port in the run request', () => {
    const text = source()
    expect(text).toContain('inboundTargetHost')
    expect(text).toContain('inboundTargetPort')
    expect(text).toContain("direction: 'inbound'")
    expect(text).toContain('target: {')
  })

  it('sends outbound current-node requests without target node ids', () => {
    const text = source()
    expect(text).toContain("direction: 'outbound'")
    expect(text).not.toContain('target_node_ids')
    expect(text).not.toContain('targetNodeIds')
  })

  it('renders endpoint metadata columns for external results', () => {
    const text = source()
    expect(text).toContain('endpointLabel')
    expect(text).toContain('endpointLocation')
    expect(text).toContain('endpointAddressText')
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd src/frontend
npm test -- MultiLocationPing.test.ts
```

Expected: FAIL because the view has not been updated.

- [ ] **Step 3: Update inbound run payload**

In `MultiLocationPing.vue`, add script state:

```ts
const inboundTargetHost = ref(defaultInboundTargetHost())
const inboundTargetPort = ref<number | null>(defaultInboundTargetPort())

function defaultInboundTargetHost(): string {
  const host = window.location.hostname
  if (!host || host === 'localhost' || host === '127.0.0.1' || host === '::1') return ''
  return host
}

function defaultInboundTargetPort(): number | null {
  const port = Number(window.location.port)
  if (Number.isFinite(port) && port > 0) return port
  return window.location.protocol === 'https:' ? 443 : 80
}
```

Replace `runInbound()`:

```ts
async function runInbound() {
  const ids = store.inboundSources.filter(s => s.enabled).map(s => s.id)
  const host = inboundTargetHost.value.trim()
  if (ids.length === 0 || !host) return
  await store.triggerExternalPing({
    direction: 'inbound',
    source_ids: ids,
    target: {
      host,
      ...(inboundTargetPort.value ? { port: inboundTargetPort.value } : {}),
      label: host,
    },
  })
}
```

Replace `runOutbound()`:

```ts
async function runOutbound() {
  const ids = store.outboundSources.filter(s => s.enabled).map(s => s.id)
  if (ids.length === 0) return
  await store.triggerExternalPing({
    direction: 'outbound',
    source_ids: ids,
    methods: ['tcp', 'http', 'icmp'],
  })
}
```

- [ ] **Step 4: Add inbound target inputs in the template**

Inside the inbound card before `<v-table density="compact">`, add:

```vue
<v-row dense class="mb-3">
  <v-col cols="12" md="8">
    <v-text-field
      v-model="inboundTargetHost"
      label="Target host"
      density="compact"
      variant="outlined"
      hide-details
    />
  </v-col>
  <v-col cols="12" md="4">
    <v-text-field
      v-model="inboundTargetPort"
      label="Target port"
      density="compact"
      variant="outlined"
      hide-details
      type="number"
      min="1"
      max="65535"
    />
  </v-col>
</v-row>
```

Update `runInbound()` to convert `inboundTargetPort.value` with `Number(...)` before sending it when the value is a string from the numeric text field.

- [ ] **Step 5: Add endpoint display helpers**

In the script, add:

```ts
function endpointLabel(endpoint: any, fallback: string): string {
  return endpoint?.label || fallback || '-'
}

function endpointLocation(endpoint: any): string {
  return [endpoint?.country, endpoint?.region, endpoint?.city].filter(Boolean).join(' / ') || '-'
}

function endpointAddressText(endpoint: any): string {
  if (!endpoint?.host) return '-'
  return endpoint.port ? `${endpoint.host}:${endpoint.port}` : endpoint.host
}
```

Update `inboundRows`, `inboundCols`, `outboundRows`, and `outboundCols` to use endpoint metadata. For inbound, rows should be probe endpoints:

```ts
const inboundRows = computed(() =>
  [...new Set(inboundResults.value.map(r => endpointLabel(r.source, r.source_label)))].sort()
)
const inboundCols = computed(() =>
  [...new Set(inboundResults.value.map(r => endpointLabel(r.target, r.target_name)))].sort()
)
```

For outbound, rows should be providers or target labels:

```ts
const outboundRows = computed(() =>
  [...new Set(outboundResults.value.map(r => endpointLabel(r.target, r.target_name)))].sort()
)
const outboundCols = computed(() => ['Current panel'])
```

Adjust `outboundCell(row, col)`:

```ts
function outboundCell(target: string, _source: string) {
  const r = outboundResults.value.find(x => endpointLabel(x.target, x.target_name) === target)
  if (!r) return null
  return { text: latencyText(r), success: r.success, ms: r.latency_ms }
}
```

- [ ] **Step 6: Run view tests and frontend tests**

Run:

```bash
cd src/frontend
npm test -- MultiLocationPing.test.ts ping.test.ts
```

Expected: PASS.

- [ ] **Step 7: Commit Task 7**

Run:

```bash
rtk git add src/frontend/src/views/MultiLocationPing.vue src/frontend/src/views/MultiLocationPing.test.ts
rtk git commit -m "feat(ping): show public probe and target nodes"
```

## Task 8: Cluster Node Detail Compatibility

**Files:**
- Modify: `src/frontend/src/views/ClusterNodeDetail.vue`
- Modify: `src/frontend/src/views/ClusterNodeDetail.test.ts`

- [ ] **Step 1: Update failing source test**

Replace the last test in `ClusterNodeDetail.test.ts` with:

```ts
it('uses current-panel external latency requests without cluster node fan-out', () => {
  const source = readFileSync(fileURLToPath(new URL('./ClusterNodeDetail.vue', import.meta.url)), 'utf8')

  expect(source).toContain('triggerExternalPing({')
  expect(source).toContain("direction: 'outbound'")
  expect(source).not.toContain('triggerExternalPing(enabledSourceIds, [currentNodeId.value])')
  expect(source).not.toContain('target_node_ids')
})
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd src/frontend
npm test -- ClusterNodeDetail.test.ts
```

Expected: FAIL because the view still calls positional `triggerExternalPing`.

- [ ] **Step 3: Update node detail run logic**

In `ClusterNodeDetail.vue`, replace `runLatencyTest()` with:

```ts
async function runLatencyTest() {
  latencyTesting.value = true
  try {
    await pingStore.loadExternalConfig()
    const enabledSourceIds = pingStore.externalConfig?.sources.filter(s => s.enabled).map(s => s.id) ?? []
    if (enabledSourceIds.length > 0) {
      await pingStore.triggerExternalPing({
        direction: 'outbound',
        source_ids: enabledSourceIds,
        methods: ['tcp', 'http', 'icmp'],
      })
    }
  } finally {
    latencyTesting.value = false
  }
}
```

Update computed filters so current-node result metadata is accepted:

```ts
const nodeInboundResults = computed(() => {
  const results = pingStore.externalResults?.results ?? []
  return sortedExternalByLatency(results.filter(r => r.direction === 'inbound'))
})

const nodeOutboundResults = computed(() => {
  const results = pingStore.externalResults?.results ?? []
  return sortedExternalByLatency(results.filter(r => r.direction === 'outbound'))
})
```

- [ ] **Step 4: Run tests and commit**

Run:

```bash
cd src/frontend
npm test -- ClusterNodeDetail.test.ts
rtk git add src/frontend/src/views/ClusterNodeDetail.vue src/frontend/src/views/ClusterNodeDetail.test.ts
rtk git commit -m "fix(ping): remove cluster node fan-out from latency card"
```

## Task 9: Full Verification

**Files:**
- Verify all changed backend and frontend files.

- [ ] **Step 1: Run backend ping and API tests**

Run:

```bash
rtk go test ./src/backend/internal/domain/services/ping ./src/backend/internal/http/api -count=1
```

Expected: PASS.

- [ ] **Step 2: Run broader backend tests that cover cluster interactions**

Run:

```bash
rtk go test ./src/backend/internal/domain/services ./src/backend/internal/domain/jobs ./src/backend/internal/http/api -count=1
```

Expected: PASS.

- [ ] **Step 3: Run frontend tests**

Run:

```bash
cd src/frontend
npm test
```

Expected: PASS.

- [ ] **Step 4: Run frontend build**

Run:

```bash
cd src/frontend
npm run build:dist
```

Expected: build completes successfully.

- [ ] **Step 5: Run backend compile**

Run:

```bash
rtk go build ./src/backend/cmd/b-ui
```

Expected: command exits successfully.

- [ ] **Step 6: Inspect git state**

Run:

```bash
rtk git status --short
rtk git log --oneline -8
```

Expected: only intentional files are changed or committed.

## Self-Review

Spec coverage:

- Current-node external tests are covered by Tasks 2, 3, 5, 6, 7, and 8.
- Standalone mode is covered by Tasks 4 and 5.
- ZStatic per-node host/port is covered by Task 2 and explicitly avoids a hidden default port.
- Inbound public probes are covered by Task 4 through Check-Host.
- Outbound target libraries are covered by Task 2.
- Frontend endpoint metadata display is covered by Task 7.
- Mesh no-domain behavior remains unchanged for execution and is protected by not touching mesh APIs in external tests.

The initial implementation set is Check-Host inbound plus ZStaticCDN, Linode/Akamai, DNS, CDN edge, and cloud endpoint outbound targets. Globalping and RIPE Atlas remain disabled configuration entries; they are not used by default and return a clear provider error if selected before a provider implementation is added.
