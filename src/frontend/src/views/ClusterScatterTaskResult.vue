<template>
  <div class="app-page">
    <v-overlay :model-value="pageLoading" class="align-center justify-center" persistent>
      <v-progress-circular indeterminate size="64" />
    </v-overlay>

    <v-alert v-if="pageError" type="error" variant="tonal" closable class="scatter-result__error">
      {{ pageError }}
    </v-alert>

    <template v-if="!pageLoading && !pageError">
      <section class="app-page__hero">
        <div class="app-page__hero-head">
          <div class="app-page__hero-kicker">Scatter-Gather</div>
          <h1 class="app-page__hero-title">{{ $t('clusterCenter.scatterTaskResult.title') }}</h1>
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
          <div class="app-page__toolbar-actions">
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
            <div v-if="latencyData.nodeIds.length === 0" class="cluster-center__empty">
              {{ $t('clusterCenter.scatterTaskResult.matrixEmpty') }}
            </div>
            <div v-else class="scatter-result__table-wrap">
              <table class="scatter-result__matrix-table">
                <thead>
                  <tr>
                    <th>{{ $t('clusterCenter.scatterTaskResult.source') }}\{{ $t('clusterCenter.scatterTaskResult.target') }}</th>
                    <th v-for="nodeId in latencyData.nodeIds" :key="nodeId">{{ nodeId }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="sourceId in latencyData.nodeIds" :key="sourceId">
                    <td class="scatter-result__source-cell">{{ sourceId }}</td>
                    <td
                      v-for="targetId in latencyData.nodeIds"
                      :key="targetId"
                      :style="matrixCellStyle(sourceId, targetId)"
                      class="scatter-result__matrix-cell"
                    >
                      {{ matrixCellValue(sourceId, targetId) }}
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
                    <td>
                      <span class="scatter-result__node-id">{{ summary.nodeId }}</span>
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
import { useClusterStore } from '@/store/modules/cluster'
import type { ScatterTaskResultDetail } from '@/types/clusters'

const route = useRoute()
const router = useRouter()
const clusterStore = useClusterStore()

const pageLoading = ref(true)
const pageError = ref<string | null>(null)
const result = ref<ScatterTaskResultDetail | null>(null)

interface LatencyDisplayData {
  nodeIds: string[]
  matrix: Record<string, Record<string, number | null>>
  summaries: Array<{
    nodeId: string
    avgMs: number | null
    reachable: number
    total: number
  }>
}

const latencyData = computed<LatencyDisplayData | null>(() => {
  if (!result.value || result.value.taskType !== 'mesh.latency') return null
  const data = result.value.result
  if (!data) return null

  const matrix = (data.matrix || data.Matrix || {}) as Record<string, Record<string, number | null>>
  const summariesRaw = (data.summaries || data.Summaries || []) as Array<{
    node_id?: string
    nodeId?: string
    avg_ms?: number
    avgMs?: number
    reachable?: number
    total?: number
  }>

  const nodeIds = Object.keys(matrix).sort()

  const summaries = summariesRaw.map(s => ({
    nodeId: s.node_id || s.nodeId || '',
    avgMs: s.avg_ms ?? s.avgMs ?? null,
    reachable: s.reachable ?? 0,
    total: s.total ?? 0,
  }))

  return { nodeIds, matrix, summaries }
})

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
    const data = await clusterStore.fetchScatterTaskResult(domainId, taskId)
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
  border-radius: 18px;
  overflow: hidden;
  overflow-x: auto;
}

.scatter-result__matrix-table {
  border-collapse: collapse;
  width: 100%;
}

.scatter-result__matrix-table th,
.scatter-result__matrix-table td {
  border-bottom: 1px solid var(--app-border-1);
  border-right: 1px solid var(--app-border-1);
  font-size: 12px;
  padding: 8px 10px;
  text-align: center;
  white-space: nowrap;
}

.scatter-result__matrix-table th {
  color: var(--app-text-3);
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
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
}

.scatter-result__node-id {
  font-family: var(--app-font-mono, ui-monospace, monospace);
  font-size: 12px;
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
