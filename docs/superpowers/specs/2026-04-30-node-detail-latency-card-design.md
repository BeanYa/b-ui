# Node Detail Page — Latency Card

## Summary

Add a collapsible latency detection card to the cluster node detail page (`ClusterNodeDetail.vue`), positioned between the existing info card and the configuration tabs. The card displays inbound (去程) and outbound (回程) latency data for the specific node being viewed.

## Motivation

The backend already implements full external latency testing (inbound and outbound) via the ping service. The frontend has a dedicated MultiLocationPing page at `/ping` and latency columns in ClusterCenter. However, the node detail page — where operators spend time managing individual nodes — shows zero latency information. Adding it here gives operators immediate visibility into a node's network quality without navigating away.

## Scope

- **In scope:** Inbound (external → this node) and outbound (this node → external targets) latency display on the node detail page.
- **Out of scope:** Mesh latency (already on ClusterCenter page). RIPE Atlas integration (backend stub). Protocol-level latency signaling. MultiLocationPing page changes.

## Architecture

### Data Source

The card uses the existing `usePingStore` (`src/frontend/src/store/modules/ping.ts`), which already wraps all `/api/ping/external/*` endpoints:

- `loadExternalResults()` — fetches cached results from `GET /api/ping/external/results`
- `triggerExternal()` — triggers a fresh test via `POST /api/ping/external`

The external results structure (`ExternalResultData`) contains:
- `inbound`: array of `{ source_id, target_member_id, latency_ms, success, error }` per source-target pair
- `outbound`: array of `{ source_member_id, target_id, latency_ms, success, error }` per member-target pair

### Data Filtering

The node detail page knows the current `nodeId` from the route query. The card filters external results to only rows relevant to this node:
- **Inbound:** filter `target_member_id === nodeId`
- **Outbound:** filter `source_member_id === nodeId`

This produces a node-scoped view of the latency data.

### Existing Utilities

The frontend already provides:
- `latencyColor(ms)` — returns CSS color string (green <50ms, yellow <150ms, orange <300ms, red >=300ms)
- `latencyText(ms)` — returns formatted string like "42ms"
- `sortedByLatency(results)` — sorts results ascending by latency

These are reused directly.

## Card Structure

### Position

Inserted between the info card (`.node-detail__info-card`) and the tabs section (`.node-detail__tabs`). Uses the same `app-card-shell` class for visual consistency.

### Collapsed State (default)

```
┌─────────────────────────────────────────────────┐
│  去程 Inbound          回程 Outbound        [▶] [Run Test] │
│  42ms avg              186ms avg                         │
│  11 sources            15 targets                         │
└─────────────────────────────────────────────────┘
```

- Two stat blocks in a grid layout (50/50)
- Each block shows: label, avg latency (color-coded, large font), count
- Expand/collapse chevron button
- "Run Test" action button (triggers `triggerExternal()`)
- Loading spinner when test is running

### Expanded State

```
┌─────────────────────────────────────────────────┐
│  去程 Inbound          回程 Outbound        [▼] [Run Test] │
│  42ms avg              186ms avg                         │
│  11 sources            15 targets                         │
│─────────────────────────────────────────────────│
│  去程详情 (Inbound)                                      │
│  Linode Tokyo              18ms                          │
│  Linode Singapore           142ms                        │
│  HE LG                      35ms                         │
│  Zstatic CDN               22ms                          │
│─────────────────────────────────────────────────│
│  回程详情 (Outbound)                                     │
│  AWS Tokyo                  15ms                         │
│  GCP US-East               312ms                         │
│  AliCloud Shanghai          89ms                         │
└─────────────────────────────────────────────────┘
```

- Summary blocks remain visible at top
- Below: two sections with per-source/target breakdown tables
- Each row: name on left, latency value on right (color-coded)
- Sorted by latency ascending (using `sortedByLatency`)
- Failed probes show error text in grey instead of latency value

### Edge Cases

- **No data:** Show "暂无测试数据，点击开始测试" placeholder. Only "Run Test" button visible.
- **Test running:** Show `v-progress-linear` indeterminate bar. Disable "Run Test" button.
- **Node offline:** Card still displays last cached data. "Run Test" disabled with tooltip "节点离线".

## Implementation

### File Changes

1. **`src/frontend/src/views/ClusterNodeDetail.vue`** — Add the latency card template and logic:
   - Import `usePingStore` and latency utilities
   - Add computed properties for filtered inbound/outbound results and averages
   - Add `loadLatencyData()` called after node connection is established in `onMounted`
   - Add `runLatencyTest()` method
   - Add expanded/collapsed state ref
   - Add the card template between info card and tabs

2. **`src/frontend/src/locales/zhcn.ts`** — Add any new i18n keys (most already exist from MultiLocationPing page; verify and add only missing ones).

### No Backend Changes

The backend is fully implemented. No API changes needed.

### No New Components

The card is implemented inline in `ClusterNodeDetail.vue`. The scope is small enough that a separate component adds indirection without benefit. If the latency display grows significantly in the future, it can be extracted then.

## Testing

- Manual verification on node detail page:
  - Card renders with no data, shows placeholder
  - "Run Test" triggers external ping, shows loading, then populates data
  - Collapsed view shows correct averages and counts
  - Expanded view shows per-source/target breakdown sorted by latency
  - Color coding matches existing patterns (MultiLocationPing page)
  - Data filters correctly for the specific node being viewed
  - Card does not interfere with existing tabs functionality
