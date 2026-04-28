# Cluster Domain Action Tree Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the flat supported-actions text in the Cluster Center domain detail card with a fixed-height, collapsible action tree and a denser metadata layout that scales as actions grow.

**Architecture:** Keep `ClusterCenter.vue` responsible for page-level data loading and selected-domain layout, but move action-tree rendering into a focused `ClusterDomainActionTree.vue` component. Back the component with a pure `clusterDomainActionTree.ts` utility that builds a merged prefix tree and flattens only the currently visible rows, so default-collapsed behavior is testable without mounting Vue components.

**Tech Stack:** Vue 3 `<script setup>`, TypeScript, Vitest, Vuetify 4, scoped CSS.

---

## File Map

- `src/frontend/src/utils/clusterDomainActionTree.ts`
  - Pure helpers for turning `supportedActions: string[]` into merged tree nodes and a visible-row list based on expanded keys.
- `src/frontend/src/utils/clusterDomainActionTree.test.ts`
  - Unit coverage for tree construction, prefix merging, and default-collapsed row visibility.
- `src/frontend/src/components/ClusterDomainActionTree.vue`
  - Scrollable, collapsible action tree rail that uses the shared utility and keeps expand/collapse state local.
- `src/frontend/src/components/ClusterDomainActionTree.test.ts`
  - Source assertions guarding the component architecture without introducing new mounting libraries.
- `src/frontend/src/views/ClusterCenter.vue`
  - Replace the flat supported-actions field with the new split metadata/tree layout and responsive styles.
- `src/frontend/src/views/ClusterCenter.test.ts`
  - Source assertions proving the view integrates the dedicated tree component and removes flat-string rendering.

## Task 1: Create Tree Utility and Unit Tests

**Files:**
- Create: `src/frontend/src/utils/clusterDomainActionTree.ts`
- Create: `src/frontend/src/utils/clusterDomainActionTree.test.ts`

- [ ] **Step 1: Write the failing utility test**

```ts
import { describe, expect, it } from 'vitest'

import {
  buildClusterDomainActionTree,
  flattenVisibleClusterDomainActionTree,
} from './clusterDomainActionTree'

describe('buildClusterDomainActionTree', () => {
  it('merges shared prefixes and preserves first-seen top-level order', () => {
    const tree = buildClusterDomainActionTree([
      'domain.cluster.changed',
      'domain.panel.update.available',
      'events',
      'heartbeat',
      '',
      'domain.cluster.changed',
    ])

    expect(tree.map(node => node.key)).toEqual([
      'domain',
      'events',
      'heartbeat',
    ])
    expect(tree[0].children.map(node => node.key)).toEqual([
      'domain.cluster',
      'domain.panel',
    ])
    expect(tree[0].children[0].children[0].key).toBe('domain.cluster.changed')
    expect(tree[0].children[1].children[0].key).toBe('domain.panel.update')
  })
})

describe('flattenVisibleClusterDomainActionTree', () => {
  const tree = buildClusterDomainActionTree([
    'domain.cluster.changed',
    'domain.panel.update.available',
    'events',
  ])

  it('shows only top-level rows when no keys are expanded', () => {
    expect(
      flattenVisibleClusterDomainActionTree(tree, new Set()).map(row => row.key),
    ).toEqual(['domain', 'events'])
  })

  it('reveals only the next depth for expanded branches', () => {
    expect(
      flattenVisibleClusterDomainActionTree(
        tree,
        new Set(['domain', 'domain.panel', 'domain.panel.update']),
      ).map(row => `${row.depth}:${row.key}`),
    ).toEqual([
      '0:domain',
      '1:domain.cluster',
      '1:domain.panel',
      '2:domain.panel.update',
      '3:domain.panel.update.available',
      '0:events',
    ])
  })

  it('returns an empty row list when the action list is empty', () => {
    expect(flattenVisibleClusterDomainActionTree([], new Set())).toEqual([])
  })
})
```

- [ ] **Step 2: Run the utility test to verify it fails**

Run:

```bash
cd C:/universe/workspace/repo/b-project/b-ui/src/frontend
npx vitest run src/utils/clusterDomainActionTree.test.ts
```

Expected: FAIL with `Cannot find module './clusterDomainActionTree'` or `ENOENT` because the utility file does not exist yet.

- [ ] **Step 3: Write the minimal utility implementation**

```ts
export interface ClusterDomainActionTreeNode {
  key: string
  label: string
  children: ClusterDomainActionTreeNode[]
}

export interface ClusterDomainActionTreeRow {
  key: string
  label: string
  depth: number
  hasChildren: boolean
}

const splitActionSegments = (value: string) => value
  .split('.')
  .map(segment => segment.trim())
  .filter(Boolean)

export const buildClusterDomainActionTree = (supportedActions: string[] = []): ClusterDomainActionTreeNode[] => {
  const roots: ClusterDomainActionTreeNode[] = []

  for (const action of supportedActions) {
    if (typeof action !== 'string') continue

    const segments = splitActionSegments(action)
    if (segments.length === 0) continue

    let branch = roots
    let currentKey = ''

    for (const segment of segments) {
      currentKey = currentKey ? `${currentKey}.${segment}` : segment

      let node = branch.find(entry => entry.key === currentKey)
      if (!node) {
        node = {
          key: currentKey,
          label: segment,
          children: [],
        }
        branch.push(node)
      }

      branch = node.children
    }
  }

  return roots
}

export const flattenVisibleClusterDomainActionTree = (
  nodes: ClusterDomainActionTreeNode[],
  expandedKeys: ReadonlySet<string>,
): ClusterDomainActionTreeRow[] => {
  const rows: ClusterDomainActionTreeRow[] = []

  const visit = (branch: ClusterDomainActionTreeNode[], depth: number) => {
    for (const node of branch) {
      const hasChildren = node.children.length > 0

      rows.push({
        key: node.key,
        label: node.label,
        depth,
        hasChildren,
      })

      if (hasChildren && expandedKeys.has(node.key)) {
        visit(node.children, depth + 1)
      }
    }
  }

  visit(nodes, 0)
  return rows
}
```

- [ ] **Step 4: Run the utility test to verify it passes**

Run:

```bash
cd C:/universe/workspace/repo/b-project/b-ui/src/frontend
npx vitest run src/utils/clusterDomainActionTree.test.ts
```

Expected: PASS with 4 tests green in `clusterDomainActionTree.test.ts`.

- [ ] **Step 5: Commit the utility slice**

```bash
cd C:/universe/workspace/repo/b-project/b-ui
git add -- src/frontend/src/utils/clusterDomainActionTree.ts src/frontend/src/utils/clusterDomainActionTree.test.ts
git commit -m "feat(frontend): add cluster domain action tree utility" -- src/frontend/src/utils/clusterDomainActionTree.ts src/frontend/src/utils/clusterDomainActionTree.test.ts
```

---

## Task 2: Create the Dedicated Action Tree Component

**Files:**
- Create: `src/frontend/src/components/ClusterDomainActionTree.vue`
- Create: `src/frontend/src/components/ClusterDomainActionTree.test.ts`

- [ ] **Step 1: Write the failing component source test**

```ts
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

describe('ClusterDomainActionTree source', () => {
  it('uses the shared tree utility and keeps expanded keys local to the component', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterDomainActionTree.vue', import.meta.url)), 'utf8')

    expect(source).toContain('buildClusterDomainActionTree')
    expect(source).toContain('flattenVisibleClusterDomainActionTree')
    expect(source).toContain('const expandedKeys = ref<Set<string>>(new Set())')
    expect(source).toContain('const visibleRows = computed(() => flattenVisibleClusterDomainActionTree(tree.value, expandedKeys.value))')
  })

  it('renders a scrollable row list, an inline empty state, and branch-only toggling', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterDomainActionTree.vue', import.meta.url)), 'utf8')

    expect(source).toContain('v-for="row in visibleRows"')
    expect(source).toContain('class="cluster-domain-action-tree__empty"')
    expect(source).toContain('class="cluster-domain-action-tree__scroll"')
    expect(source).toContain('if (!row.hasChildren) return')
    expect(source).toContain("cluster-domain-action-tree__row--expanded")
  })
})
```

- [ ] **Step 2: Run the component source test to verify it fails**

Run:

```bash
cd C:/universe/workspace/repo/b-project/b-ui/src/frontend
npx vitest run src/components/ClusterDomainActionTree.test.ts
```

Expected: FAIL with `ENOENT` because `ClusterDomainActionTree.vue` has not been created yet.

- [ ] **Step 3: Write the minimal action tree component**

```vue
<template>
  <div class="cluster-domain-action-tree">
    <div v-if="visibleRows.length === 0" class="cluster-domain-action-tree__empty">
      {{ emptyText }}
    </div>
    <div v-else class="cluster-domain-action-tree__scroll">
      <button
        v-for="row in visibleRows"
        :key="row.key"
        type="button"
        class="cluster-domain-action-tree__row"
        :class="{
          'cluster-domain-action-tree__row--branch': row.hasChildren,
          'cluster-domain-action-tree__row--expanded': expandedKeys.has(row.key),
        }"
        :style="{ '--cluster-action-depth': String(row.depth) }"
        @click="toggleRow(row)"
      >
        <span class="cluster-domain-action-tree__chevron">
          {{ row.hasChildren ? (expandedKeys.has(row.key) ? 'v' : '>') : '' }}
        </span>
        <span class="cluster-domain-action-tree__label">{{ row.label }}</span>
      </button>
    </div>
  </div>
</template>

<script lang="ts" setup>
import { computed, ref } from 'vue'

import {
  buildClusterDomainActionTree,
  flattenVisibleClusterDomainActionTree,
  type ClusterDomainActionTreeRow,
} from '@/utils/clusterDomainActionTree'

const props = withDefaults(defineProps<{
  supportedActions?: string[]
  emptyText?: string
}>(), {
  supportedActions: () => [],
  emptyText: '-',
})

const expandedKeys = ref<Set<string>>(new Set())
const tree = computed(() => buildClusterDomainActionTree(props.supportedActions))
const visibleRows = computed(() => flattenVisibleClusterDomainActionTree(tree.value, expandedKeys.value))

const toggleRow = (row: ClusterDomainActionTreeRow) => {
  if (!row.hasChildren) return

  const next = new Set(expandedKeys.value)
  if (next.has(row.key)) {
    next.delete(row.key)
  } else {
    next.add(row.key)
  }
  expandedKeys.value = next
}
</script>

<style scoped>
.cluster-domain-action-tree {
  min-height: 0;
}

.cluster-domain-action-tree__scroll {
  display: grid;
  gap: 4px;
  max-height: 240px;
  min-height: 240px;
  overflow: auto;
  padding-right: 4px;
}

.cluster-domain-action-tree__empty {
  align-items: center;
  color: var(--app-text-3);
  display: flex;
  min-height: 240px;
}

.cluster-domain-action-tree__row {
  align-items: center;
  background: transparent;
  border: 0;
  border-radius: 12px;
  color: inherit;
  cursor: default;
  display: flex;
  gap: 8px;
  min-height: 34px;
  padding: 8px 10px;
  padding-inline-start: calc(10px + (var(--cluster-action-depth) * 18px));
  text-align: left;
  width: 100%;
}

.cluster-domain-action-tree__row--branch {
  cursor: pointer;
}

.cluster-domain-action-tree__row--branch:hover {
  background: color-mix(in srgb, var(--app-state-info) 10%, transparent);
}

.cluster-domain-action-tree__row--expanded {
  background: color-mix(in srgb, var(--app-state-info) 7%, transparent);
}

.cluster-domain-action-tree__chevron {
  color: var(--app-text-3);
  flex: 0 0 14px;
  text-align: center;
}

.cluster-domain-action-tree__label {
  overflow-wrap: anywhere;
}

@media (max-width: 640px) {
  .cluster-domain-action-tree__scroll,
  .cluster-domain-action-tree__empty {
    max-height: 208px;
    min-height: 208px;
  }
}
</style>
```

- [ ] **Step 4: Run the component source test to verify it passes**

Run:

```bash
cd C:/universe/workspace/repo/b-project/b-ui/src/frontend
npx vitest run src/components/ClusterDomainActionTree.test.ts
```

Expected: PASS with both source assertions green.

- [ ] **Step 5: Commit the component slice**

```bash
cd C:/universe/workspace/repo/b-project/b-ui
git add -- src/frontend/src/components/ClusterDomainActionTree.vue src/frontend/src/components/ClusterDomainActionTree.test.ts
git commit -m "feat(frontend): add cluster domain action tree component" -- src/frontend/src/components/ClusterDomainActionTree.vue src/frontend/src/components/ClusterDomainActionTree.test.ts
```

---

## Task 3: Integrate the Tree into ClusterCenter and Replace the Flat Metadata Grid

**Files:**
- Modify: `src/frontend/src/views/ClusterCenter.vue`
- Modify: `src/frontend/src/views/ClusterCenter.test.ts`

- [ ] **Step 1: Update the ClusterCenter source test so the old flat rendering fails**

```ts
it('renders a detail state with back navigation, domain metadata rows, a dedicated action tree rail, and registered cluster servers', () => {
  const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')

  expect(source).toContain('v-else class="cluster-center__detail"')
  expect(source).toContain('@click="backToClusterCenter"')
  expect(source).toContain('ClusterDomainActionTree')
  expect(source).toContain('class="cluster-center__detail-panel"')
  expect(source).toContain('class="cluster-center__domain-meta"')
  expect(source).toContain('class="cluster-center__actions-tree"')
  expect(source).toContain("{{ $t('clusterCenter.registeredServers') }}")
  expect(source).toContain('const backToClusterCenter = () => {')
})

it('renders supported actions through the dedicated tree component instead of a flat joined string', () => {
  const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')

  expect(source).toContain(':supported-actions="selectedDomain.supportedActions"')
  expect(source).toContain("{{ $t('clusterCenter.fields.supportedActions') }}")
  expect(source).toContain('cluster-center__meta-row')
  expect(source).not.toContain('formatSupportedActions(selectedDomain.supportedActions)')
  expect(source).not.toContain('const formatSupportedActions =')
})
```

- [ ] **Step 2: Run the ClusterCenter source test to verify it fails**

Run:

```bash
cd C:/universe/workspace/repo/b-project/b-ui/src/frontend
npx vitest run src/views/ClusterCenter.test.ts
```

Expected: FAIL because `ClusterCenter.vue` still contains `formatSupportedActions(...)` and the old `cluster-center__info-grid` layout.

- [ ] **Step 3: Integrate the component and replace the detail-card layout**

Update the selected-domain card markup and imports:

```vue
<script lang="ts" setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { push } from 'notivue'

import ClusterDomainActionTree from '@/components/ClusterDomainActionTree.vue'
import { parseClusterHubJoinUri } from '@/features/clusterHubUri'
import { i18n } from '@/locales'
import HttpUtils from '@/plugins/httputil'
import { usePingStore } from '@/store/modules/ping'
import type { ClusterDomain, ClusterMember, ClusterOperationStatus } from '@/types/clusters'
import type { MeshPairResult } from '@/types/ping'

const formatClusterVersionLabel = (version: number) => `version-${version}`
```

```vue
<v-card class="app-card-shell cluster-center__domain-info" :loading="pageLoading">
  <v-card-title>
    <div class="cluster-center__selected-head">
      <span>{{ selectedDomain.domain }}</span>
      <span class="cluster-center__version cluster-center__selected-version">
        {{ formatClusterVersionLabel(selectedDomain.lastVersion) }}
      </span>
    </div>
  </v-card-title>
  <v-card-text>
    <div class="cluster-center__detail-panel">
      <div class="cluster-center__domain-meta">
        <div class="cluster-center__meta-row">
          <span class="cluster-center__meta-label">{{ $t('clusterCenter.fields.domain') }}</span>
          <strong class="cluster-center__meta-value">{{ selectedDomain.domain }}</strong>
        </div>
        <div class="cluster-center__meta-row">
          <span class="cluster-center__meta-label">{{ $t('clusterCenter.fields.hubUrl') }}</span>
          <strong class="cluster-center__meta-value">{{ selectedDomain.hubUrl || '-' }}</strong>
        </div>
        <div class="cluster-center__meta-row">
          <span class="cluster-center__meta-label">{{ $t('clusterCenter.table.version') }}</span>
          <strong class="cluster-center__meta-value">{{ formatClusterVersionLabel(selectedDomain.lastVersion) }}</strong>
        </div>
        <div class="cluster-center__meta-row">
          <span class="cluster-center__meta-label">{{ $t('clusterCenter.fields.communicationProtocol') }}</span>
          <strong class="cluster-center__meta-value">{{ selectedDomain.communicationProtocolVersion || '-' }}</strong>
        </div>
        <div class="cluster-center__meta-row">
          <span class="cluster-center__meta-label">{{ $t('clusterCenter.fields.communicationEndpoint') }}</span>
          <strong class="cluster-center__meta-value">{{ selectedDomain.communicationEndpointPath || '-' }}</strong>
        </div>
        <div class="cluster-center__meta-row">
          <span class="cluster-center__meta-label">{{ $t('clusterCenter.mirroredMembers') }}</span>
          <strong class="cluster-center__meta-value">{{ selectedDomainMembers.length }}</strong>
        </div>
      </div>

      <div class="cluster-center__actions-tree">
        <span class="cluster-center__meta-label cluster-center__meta-label--header">
          {{ $t('clusterCenter.fields.supportedActions') }}
        </span>
        <ClusterDomainActionTree
          :supported-actions="selectedDomain.supportedActions"
        />
      </div>
    </div>
  </v-card-text>
</v-card>
```

Replace the old grid CSS with the denser split layout:

```css
.cluster-center__detail-panel {
  display: grid;
  gap: 14px;
  grid-template-columns: minmax(0, 1fr) 280px;
}

.cluster-center__domain-meta {
  display: grid;
  gap: 8px;
}

.cluster-center__meta-row {
  align-items: start;
  border-bottom: 1px solid var(--app-border-1);
  display: grid;
  gap: 10px;
  grid-template-columns: 112px minmax(0, 1fr);
  padding-bottom: 8px;
}

.cluster-center__meta-row:last-child {
  border-bottom: none;
  padding-bottom: 0;
}

.cluster-center__meta-label {
  color: var(--app-text-3);
  font-size: 12px;
  letter-spacing: 0.04em;
}

.cluster-center__meta-label--header {
  display: inline-flex;
  margin-bottom: 10px;
  text-transform: uppercase;
}

.cluster-center__meta-value {
  font-size: 14px;
  overflow-wrap: anywhere;
}

.cluster-center__actions-tree {
  background: color-mix(in srgb, var(--app-surface-2) 82%, transparent);
  border: 1px solid var(--app-border-1);
  border-radius: 16px;
  min-width: 0;
  padding: 14px 16px;
}

@media (max-width: 960px) {
  .cluster-center__detail-panel {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 640px) {
  .cluster-center__meta-row {
    grid-template-columns: 1fr;
    gap: 6px;
  }
}
```

- [ ] **Step 4: Run the focused frontend verification**

Run:

```bash
cd C:/universe/workspace/repo/b-project/b-ui/src/frontend
npx vitest run src/utils/clusterDomainActionTree.test.ts src/components/ClusterDomainActionTree.test.ts src/views/ClusterCenter.test.ts
npm run build:dist
```

Expected:

- All three Vitest files PASS
- `vue-tsc --noEmit` reports no type errors
- Vite build completes successfully

- [ ] **Step 5: Commit the integrated UI slice**

```bash
cd C:/universe/workspace/repo/b-project/b-ui
git add -- src/frontend/src/views/ClusterCenter.vue src/frontend/src/views/ClusterCenter.test.ts
git commit -m "feat(frontend): show cluster domain actions as a tree" -- src/frontend/src/views/ClusterCenter.vue src/frontend/src/views/ClusterCenter.test.ts
```
