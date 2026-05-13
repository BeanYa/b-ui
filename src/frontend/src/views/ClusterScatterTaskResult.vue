<template>
  <div class="app-page">
    <v-overlay :model-value="pageLoading" class="align-center justify-center" persistent>
      <v-progress-circular indeterminate size="64" />
    </v-overlay>

    <v-alert v-if="pageError" type="error" variant="tonal" closable class="scatter-result__error">
      {{ pageError }}
    </v-alert>

    <template v-if="!pageLoading && !pageError">
      <section class="app-page__hero scatter-result__hero">
        <div class="app-page__hero-head">
          <div class="app-page__hero-kicker">Scatter-Gather</div>
          <h1 class="app-page__hero-title scatter-result__title">
            {{ $t('clusterCenter.scatterTaskResult.title') }}
          </h1>
          <p class="app-page__hero-copy">
            {{ result?.taskType }}
          </p>
          <div class="app-page__hero-meta">
            <span class="app-page__hero-meta-item">{{ result?.status }}</span>
            <span class="app-page__hero-meta-item">{{ result?.taskId }}</span>
          </div>
        </div>
      </section>

      <v-row class="app-page__toolbar">
        <v-col cols="12">
          <div class="app-page__toolbar-actions app-toolbar-cluster">
            <v-btn variant="outlined" prepend-icon="mdi-arrow-left" @click="goBack">
              {{ $t('clusterCenter.scatterTaskResult.backToCluster') }}
            </v-btn>
            <v-btn
              v-if="result"
              variant="outlined"
              prepend-icon="mdi-download"
              @click="downloadJson"
            >
              {{ $t('clusterCenter.scatterTaskResult.downloadJson') }}
            </v-btn>
          </div>
        </v-col>
      </v-row>

      <v-card v-if="result" class="app-card-shell scatter-result__meta-card">
        <v-card-text>
          <div class="scatter-result__info-grid">
            <div class="scatter-result__info-row">
              <span class="scatter-result__info-label">{{ $t('clusterCenter.scatterTaskResult.taskId') }}</span>
              <strong class="scatter-result__info-value scatter-result__info-value--mono">{{ result.taskId }}</strong>
            </div>
            <div class="scatter-result__info-row">
              <span class="scatter-result__info-label">{{ $t('clusterCenter.scatterTaskResult.taskType') }}</span>
              <strong class="scatter-result__info-value">{{ result.taskType }}</strong>
            </div>
            <div class="scatter-result__info-row">
              <span class="scatter-result__info-label">{{ $t('clusterCenter.scatterTaskResult.status') }}</span>
              <strong class="scatter-result__info-value">
                <v-chip size="small" variant="flat" :color="statusColor(result.status)">
                  {{ result.status }}
                </v-chip>
              </strong>
            </div>
            <div v-if="result.generatedAt" class="scatter-result__info-row">
              <span class="scatter-result__info-label">{{ $t('clusterCenter.scatterTaskResult.generatedAt') }}</span>
              <strong class="scatter-result__info-value">{{ result.generatedAt }}</strong>
            </div>
          </div>
        </v-card-text>
      </v-card>

      <template v-if="result && result.taskType === 'mesh.latency' && latencyData">
        <v-card class="app-card-shell scatter-result__matrix-card">
          <v-card-title>{{ $t('clusterCenter.scatterTaskResult.latencyMatrix') }}</v-card-title>
          <v-card-text>
            <div v-if="latencyData.nodes.length === 0" class="cluster-center__empty">
              {{ $t('clusterCenter.scatterTaskResult.matrixEmpty') }}
            </div>
            <div v-else class="scatter-result__table-wrap">
              <table class="scatter-result__matrix-table">
                <thead>
                  <tr>
                    <th>{{ $t('clusterCenter.scatterTaskResult.source') }}\{{ $t('clusterCenter.scatterTaskResult.target') }}</th>
                    <th v-for="node in latencyData.nodes" :key="node.nodeId" :title="node.nodeId">
                      <span class="scatter-result__node-name">{{ node.displayName }}</span>
                      <span class="scatter-result__node-meta">{{ node.nodeIdShort }}</span>
                    </th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="sourceNode in latencyData.nodes" :key="sourceNode.nodeId">
                    <td class="scatter-result__source-cell" :title="sourceNode.nodeId">
                      <span class="scatter-result__node-name">{{ sourceNode.displayName }}</span>
                      <span class="scatter-result__node-meta">{{ sourceNode.nodeIdShort }}</span>
                    </td>
                    <td
                      v-for="targetNode in latencyData.nodes"
                      :key="targetNode.nodeId"
                      :style="matrixCellStyle(sourceNode.nodeId, targetNode.nodeId)"
                      class="scatter-result__matrix-cell"
                    >
                      {{ matrixCellValue(sourceNode.nodeId, targetNode.nodeId) }}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </v-card-text>
        </v-card>

        <v-card class="app-card-shell scatter-result__summary-card">
          <v-card-title>{{ $t('clusterCenter.scatterTaskResult.nodeSummary') }}</v-card-title>
          <v-card-text>
            <div class="scatter-result__table-wrap">
              <table class="cluster-center__member-table">
                <thead>
                  <tr>
                    <th>{{ $t('clusterCenter.scatterTaskResult.nodeId') }}</th>
                    <th>{{ $t('clusterCenter.scatterTaskResult.avgLatency') }}</th>
                    <th>{{ $t('clusterCenter.scatterTaskResult.reachable') }}</th>
                    <th>{{ $t('clusterCenter.scatterTaskResult.total') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="summary in latencyData.summaries" :key="summary.nodeId">
                    <td :title="summary.nodeId">
                      <span class="scatter-result__node-name">{{ summary.displayName }}</span>
                      <span class="scatter-result__node-meta scatter-result__node-id">{{ summary.nodeIdShort }}</span>
                    </td>
                    <td :style="latencyStyle(summary.avgMs)">
                      {{ summary.avgMs != null ? summary.avgMs.toFixed(1) + ' ms' : '-' }}
                    </td>
                    <td>{{ summary.reachable }}</td>
                    <td>{{ summary.total }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </v-card-text>
        </v-card>
      </template>

      <template v-else-if="result && result.taskType !== 'mesh.latency'">
        <v-card class="app-card-shell scatter-result__raw-card">
          <v-card-title>Result</v-card-title>
          <v-card-text>
            <pre class="scatter-result__raw-json">{{ JSON.stringify(result.result, null, 2) }}</pre>
          </v-card-text>
        </v-card>
      </template>
    </template>
  </div>
</template>

<script lang="ts" setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import api from '@/plugins/api'
import { useClusterStore } from '@/store/modules/cluster'
import type { ClusterMember, ScatterTaskResultDetail } from '@/types/clusters'

const route = useRoute()
const router = useRouter()
const clusterStore = useClusterStore()

const pageLoading = ref(true)
const pageError = ref<string | null>(null)
const result = ref<ScatterTaskResultDetail | null>(null)
const displayNameByNodeId = ref<Record<string, string>>({})

interface LatencyNodeDisplay {
  nodeId: string
  displayName: string
  nodeIdShort: string
}

interface LatencyDisplayData {
  nodes: LatencyNodeDisplay[]
  matrix: Record<string, Record<string, number | null>>
  summaries: Array<{
    nodeId: string
    displayName: string
    nodeIdShort: string
    avgMs: number | null
    reachable: number
    total: number
  }>
}

const latencyData = computed<LatencyDisplayData | null>(() => {
  if (!result.value || result.value.taskType !== 'mesh.latency') return null
  const data = result.value.result
  if (!data) return null

  const matrix = normalizeLatencyMatrix(data.matrix || data.Matrix || {})
  const summariesRaw = data.summaries || data.Summaries || data.node_summary || data.NodeSummary || {}
  const summariesInput = normalizeLatencySummaries(summariesRaw)
  const nodeIds = collectLatencyNodeIds(matrix, summariesInput).sort((a, b) =>
    resolveNodeDisplayName(a).localeCompare(resolveNodeDisplayName(b), undefined, { numeric: true, sensitivity: 'base' }),
  )
  const nodes = nodeIds.map(buildNodeDisplay)

  const summaries = nodeIds.map(nodeId => {
    const summary = summariesInput.get(nodeId)
    const node = buildNodeDisplay(nodeId)

    return {
      nodeId,
      displayName: node.displayName,
      nodeIdShort: node.nodeIdShort,
      avgMs: summary?.avgMs ?? null,
      reachable: summary?.reachable ?? 0,
      total: summary?.total ?? 0,
    }
  })

  return { nodes, matrix, summaries }
})

function normalizeLatencyMatrix(raw: unknown): Record<string, Record<string, number | null>> {
  if (!raw || typeof raw !== 'object') return {}

  const matrix: Record<string, Record<string, number | null>> = {}
  for (const [sourceId, row] of Object.entries(raw as Record<string, unknown>)) {
    if (!row || typeof row !== 'object') continue
    matrix[sourceId] = {}

    for (const [targetId, value] of Object.entries(row as Record<string, unknown>)) {
      matrix[sourceId][targetId] = typeof value === 'number' ? value : null
    }
  }

  return matrix
}

function normalizeLatencySummaries(raw: unknown): Map<string, {
  avgMs: number | null
  reachable: number
  total: number
}> {
  const summaries = new Map<string, {
    avgMs: number | null
    reachable: number
    total: number
  }>()

  const addSummary = (nodeId: string, value: {
    node_id?: string
    nodeId?: string
    avg_ms?: number
    avgMs?: number
    reachable?: number
    total?: number
  }) => {
    const resolvedNodeId = value.node_id || value.nodeId || nodeId
    if (!resolvedNodeId) return
    const reachable = value.reachable ?? 0
    const total = value.total ?? 0

    summaries.set(resolvedNodeId, {
      avgMs: reachable > 0 ? value.avg_ms ?? value.avgMs ?? null : null,
      reachable,
      total,
    })
  }

  if (Array.isArray(raw)) {
    for (const item of raw) {
      if (item && typeof item === 'object') {
        addSummary('', item as Parameters<typeof addSummary>[1])
      }
    }
    return summaries
  }

  if (raw && typeof raw === 'object') {
    for (const [nodeId, value] of Object.entries(raw as Record<string, unknown>)) {
      if (value && typeof value === 'object') {
        addSummary(nodeId, value as Parameters<typeof addSummary>[1])
      }
    }
  }

  return summaries
}

function collectLatencyNodeIds(
  matrix: Record<string, Record<string, number | null>>,
  summaries: Map<string, unknown>,
): string[] {
  const ids = new Set<string>()

  for (const [sourceId, row] of Object.entries(matrix)) {
    ids.add(sourceId)
    for (const targetId of Object.keys(row)) {
      ids.add(targetId)
    }
  }

  for (const nodeId of summaries.keys()) {
    ids.add(nodeId)
  }

  return [...ids]
}

function buildNodeDisplay(nodeId: string): LatencyNodeDisplay {
  return {
    nodeId,
    displayName: resolveNodeDisplayName(nodeId),
    nodeIdShort: shortenNodeId(nodeId),
  }
}

function resolveNodeDisplayName(nodeId: string): string {
  return displayNameByNodeId.value[nodeId] || nodeId
}

function shortenNodeId(nodeId: string): string {
  if (nodeId.length <= 12) return nodeId
  return `${nodeId.slice(0, 8)}...${nodeId.slice(-4)}`
}

function matrixCellValue(sourceId: string, targetId: string): string {
  const val = latencyData.value?.matrix[sourceId]?.[targetId]
  if (val == null) return '-'
  return val.toFixed(1) + ' ms'
}

function matrixCellStyle(sourceId: string, targetId: string): Record<string, string> {
  const val = latencyData.value?.matrix[sourceId]?.[targetId]
  if (val == null) return { color: 'var(--app-text-3)' }
  if (sourceId === targetId) return { color: 'var(--app-text-3)', fontStyle: 'italic' }
  if (val < 10) return { color: '#155724', fontWeight: '600' }
  if (val < 50) return { color: '#0f7044', fontWeight: '600' }
  if (val < 150) return { color: '#856404', fontWeight: '600' }
  if (val < 300) return { color: '#b45309', fontWeight: '600' }
  return { color: '#721c24', fontWeight: '600' }
}

function latencyStyle(avgMs: number | null): Record<string, string> {
  if (avgMs == null) return { color: 'var(--app-text-3)' }
  if (avgMs < 50) return { color: '#155724', fontWeight: '600' }
  if (avgMs < 150) return { color: '#856404', fontWeight: '600' }
  if (avgMs < 300) return { color: '#b45309', fontWeight: '600' }
  return { color: '#721c24', fontWeight: '600' }
}

function statusColor(status: string): string {
  switch (status) {
    case 'completed': return 'green'
    case 'failed': return 'red'
    case 'timeout': return 'orange'
    default: return 'grey'
  }
}

function goBack() {
  router.push({ name: 'pages.clusterCenter' })
}

function downloadJson() {
  if (!result.value) return
  const json = JSON.stringify(result.value, null, 2)
  const blob = new Blob([json], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `scatter-${result.value.taskId}.json`
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}

async function loadNodeDisplayNames(domainId: string) {
  displayNameByNodeId.value = {}
  const localDomainId = Number(domainId)
  if (!Number.isFinite(localDomainId)) return

  try {
    const { data } = await api.get('api/cluster/members')
    const members = Array.isArray(data?.obj) ? data.obj as ClusterMember[] : []
    displayNameByNodeId.value = Object.fromEntries(
      members
        .filter(member => member.domainId === localDomainId)
        .map(member => [
          member.nodeId,
          (member.displayName || member.name || member.nodeId).trim(),
        ]),
    )
  } catch {
    displayNameByNodeId.value = {}
  }
}

onMounted(async () => {
  pageLoading.value = true
  pageError.value = null
  try {
    const domainId = route.params.domainId as string
    const taskId = route.params.taskId as string
    if (!domainId || !taskId) {
      pageError.value = 'Missing domain or task ID'
      return
    }
    const [data] = await Promise.all([
      clusterStore.fetchScatterTaskResult(domainId, taskId),
      loadNodeDisplayNames(domainId),
    ])
    if (!data) {
      pageError.value = 'Task result not found'
      return
    }
    result.value = data
  } catch (e: any) {
    pageError.value = e.message || 'Failed to load task result'
  } finally {
    pageLoading.value = false
  }
})
</script>

<style scoped>
.scatter-result__error {
  margin-bottom: 16px;
}

.scatter-result__hero {
  padding-block: clamp(26px, 4vw, 44px);
}

.scatter-result__title {
  font-size: clamp(34px, 4.2vw, 58px);
  letter-spacing: 0;
  line-height: 0.98;
  max-width: 780px;
}

.scatter-result__meta-card,
.scatter-result__matrix-card,
.scatter-result__summary-card,
.scatter-result__raw-card {
  margin-bottom: 16px;
}

.scatter-result__info-grid {
  display: grid;
  gap: 8px;
}

.scatter-result__info-row {
  align-items: start;
  border-bottom: 1px solid var(--app-border-1);
  display: grid;
  gap: 10px;
  grid-template-columns: 112px minmax(0, 1fr);
  padding-bottom: 8px;
}

.scatter-result__info-row:last-child {
  border-bottom: none;
  padding-bottom: 0;
}

.scatter-result__info-label {
  color: var(--app-text-3);
  font-size: 12px;
  letter-spacing: 0.04em;
}

.scatter-result__info-value {
  font-size: 14px;
  overflow-wrap: anywhere;
}

.scatter-result__info-value--mono {
  font-family: var(--app-font-mono, ui-monospace, monospace);
  font-size: 13px;
}

.scatter-result__table-wrap {
  border: 1px solid var(--app-border-1);
  border-radius: 14px;
  overflow: hidden;
  overflow-x: auto;
  scrollbar-color: color-mix(in srgb, var(--app-text-3) 40%, transparent) transparent;
}

.scatter-result__matrix-table {
  border-collapse: collapse;
  min-width: 100%;
  table-layout: fixed;
  width: max-content;
}

.scatter-result__matrix-table th,
.scatter-result__matrix-table td {
  border-bottom: 1px solid var(--app-border-1);
  border-right: 1px solid var(--app-border-1);
  font-size: 11px;
  line-height: 1.25;
  min-width: 108px;
  max-width: 132px;
  padding: 9px 10px;
  text-align: center;
  vertical-align: middle;
}

.scatter-result__matrix-table th {
  background: color-mix(in srgb, var(--app-surface-2) 82%, transparent);
  color: var(--app-text-2);
  font-weight: 700;
}

.scatter-result__matrix-table th:first-child,
.scatter-result__source-cell {
  background: color-mix(in srgb, var(--app-surface-2) 88%, transparent);
  left: 0;
  min-width: 150px;
  max-width: 180px;
  position: sticky;
  z-index: 2;
}

.scatter-result__matrix-table th:first-child {
  z-index: 3;
}

.scatter-result__matrix-table tbody tr:nth-child(odd) td:nth-child(even),
.scatter-result__matrix-table tbody tr:nth-child(even) td:nth-child(odd) {
  background: color-mix(in srgb, var(--app-state-info) 7%, transparent);
}

.scatter-result__matrix-table tbody tr:nth-child(odd) td:nth-child(odd):not(:first-child),
.scatter-result__matrix-table tbody tr:nth-child(even) td:nth-child(even) {
  background: color-mix(in srgb, var(--app-surface-1) 82%, transparent);
}

.scatter-result__matrix-table tbody tr:hover td {
  background: color-mix(in srgb, var(--app-state-info) 12%, transparent);
}

.scatter-result__matrix-table tbody tr:last-child td {
  border-bottom: none;
}

.scatter-result__source-cell {
  font-weight: 600;
  text-align: left !important;
}

.scatter-result__matrix-cell {
  font-family: var(--app-font-mono, ui-monospace, monospace);
  font-size: 11px !important;
}

.scatter-result__node-name,
.scatter-result__node-meta {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.scatter-result__node-name {
  color: var(--app-text-1);
  font-size: 12px;
  font-weight: 700;
}

.scatter-result__node-meta {
  color: var(--app-text-3);
  font-family: var(--app-font-mono, ui-monospace, monospace);
  font-size: 10px;
  font-weight: 500;
  margin-top: 3px;
}

.scatter-result__node-id {
  font-family: var(--app-font-mono, ui-monospace, monospace);
}

.scatter-result__raw-json {
  background: color-mix(in srgb, var(--app-surface-1) 90%, transparent);
  border: 1px solid var(--app-border-1);
  border-radius: 14px;
  font-family: var(--app-font-mono, ui-monospace, monospace);
  font-size: 12px;
  line-height: 1.6;
  max-height: 600px;
  overflow: auto;
  padding: 16px;
}

@media (max-width: 640px) {
  .scatter-result__info-row {
    gap: 6px;
    grid-template-columns: 1fr;
  }
}
</style>
