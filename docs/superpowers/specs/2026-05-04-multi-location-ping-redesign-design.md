# Multi-Location Ping Redesign

Date: 2026-05-04

## Overview

Redesign the Multi-Location Ping feature so it measures latency for the current panel/node instead of repeating local probes across every cluster member. The feature should behave like a lightweight ITDog-style latency view:

- Inbound: public external probes test the current server.
- Outbound: the current server tests public multi-location targets.
- Mesh: domain members test each other only when the panel has joined a domain.

The redesign must work in standalone mode, where the panel has not joined any domain.

## Current Problems

The existing backend stores external sources and targets in `internal/domain/services/ping/external.go`, but the execution model is incorrect:

- Inbound results are labelled as if external sources ran the probes, but the local panel process actually pings its own cluster member addresses.
- Outbound results iterate over all cluster members even though the current panel process performs every probe locally, which produces repeated or misleading rows.
- ZStaticCDN is represented as one aggregate label, so the UI cannot show which ZStatic node or region participated.
- The page depends on cluster member data for external tests, so standalone mode can fail or show no useful target.

## Goals

- Scope external latency tests to the current panel/node by default.
- Support standalone mode without requiring any cluster domain or member row.
- Show the actual public probe node or public target node used for each result.
- Let users override the inbound test target host/IP and port.
- Keep mesh testing domain-scoped and disabled/empty when there is no domain.
- Preserve cached result loading and manual test triggering.
- Add tests that prove standalone behavior and current-node scoping.

## Non-Goals

- Do not implement full remote scatter-gather external latency tests in this change.
- Do not require paid API keys for the default experience.
- Do not make the Hub the source of truth for external latency data.
- Do not store long-term latency history beyond the existing latest-result cache model.

## Terminology

**Current node** means the server process serving the panel request.

**Probe provider** means an external service that runs a test from public probe locations toward the current server. This is used for inbound latency.

**Target provider** means a public catalogue of reachable targets that the current server probes directly. This is used for outbound latency.

## Provider Model

### Probe Providers

Probe providers are used for inbound tests:

```
public probe node -> current server target
```

The interface should expose:

- Provider ID and display name
- Probe node ID
- Probe node label
- Region/country/city if available
- ISP/network if available
- Probe method
- Target host and port
- Latency or error

Default candidates:

- Check-Host API for public no-key HTTP/TCP/ICMP checks
- Globalping API for public probes with location filters
- RIPE Atlas as an optional API-key provider
- Cloudflare Workers only when a user supplies a worker URL

Public looking glass sites belong here only when they expose an API or stable workflow that runs a remote probe from the looking glass node toward the current server. A public speed-test host without remote execution belongs to target providers instead.

### Target Providers

Target providers are used for outbound tests:

```
current server -> public target node
```

The interface should expose:

- Provider ID and display name
- Target node ID
- Target node label
- Region/country/city if known
- Network/provider
- Host
- Port
- Supported probe methods

Default candidates:

- ZStaticCDN regional TCP Ping targets
- Linode/Akamai speed-test hosts
- Public DNS targets
- CDN edge hosts
- Cloud provider regional endpoint hosts

ZStaticCDN targets should default to TCP connect latency on port 80 because the public node page describes the listed hosts as TCP Ping targets. ICMP can be opt-in only when a node is known to support it.

## Data Model

Replace the ambiguous `source_label` / `target_name` display contract with explicit endpoint metadata. The backend may keep the existing JSON fields for compatibility, but new fields should be added and used by the UI.

```go
type ExternalEndpoint struct {
    ID       string `json:"id"`
    Label    string `json:"label"`
    Provider string `json:"provider"`
    Region   string `json:"region,omitempty"`
    Country  string `json:"country,omitempty"`
    City     string `json:"city,omitempty"`
    Network  string `json:"network,omitempty"`
    Host     string `json:"host,omitempty"`
    Port     int    `json:"port,omitempty"`
}

type ExternalTestResult struct {
    Direction string `json:"direction"` // inbound or outbound
    Source    ExternalEndpoint `json:"source"`
    Target    ExternalEndpoint `json:"target"`
    Method    *string `json:"method"`
    LatencyMs *float64 `json:"latency_ms"`
    Success   bool `json:"success"`
    Error     *string `json:"error"`
}
```

Compatibility mapping:

- Inbound `source_label` becomes `Source.Label`.
- Inbound `target_name` becomes `Target.Label`.
- Outbound `source_label` becomes the current node label.
- Outbound `target_name` becomes `Target.Host`.

## Request Model

External run requests should include direction and target options:

```json
{
  "direction": "inbound",
  "source_ids": ["check_host", "globalping"],
  "target": {
    "host": "panel.example.com",
    "port": 443,
    "label": "Current panel"
  }
}
```

For outbound:

```json
{
  "direction": "outbound",
  "source_ids": ["zstatic_cdn_targets", "linode_speedtest"],
  "methods": ["tcp", "http"]
}
```

If `direction` is omitted, the backend should infer it from the requested source/provider IDs for backward compatibility.

## Target Resolution

Inbound target resolution order:

1. Request body target host/port.
2. Current request host from the HTTP request, excluding localhost-style hosts when possible.
3. Local cluster member BaseURL only if the current node is a member of a domain.

If no routable target can be inferred, the API returns a clear validation error asking the user to enter a public host/IP. It must not fail because there are no cluster domains.

Outbound target resolution:

- Use static provider target catalogues and optional user-selected method filters.
- Do not call `ListMembers()` for outbound tests.
- Label source as the current node/panel.

Mesh target resolution:

- Continue using domain members only.
- No domain means no mesh options and no runtime error.

## Backend Architecture

Add a provider layer under `internal/domain/services/ping`:

```
provider.go
providers_checkhost.go
providers_globalping.go
targets_zstatic.go
targets_linode.go
targets_dns.go
external_service.go
```

Core interfaces:

```go
type ProbeProvider interface {
    ID() string
    Direction() string // inbound
    Probes(ctx context.Context, req InboundProbeRequest) ([]ExternalTestResult, error)
}

type TargetProvider interface {
    ID() string
    Direction() string // outbound
    Targets() []ExternalEndpoint
}
```

`ExternalService.Run` becomes an orchestrator:

- Normalize request.
- Resolve current-node target for inbound.
- Resolve selected target providers for outbound.
- Run bounded concurrent tasks.
- Save latest results to the existing store.
- Return partial results when some providers fail, with per-result errors.

## Probe Methods

Supported methods:

- `icmp`: shell `ping` parser, only where appropriate.
- `tcp`: TCP connect latency to host:port.
- `http`: HTTP HEAD first, GET fallback if HEAD is rejected.

Method selection:

- Target provider defines allowed methods per endpoint.
- Request can further filter methods.
- If no method is requested, use provider defaults.
- Result records the method that produced the latency.

## Frontend UX

The `MultiLocationPing.vue` page should keep the three tabs but change the external tabs:

### Inbound Tab

- Add target host and port inputs near the source table.
- Default host is inferred from `window.location.host` when usable.
- Provider/source table shows available probe providers.
- Results table rows are probe nodes, not provider aggregates.
- Columns include provider, region/city, network, method, target, latency, error.

### Outbound Tab

- Rename source table to target providers.
- Results are grouped by provider and target node.
- Show target label, region/city, host:port, method, latency, error.
- The source is displayed once as the current panel/node, not as multiple cluster members.

### Mesh Tab

- Keep existing domain selector.
- If no domains exist, show an empty state and do not block inbound/outbound tests.

## Error Handling

- Provider-wide failures should not fail the whole request when other providers return results.
- A request with no enabled selected providers returns a validation error.
- Inbound with no public target returns a validation error with remediation text.
- Standalone mode skips cluster lookups for external tests.
- HTTP/TCP timeouts are recorded per endpoint as errors.

## Testing

Backend tests:

- `ExternalService.Run` outbound does not require cluster members and produces one current-node source.
- `ExternalService.Run` inbound accepts an explicit target in standalone mode.
- Target node metadata is preserved in results.
- ZStatic targets default to TCP port 80.
- Request `target_node_ids` no longer causes outbound to fan out across cluster members for current-node external tests.
- Provider errors produce partial result errors instead of aborting all results.

API tests:

- `POST /api/ping/external` works with no cluster domains for outbound.
- `POST /api/ping/external` works with explicit inbound target and no cluster domains.
- Missing inbound target returns a clear 400-style message when no request host can be used.

Frontend tests:

- Inbound target form payload includes host and port.
- Inbound table displays probe node label/region rather than only provider label.
- Outbound table displays current-node scoped results and public target metadata.
- Mesh empty state renders when there are no domains.

## External References

- ZStaticCDN public node list: `https://lf3-ips.zstaticcdn.com`
- Linode/Akamai speed test targets: `https://www.linode.com/speed-test/`
- Check-Host API: `https://check-host.net/about/api`
- Globalping API: `https://globalping.io/docs/api.globalping.io`
- RIPE Atlas measurements API: `https://atlas.ripe.net/docs/apis/rest-api-manual/measurements/`
