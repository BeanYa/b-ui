# Node Detail Latency Card Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a collapsible inbound/outbound latency card to the cluster node detail page, with manual test triggering only.

**Architecture:** The card lives inline in `ClusterNodeDetail.vue`, using the existing `usePingStore` for data. The backend `ExternalTestResult` struct needs a `SourceMemberID` field so outbound results can be filtered per-node. No auto-triggering — cached results load on mount, a button triggers fresh tests.

**Tech Stack:** Vue 3 + Vuetify + Pinia (frontend), Go/Gin (backend)

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `src/backend/internal/domain/services/ping/types.go` | Modify | Add `SourceMemberID` field to `ExternalTestResult` |
| `src/backend/internal/domain/services/ping/external.go` | Modify | Populate `SourceMemberID` in `RunInbound`, `RunOutbound`, `probeExternalTarget`, `RunRIPEAtlas` |
| `src/frontend/src/types/ping.ts` | Modify | Add `source_member_id` to `ExternalTestResult`, add `sortedExternalByLatency` helper |
| `src/frontend/src/views/ClusterNodeDetail.vue` | Modify | Add latency card template, computed properties, and logic |
| `src/frontend/src/locales/zhcn.ts` | Modify | Add i18n keys under `nodeDetail` namespace |

---

### Task 1: Add `SourceMemberID` to backend `ExternalTestResult`

**Files:**
- Modify: `src/backend/internal/domain/services/ping/types.go:52-61`

- [ ] **Step 1: Add the field to the struct**

In `types.go`, add `SourceMemberID` field to `ExternalTestResult`:

```go
type ExternalTestResult struct {
	SourceMemberID string  `json:"source_member_id"`
	SourceLabel    string  `json:"source_label"`
	Direction      string  `json:"direction"`
	TargetMemberID string  `json:"target_member_id"`
	TargetName     string  `json:"target_name"`
	Method         *string `json:"method"`
	LatencyMs      *float64 `json:"latency_ms"`
	Success        bool    `json:"success"`
	Error          *string `json:"error"`
}
```

- [ ] **Step 2: Verify build**

Run: `cd src/backend && go build ./...`
Expected: compiles (field is zero-value string for now, no breakage)

- [ ] **Step 3: Commit**

```bash
git add src/backend/internal/domain/services/ping/types.go
git commit -m "feat(ping): add SourceMemberID to ExternalTestResult struct"
```

---

### Task 2: Populate `SourceMemberID` in backend outbound and inbound results

**Files:**
- Modify: `src/backend/internal/domain/services/ping/external.go:89-127` (RunInbound)
- Modify: `src/backend/internal/domain/services/ping/external.go:130-154` (RunOutbound)
- Modify: `src/backend/internal/domain/services/ping/external.go:156-210` (probeExternalTarget)
- Modify: `src/backend/internal/domain/services/ping/external.go:212-236` (RunRIPEAtlas)

- [ ] **Step 1: Update `RunInbound` — set `SourceMemberID` to empty for inbound (source is external)**

In `RunInbound`, the result construction at line ~103:

```go
r := ExternalTestResult{
	SourceMemberID: "",
	SourceLabel:    tgt.Label,
	Direction:      "inbound",
	TargetMemberID: member.MemberID,
	TargetName:     member.Name,
}
```

- [ ] **Step 2: Update `probeExternalTarget` — set `SourceMemberID` to the member's ID**

In `probeExternalTarget`, the result construction at line ~157:

```go
r := ExternalTestResult{
	SourceMemberID: member.MemberID,
	SourceLabel:    member.Name,
	Direction:      "outbound",
	TargetMemberID: tgt.Label,
	TargetName:     tgt.IP,
}
```

- [ ] **Step 3: Update `RunRIPEAtlas` — set `SourceMemberID` to empty for inbound**

In `RunRIPEAtlas`, the result construction at line ~223:

```go
r := ExternalTestResult{
	SourceMemberID: "",
	SourceLabel:    "RIPE Atlas",
	Direction:      "inbound",
	TargetMemberID: member.MemberID,
	TargetName:     member.Name,
	Success:        false,
	Error:          errorPtr("RIPE Atlas integration not implemented — requires measurement lifecycle management"),
}
```

- [ ] **Step 4: Verify build**

Run: `cd src/backend && go build ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src/backend/internal/domain/services/ping/external.go
git commit -m "feat(ping): populate SourceMemberID in external test results"
```

---

### Task 3: Update frontend TypeScript type and add sorting helper

**Files:**
- Modify: `src/frontend/src/types/ping.ts:32-41`

- [ ] **Step 1: Add `source_member_id` field to `ExternalTestResult`**

```typescript
export interface ExternalTestResult {
  source_member_id: string
  source_label: string
  direction: 'inbound' | 'outbound'
  target_member_id: string
  target_name: string
  method: string | null
  latency_ms: number | null
  success: boolean
  error: string | null
}
```

- [ ] **Step 2: Add `sortedExternalByLatency` helper after `sortedByLatency` (after line 71)**

```typescript
export function sortedExternalByLatency(results: ExternalTestResult[]): ExternalTestResult[] {
  return [...results]
    .filter(r => r.success && r.latency_ms !== null)
    .sort((a, b) => (a.latency_ms ?? Infinity) - (b.latency_ms ?? Infinity))
}
```

- [ ] **Step 3: Verify build**

Run: `cd src/frontend && npx vue-tsc --noEmit 2>&1 | head -20`
Expected: no type errors related to ping.ts (other errors may exist, ignore)

- [ ] **Step 4: Commit**

```bash
git add src/frontend/src/types/ping.ts
git commit -m "feat(ping): add source_member_id to ExternalTestResult type and sorting helper"
```

---

### Task 4: Add i18n keys

**Files:**
- Modify: `src/frontend/src/locales/zhcn.ts`

- [ ] **Step 1: Add keys under the `ping` section (after line 835, before closing brace at 836)**

Add these keys inside the `ping` object:

```typescript
    nodeCardTitle: '延迟检测',
    runTest: '开始测试',
    noData: '暂无测试数据，点击开始测试。',
    nodeOffline: '节点离线',
    avgLatency: '平均延迟',
    sources: '个来源',
    targets: '个目标',
    inboundDetail: '去程详情',
    outboundDetail: '回程详情',
    testing: '测试中...',
```

The full `ping` section should end like:

```typescript
    notTested: '-',
    nodeCardTitle: '延迟检测',
    runTest: '开始测试',
    noData: '暂无测试数据，点击开始测试。',
    nodeOffline: '节点离线',
    avgLatency: '平均延迟',
    sources: '个来源',
    targets: '个目标',
    inboundDetail: '去程详情',
    outboundDetail: '回程详情',
    testing: '测试中...',
  },
```

- [ ] **Step 2: Commit**

```bash
git add src/frontend/src/locales/zhcn.ts
git commit -m "feat(ping): add i18n keys for node detail latency card"
```

---

### Task 5: Add latency card to `ClusterNodeDetail.vue`

**Files:**
- Modify: `src/frontend/src/views/ClusterNodeDetail.vue`

This is the main task. The changes are:

1. Add imports for ping store and utilities
2. Add reactive state and computed properties for latency data
3. Load cached results on mount (after node connection is resolved)
4. Add the card template between info card and tabs
5. Add styles

- [ ] **Step 1: Add imports**

After the existing imports (line ~169), add:

```typescript
import { usePingStore } from '@/store/modules/ping'
import { latencyColor, latencyText, sortedExternalByLatency } from '@/types/ping'
import type { ExternalTestResult } from '@/types/ping'
```

- [ ] **Step 2: Add ping store and state**

After `const remoteNode = useRemoteNodeStore()` (line ~173), add:

```typescript
const pingStore = usePingStore()
const latencyExpanded = ref(false)
const latencyTesting = ref(false)
```

- [ ] **Step 3: Add computed properties for filtered data**

After the `isPageLoading` computed (line ~185), add:

```typescript
const nodeId = computed(() => nodeConnection.value?.nodeId ?? '')

const nodeInboundResults = computed(() => {
  const results = pingStore.externalResults?.results ?? []
  return sortedExternalByLatency(results.filter(r => r.direction === 'inbound' && r.target_member_id === nodeId.value))
})

const nodeOutboundResults = computed(() => {
  const results = pingStore.externalResults?.results ?? []
  return sortedExternalByLatency(results.filter(r => r.direction === 'outbound' && r.source_member_id === nodeId.value))
})

const inboundAvg = computed(() => {
  if (nodeInboundResults.value.length === 0) return null
  const sum = nodeInboundResults.value.reduce((a, r) => a + (r.latency_ms ?? 0), 0)
  return Math.round(sum / nodeInboundResults.value.length)
})

const outboundAvg = computed(() => {
  if (nodeOutboundResults.value.length === 0) return null
  const sum = nodeOutboundResults.value.reduce((a, r) => a + (r.latency_ms ?? 0), 0)
  return Math.round(sum / nodeOutboundResults.value.length)
})

const hasLatencyData = computed(() => nodeInboundResults.value.length > 0 || nodeOutboundResults.value.length > 0)
const nodeOffline = computed(() => nodeMember.value?.status === 'offline')

async function runLatencyTest() {
  latencyTesting.value = true
  try {
    await pingStore.loadExternalConfig()
    const allSourceIds = pingStore.externalConfig?.sources.map(s => s.id) ?? []
    if (allSourceIds.length > 0) {
      await pingStore.triggerExternalPing(allSourceIds)
    }
  } finally {
    latencyTesting.value = false
  }
}
```

- [ ] **Step 4: Load cached results on mount**

In the `onMounted` callback, after `nodeConnection.value = await loadNodeConnection(nodeId)` succeeds (around line ~307), add:

```typescript
pingStore.loadExternalResults().catch(() => {})
```

- [ ] **Step 5: Add the latency card template**

Insert the following between the info card `</v-card>` (line 69) and `<template v-if="supportsPanelExperience">` (line 71):

```html
      <v-card class="app-card-shell node-detail__latency-card">
        <v-card-text>
          <div class="node-detail__latency-header">
            <span class="node-detail__latency-title">{{ $t('ping.nodeCardTitle') }}</span>
            <div class="node-detail__latency-actions">
              <v-btn
                variant="outlined"
                size="small"
                :loading="latencyTesting"
                :disabled="latencyTesting || nodeOffline"
                @click="runLatencyTest"
              >
                <v-tooltip v-if="nodeOffline" activator="parent" location="top">
                  {{ $t('ping.nodeOffline') }}
                </v-tooltip>
                {{ latencyTesting ? $t('ping.testing') : $t('ping.runTest') }}
              </v-btn>
              <v-btn
                v-if="hasLatencyData"
                icon
                variant="text"
                size="small"
                @click="latencyExpanded = !latencyExpanded"
              >
                <v-icon>{{ latencyExpanded ? 'mdi-chevron-up' : 'mdi-chevron-down' }}</v-icon>
              </v-btn>
            </div>
          </div>

          <v-progress-linear v-if="latencyTesting" indeterminate color="primary" class="mt-2 mb-2" />

          <div v-if="!hasLatencyData && !latencyTesting" class="node-detail__latency-empty">
            {{ $t('ping.noData') }}
          </div>

          <div v-if="hasLatencyData" class="node-detail__latency-summary">
            <div class="node-detail__latency-stat">
              <div class="node-detail__latency-stat-label">{{ $t('ping.inbound') }}</div>
              <div
                class="node-detail__latency-stat-value"
                :style="{ color: latencyColor(inboundAvg, true) }"
              >
                {{ inboundAvg !== null ? `${inboundAvg}ms` : '-' }}
              </div>
              <div class="node-detail__latency-stat-sub">{{ nodeInboundResults.length }}{{ $t('ping.sources') }}</div>
            </div>
            <div class="node-detail__latency-stat">
              <div class="node-detail__latency-stat-label">{{ $t('ping.outbound') }}</div>
              <div
                class="node-detail__latency-stat-value"
                :style="{ color: latencyColor(outboundAvg, true) }"
              >
                {{ outboundAvg !== null ? `${outboundAvg}ms` : '-' }}
              </div>
              <div class="node-detail__latency-stat-sub">{{ nodeOutboundResults.length }}{{ $t('ping.targets') }}</div>
            </div>
          </div>

          <template v-if="latencyExpanded">
            <div v-if="nodeInboundResults.length > 0" class="node-detail__latency-detail">
              <div class="node-detail__latency-detail-title">{{ $t('ping.inboundDetail') }}</div>
              <div class="node-detail__latency-detail-rows">
                <div v-for="r in nodeInboundResults" :key="r.source_label" class="node-detail__latency-detail-row">
                  <span class="node-detail__latency-detail-name">{{ r.source_label }}</span>
                  <span
                    class="node-detail__latency-detail-value"
                    :style="{ color: latencyColor(r.latency_ms, r.success) }"
                  >
                    {{ latencyText(r) }}
                  </span>
                </div>
              </div>
            </div>
            <div v-if="nodeOutboundResults.length > 0" class="node-detail__latency-detail">
              <div class="node-detail__latency-detail-title">{{ $t('ping.outboundDetail') }}</div>
              <div class="node-detail__latency-detail-rows">
                <div v-for="r in nodeOutboundResults" :key="r.target_member_id + '-' + r.target_name" class="node-detail__latency-detail-row">
                  <span class="node-detail__latency-detail-name">{{ r.target_member_id }}</span>
                  <span
                    class="node-detail__latency-detail-value"
                    :style="{ color: latencyColor(r.latency_ms, r.success) }"
                  >
                    {{ latencyText(r) }}
                  </span>
                </div>
              </div>
            </div>
          </template>
        </v-card-text>
      </v-card>
```

- [ ] **Step 6: Add styles**

Append to the `<style scoped>` section:

```css
.node-detail__latency-card {
  margin-bottom: 16px;
}

.node-detail__latency-header {
  align-items: center;
  display: flex;
  justify-content: space-between;
}

.node-detail__latency-title {
  font-size: 14px;
  font-weight: 600;
}

.node-detail__latency-actions {
  display: flex;
  gap: 4px;
}

.node-detail__latency-empty {
  color: var(--app-text-3);
  font-size: 13px;
  padding: 16px 0;
  text-align: center;
}

.node-detail__latency-summary {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  margin-top: 12px;
}

.node-detail__latency-stat {
  background: rgba(128, 128, 128, 0.08);
  border-radius: 8px;
  padding: 12px;
  text-align: center;
}

.node-detail__latency-stat-label {
  color: var(--app-text-3);
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.08em;
}

.node-detail__latency-stat-value {
  font-size: 24px;
  font-weight: 700;
  line-height: 1.2;
  margin: 4px 0;
}

.node-detail__latency-stat-sub {
  color: var(--app-text-3);
  font-size: 11px;
}

.node-detail__latency-detail {
  border-top: 1px solid var(--app-border-1);
  margin-top: 12px;
  padding-top: 8px;
}

.node-detail__latency-detail-title {
  color: var(--app-text-3);
  font-size: 11px;
  font-weight: 600;
  margin-bottom: 6px;
  text-transform: uppercase;
  letter-spacing: 0.06em;
}

.node-detail__latency-detail-rows {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.node-detail__latency-detail-row {
  align-items: center;
  display: flex;
  font-size: 12px;
  justify-content: space-between;
  padding: 2px 0;
}

.node-detail__latency-detail-name {
  color: var(--app-text-2);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.node-detail__latency-detail-value {
  font-family: var(--app-font-mono, ui-monospace, monospace);
  font-size: 12px;
  font-weight: 600;
  margin-left: 12px;
  white-space: nowrap;
}

@media (max-width: 480px) {
  .node-detail__latency-summary {
    grid-template-columns: 1fr;
  }
}
```

- [ ] **Step 7: Verify dev build**

Run: `cd src/frontend && npx vue-tsc --noEmit 2>&1 | head -30`
Expected: no type errors in ClusterNodeDetail.vue

- [ ] **Step 8: Commit**

```bash
git add src/frontend/src/views/ClusterNodeDetail.vue
git commit -m "feat(cluster): add collapsible latency card to node detail page"
```

---

## Self-Review

**Spec coverage:**
- Collapsible card between info card and tabs → Task 5
- Summary view with avg inbound/outbound → Task 5
- Expandable detail with per-source/target breakdown → Task 5
- No auto-trigger, manual button only → Task 5 (no cron/auto-call, `runLatencyTest` is button-triggered only)
- Load cached results on mount → Task 5 Step 4
- Node offline handling → Task 5 (disabled button with tooltip)
- No data placeholder → Task 5
- Color coding with `latencyColor()` → Task 5
- Sorted by latency → Task 3 + Task 5

**Placeholder scan:** No TBDs, TODOs, or vague steps. All code is complete.

**Type consistency:**
- `source_member_id: string` in TS matches `SourceMemberID string` with `json:"source_member_id"` in Go
- `sortedExternalByLatency` accepts `ExternalTestResult[]` (defined in Task 3, used in Task 5)
- `latencyColor(ms, success)` signature matches usage in Task 5
- `latencyText(r)` accepts `ExternalTestResult` (already typed for union `MeshPairResult | ExternalTestResult`)

**Gap found:** The spec said "filter `source_member_id === nodeId` for outbound" but the backend didn't have this field. Tasks 1-2 fix this. The plan covers it.
