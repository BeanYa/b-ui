# External Ping Target Catalog Design

## Context

Multi-Location Ping currently treats outbound testing as a provider-level action. The frontend lets an admin enable target groups such as ZStaticCDN or Linode, then the backend probes every hard-coded target for each enabled provider. ZStaticCDN only has a few hard-coded provincial nodes, while the public ZStaticCDN page exposes a much larger IPv4 catalog: 93 provincial nodes and 223 city nodes at the time this design was written.

The requested behavior is to keep the provider groups, but let admins select concrete target nodes inside those groups. Targets should be refreshed from provider pages through a fixed script or backend command, with a frontend button that triggers refresh and updates the stored catalog. ZStaticCDN must be imported in full, defaulting to no selected nodes. Other providers, including Linode, should follow the same catalog and refresh model.

## Goals

- Store outbound target definitions in a project catalog instead of scattering provider lists through Go code.
- Import the full available ZStaticCDN IPv4 node catalog from `https://lf3-ips.zstaticcdn.com/nodes_data.js`.
- Refresh Linode/Akamai speed test targets through a provider-specific fetcher using public speedtest host data.
- Expose an admin API and frontend button to refresh target catalogs on demand.
- Let admins select multiple concrete nodes across multiple provider groups before starting outbound tests.
- Keep all outbound target nodes unselected by default.
- Group large providers by provider, then by province or city-level grouping.

## Non-Goals

- Do not automatically run large outbound scans when the page loads.
- Do not add scheduled background catalog refresh in this change.
- Do not implement IPv6 ZStaticCDN nodes unless the provider data source exposes them in the same refresh path and tests cover them.
- Do not change inbound or mesh ping behavior except where shared types need compatibility.

## Data Model

Add an external target catalog structure:

- `provider_id`: stable provider key, such as `zstatic_cdn` or `linode_speedtest`.
- `provider_name`: display name.
- `updated_at`: Unix timestamp from the last successful refresh.
- `targets`: list of concrete `ExternalEndpoint` entries.

Each target keeps the existing endpoint shape and adds enough metadata for UI grouping:

- `id`: globally stable target ID, preferably provider-prefixed when needed.
- `label`: human-readable node label.
- `provider`: provider ID.
- `region`: province, state, or broad geography.
- `city`: city name when known.
- `country`: country code or country name.
- `network`: carrier or provider network.
- `host`: target host without port.
- `port`: target port.
- `methods`: supported probe methods.
- `group`: UI grouping label, derived from province/city where applicable.
- `level`: optional source level such as `province`, `city`, or `region`.

The catalog is persisted under the ping data directory for runtime updates, with a committed seed file in the source tree. Runtime load order is:

1. Data directory catalog if it exists.
2. Committed seed catalog.
3. Small hard-coded fallback only if neither file can be read.

## Refreshers

Add a backend catalog refresh service and a CLI/script entry point that both use the same provider refreshers.

ZStaticCDN refresher:

- Fetch `https://lf3-ips.zstaticcdn.com/nodes_data.js`.
- Parse `window.nodeData.provinceBaseData`, `cityKeyList`, and `extraCityNodeMeta`.
- Produce all IPv4 endpoints available in that page data.
- Use TCP on the published port. Provincial entries currently use port `80`; generated city entries on the page use port `443`.
- Map carrier suffixes to China Mobile, China Unicom, and China Telecom.
- Group provincial nodes by province, and city nodes by province plus city.

Linode/Akamai refresher:

- Use a provider-specific source of public speedtest hosts, with an allowlisted parser instead of broad crawling.
- Normalize hosts such as `speedtest.<region>.linode.com` into target IDs and labels.
- Preserve HTTP, TCP, and ICMP methods when applicable.
- Group by geographic region.

Other providers:

- Existing Public DNS, CDN Edge, and Cloud Provider targets are moved into the seed catalog.
- If a provider has no remote refresher yet, refresh keeps its seed targets and marks the provider as static.

## API

Add admin-only endpoints:

- `GET /api/ping/external/targets`: returns the current target catalog grouped by provider.
- `POST /api/ping/external/targets/refresh`: refreshes supported providers and persists the catalog.

The refresh endpoint accepts an optional provider list. Without a list, it refreshes all refreshable providers and preserves static providers.

`POST /api/ping/external` keeps the existing request shape, but `target_node_ids` becomes active for outbound tests. When outbound sources are requested:

- If `target_node_ids` is empty, return a clear error instead of probing every provider target.
- If IDs are supplied, probe only matching targets from the selected providers.
- Unknown selected IDs are ignored unless every selected ID is unknown, in which case return a clear error.

## Frontend

The outbound tab becomes a target picker:

- Provider rows remain the top-level grouping.
- Each provider can expand to nested region/city groups.
- Each group contains concrete target checkboxes.
- Provider and subgroup controls support select all and clear for that scope.
- All targets are unchecked by default after loading.
- The Start Outbound Test button is disabled until at least one target is checked.
- The request includes both selected provider IDs and selected target node IDs.
- A refresh button calls the catalog refresh endpoint, reloads the catalog, and preserves selected IDs that still exist.

For large ZStaticCDN catalogs, the UI uses collapsible groups and compact rows rather than rendering one flat table.

## Testing

Backend tests:

- ZStaticCDN fixture parsing imports all fixture nodes and preserves provider, group, host, port, carrier, and methods.
- Catalog loader prefers runtime data over seed data and falls back safely.
- Outbound run filters targets by `target_node_ids`.
- Empty outbound selection returns a clear error.
- Refresh endpoint persists refreshed targets.

Frontend tests:

- Outbound start is disabled with no selected targets.
- Selecting nodes under multiple providers sends the expected `source_ids` and `target_node_ids`.
- Refresh button calls the refresh API and reloads target catalog data.
- Collapsed provider and subgroup selection logic keeps selected IDs consistent.

## Rollout

1. Add seed catalog and parser fixtures.
2. Implement backend catalog load, refresh, API, and outbound filtering.
3. Update frontend store types and outbound picker UI.
4. Verify Go tests, frontend unit tests, and build.
