<template>
  <div class="app-page">
    <section class="app-page__hero">
      <div class="app-page__hero-head">
        <div class="app-page__hero-kicker">Network</div>
        <h1 class="app-page__hero-title">Multi-Location Ping</h1>
        <p class="app-page__hero-copy">
          Latency testing from multiple geographic locations — inbound, outbound, and intra-domain mesh.
        </p>
      </div>
    </section>

    <v-tabs v-model="activeTab" color="primary" grow class="mb-4">
      <v-tab value="inbound">去程测速 (Inbound)</v-tab>
      <v-tab value="outbound">回程测速 (Outbound)</v-tab>
      <v-tab value="mesh">域内 Mesh</v-tab>
    </v-tabs>

    <!-- ===== INBOUND TAB ===== -->
    <template v-if="activeTab === 'inbound'">
      <v-card class="app-card-shell mb-4">
        <v-card-title>Inbound Data Sources (External → Cluster)</v-card-title>
        <v-card-text>
          <v-table density="compact">
            <thead>
              <tr>
                <th>Data Source</th>
                <th>Type</th>
                <th>API Key</th>
                <th>Direction</th>
                <th>Enabled</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="src in store.inboundSources" :key="src.id">
                <td>{{ src.name }}</td>
                <td>{{ src.type }}</td>
                <td>
                  <v-text-field
                    v-if="src.type === 'rest_api' || src.type === 'self_hosted'"
                    v-model="src.api_key"
                    density="compact"
                    variant="outlined"
                    hide-details
                    :type="showKeys[src.id] ? 'text' : 'password'"
                    :append-inner-icon="showKeys[src.id] ? 'mdi-eye-off' : 'mdi-eye'"
                    @click:append-inner="showKeys[src.id] = !showKeys[src.id]"
                    style="max-width: 200px"
                  />
                  <span v-else class="text-grey">—</span>
                </td>
                <td>
                  <v-chip size="small" :color="src.direction === 'inbound' ? 'primary' : 'warning'" variant="tonal">
                    {{ src.direction === 'inbound' ? '去程' : '回程' }}
                  </v-chip>
                </td>
                <td>
                  <v-switch v-model="src.enabled" color="primary" hide-details density="compact" @update:model-value="saveConfig" />
                </td>
              </tr>
            </tbody>
          </v-table>
        </v-card-text>
        <v-card-actions>
          <v-btn color="primary" :loading="store.loading" @click="runInbound">Start Inbound Test</v-btn>
        </v-card-actions>
      </v-card>

      <!-- Results heatmap -->
      <v-card v-if="inboundResults.length > 0" class="app-card-shell">
        <v-card-title>Inbound Latency Matrix (ms)</v-card-title>
        <v-card-text>
          <div class="ping-heatmap-scroll">
            <table class="ping-heatmap">
              <thead>
                <tr>
                  <th>Source ↓ / Target →</th>
                  <th v-for="col in inboundCols" :key="col">{{ col }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="row in inboundRows" :key="row">
                  <td><strong>{{ row }}</strong></td>
                  <td
                    v-for="col in inboundCols"
                    :key="col"
                    :style="cellStyle(inboundCell(row, col))"
                    class="ping-heatmap-cell"
                  >
                    {{ inboundCell(row, col)?.text ?? '-' }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </v-card-text>
      </v-card>
    </template>

    <!-- ===== OUTBOUND TAB ===== -->
    <template v-if="activeTab === 'outbound'">
      <v-card class="app-card-shell mb-4">
        <v-card-title>Outbound Target Groups (Cluster → External)</v-card-title>
        <v-card-text>
          <v-table density="compact">
            <thead>
              <tr>
                <th>Target Group</th>
                <th>Type</th>
                <th>API Key</th>
                <th>Direction</th>
                <th>Enabled</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="src in store.outboundSources" :key="src.id">
                <td>{{ src.name }}</td>
                <td>{{ src.type }}</td>
                <td><span class="text-grey">—</span></td>
                <td>
                  <v-chip size="small" color="warning" variant="tonal">回程</v-chip>
                </td>
                <td>
                  <v-switch v-model="src.enabled" color="primary" hide-details density="compact" @update:model-value="saveConfig" />
                </td>
              </tr>
            </tbody>
          </v-table>
        </v-card-text>
        <v-card-actions>
          <v-btn color="primary" :loading="store.loading" @click="runOutbound">Start Outbound Test</v-btn>
        </v-card-actions>
      </v-card>

      <v-card v-if="outboundResults.length > 0" class="app-card-shell">
        <v-card-title>Outbound Latency Matrix (ms)</v-card-title>
        <v-card-text>
          <div class="ping-heatmap-scroll">
            <table class="ping-heatmap">
              <thead>
                <tr>
                  <th>Node ↓ / Target →</th>
                  <th v-for="col in outboundCols" :key="col">{{ col }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="row in outboundRows" :key="row">
                  <td><strong>{{ row }}</strong></td>
                  <td
                    v-for="col in outboundCols"
                    :key="col"
                    :style="cellStyle(outboundCell(row, col))"
                    class="ping-heatmap-cell"
                  >
                    {{ outboundCell(row, col)?.text ?? '-' }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </v-card-text>
      </v-card>
    </template>

    <!-- ===== MESH TAB ===== -->
    <template v-if="activeTab === 'mesh'">
      <v-card class="app-card-shell mb-4">
        <v-card-title>Intra-Domain Mesh Ping</v-card-title>
        <v-card-text>
          <v-row align="center">
            <v-col cols="12" sm="6">
              <v-select
                v-model="selectedDomain"
                :items="domainOptions"
                label="Select Domain"
                density="compact"
                hide-details
                @update:model-value="onDomainSelect"
              />
            </v-col>
          </v-row>
        </v-card-text>
        <v-card-actions>
          <v-btn
            color="primary"
            :loading="store.loading"
            :disabled="!selectedDomain"
            @click="runMeshPing"
          >
            Re-run Mesh Ping
          </v-btn>
        </v-card-actions>
      </v-card>

      <v-card v-if="meshPairs.length > 0" class="app-card-shell mb-4">
        <v-card-title>Mesh Latency Matrix (ms)</v-card-title>
        <v-card-text>
          <div class="ping-heatmap-scroll">
            <table class="ping-heatmap">
              <thead>
                <tr>
                  <th>Source ↓ / Target →</th>
                  <th v-for="col in meshCols" :key="col">{{ col }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="row in meshRows" :key="row">
                  <td><strong>{{ row }}</strong></td>
                  <td
                    v-for="col in meshCols"
                    :key="col"
                    :style="cellStyle(meshCell(row, col))"
                    class="ping-heatmap-cell"
                  >
                    {{ meshCell(row, col)?.text ?? '-' }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </v-card-text>
      </v-card>

      <!-- Optimal path recommendations -->
      <v-card v-if="meshRecommendations.length > 0" class="app-card-shell">
        <v-card-title>Optimal Path Recommendations</v-card-title>
        <v-card-text>
          <v-table density="compact">
            <thead>
              <tr>
                <th>#</th>
                <th>Source</th>
                <th>Target</th>
                <th>Method</th>
                <th>Latency</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(r, i) in meshRecommendations" :key="i">
                <td>{{ i + 1 }}</td>
                <td>{{ r.source_name }}</td>
                <td>{{ r.target_name }}</td>
                <td>{{ r.method }}</td>
                <td :style="{ color: latencyColor(r.latency_ms, r.success) }">{{ latencyText(r) }}</td>
              </tr>
            </tbody>
          </v-table>
        </v-card-text>
      </v-card>
    </template>
  </div>
</template>

<script lang="ts" setup>
import { ref, computed, onMounted, reactive, watch } from 'vue'
import { usePingStore } from '@/store/modules/ping'
import type { ExternalTestResult, MeshPairResult, ExternalSource } from '@/types/ping'
import { latencyColor, latencyText, sortedByLatency } from '@/types/ping'

const store = usePingStore()
const activeTab = ref('inbound')
const selectedDomain = ref<string | null>(null)
const domainOptions = ref<{ title: string; value: string }[]>([])
const showKeys = reactive<Record<string, boolean>>({})

onMounted(async () => {
  await store.loadExternalConfig()
  try {
    const { data } = await (await import('axios')).default.get('/api/cluster/domains')
    if (data.success) {
      domainOptions.value = (data.obj ?? []).map((d: any) => ({
        title: d.domain,
        value: d.domain,
      }))
    }
  } catch { /* ignore */ }
})

async function saveConfig() {
  if (store.externalConfig) {
    await store.saveExternalConfig(store.externalConfig)
  }
}

async function runInbound() {
  const ids = store.inboundSources.filter(s => s.enabled).map(s => s.id)
  if (ids.length === 0) return
  await store.triggerExternalPing(ids)
}

async function runOutbound() {
  const ids = store.outboundSources.filter(s => s.enabled).map(s => s.id)
  if (ids.length === 0) return
  await store.triggerExternalPing(ids)
}

async function onDomainSelect(domain: string | null) {
  if (domain) {
    await store.loadMeshResult(domain)
  }
}

async function runMeshPing() {
  if (!selectedDomain.value) return
  await store.triggerMeshPing(selectedDomain.value)
}

// Heatmap data computation — inbound
const inboundResults = computed(() =>
  store.externalResults?.results.filter(r => r.direction === 'inbound') ?? []
)

const sourceLabels = (results: ExternalTestResult[]) =>
  [...new Set(results.map(r => r.source_label))].sort()

const targetLabels = (results: ExternalTestResult[]) =>
  [...new Set(results.map(r => r.target_name))].sort()

const inboundRows = computed(() => sourceLabels(inboundResults.value))
const inboundCols = computed(() => targetLabels(inboundResults.value))

function inboundCell(source: string, target: string) {
  const r = inboundResults.value.find(x => x.source_label === source && x.target_name === target)
  if (!r) return null
  return { text: latencyText(r), success: r.success, ms: r.latency_ms }
}

// Heatmap — outbound
const outboundResults = computed(() =>
  store.externalResults?.results.filter(r => r.direction === 'outbound') ?? []
)

const outboundRows = computed(() => sourceLabels(outboundResults.value))
const outboundCols = computed(() => targetLabels(outboundResults.value))

function outboundCell(source: string, target: string) {
  const r = outboundResults.value.find(x => x.source_label === source && x.target_name === target)
  if (!r) return null
  return { text: latencyText(r), success: r.success, ms: r.latency_ms }
}

// Heatmap — mesh
const meshPairs = computed(() => store.meshResult?.results ?? [])

const meshSources = computed(() =>
  [...new Set(meshPairs.value.map(r => r.source_name))].sort()
)
const meshTargets = computed(() =>
  [...new Set(meshPairs.value.map(r => r.target_name))].sort()
)

const meshRows = meshSources
const meshCols = meshTargets

function meshCell(source: string, target: string) {
  const r = meshPairs.value.find(x => x.source_name === source && x.target_name === target)
  if (!r) return null
  return { text: latencyText(r), success: r.success, ms: r.latency_ms }
}

const meshRecommendations = computed(() => sortedByLatency(meshPairs.value).slice(0, 20))

function cellStyle(cell: { success: boolean; ms: number | null } | null) {
  if (!cell || !cell.success) return { background: '#fdd', color: '#a00' }
  const ms = cell.ms ?? Infinity
  if (ms < 50) return { background: '#d4edda', color: '#155724' }
  if (ms < 150) return { background: '#fff3cd', color: '#856404' }
  if (ms < 300) return { background: '#ffe5b4', color: '#b45309' }
  return { background: '#f8d7da', color: '#721c24' }
}
</script>

<style scoped>
.ping-heatmap-scroll {
  overflow-x: auto;
}
.ping-heatmap {
  border-collapse: collapse;
  width: 100%;
}
.ping-heatmap th,
.ping-heatmap td {
  border: 1px solid var(--v-border-color, #ddd);
  padding: 6px 10px;
  text-align: center;
  white-space: nowrap;
  font-size: 13px;
}
.ping-heatmap th {
  background: var(--v-theme-surface-variant, #f5f5f5);
  font-weight: 600;
}
.ping-heatmap-cell {
  font-variant-numeric: tabular-nums;
  min-width: 60px;
}
</style>
