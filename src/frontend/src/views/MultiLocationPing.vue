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
          <div class="multi-location-ping__target-controls">
            <v-text-field
              v-model="inboundTargetHost"
              label="Target host"
              density="compact"
              variant="outlined"
              hide-details
              class="multi-location-ping__target-host"
            />
            <v-text-field
              v-model="inboundTargetPort"
              label="Port"
              density="compact"
              variant="outlined"
              hide-details
              type="number"
              min="1"
              max="65535"
              step="1"
              class="multi-location-ping__target-port"
            />
          </div>

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
                  <th v-for="col in inboundCols" :key="col.key" class="ping-heatmap-axis">
                    <span class="ping-heatmap-axis__label">{{ col.label }}</span>
                    <span v-if="col.location" class="ping-heatmap-axis__meta">{{ col.location }}</span>
                    <span v-if="col.address" class="ping-heatmap-axis__address">{{ col.address }}</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="row in inboundRows" :key="row.key">
                  <td class="ping-heatmap-row-heading">
                    <strong>{{ row.label }}</strong>
                    <span v-if="row.location">{{ row.location }}</span>
                    <span v-if="row.address">{{ row.address }}</span>
                  </td>
                  <td
                    v-for="col in inboundCols"
                    :key="col.key"
                    :style="cellStyle(inboundCell(row.key, col.key))"
                    class="ping-heatmap-cell"
                    :title="inboundCell(row.key, col.key)?.title"
                  >
                    <span class="ping-heatmap-cell__value">{{ inboundCell(row.key, col.key)?.text ?? '-' }}</span>
                    <span v-if="inboundCell(row.key, col.key)?.method" class="ping-heatmap-cell__method">
                      {{ inboundCell(row.key, col.key)?.method }}
                    </span>
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
                  <th v-for="col in outboundCols" :key="col.key" class="ping-heatmap-axis">
                    <span class="ping-heatmap-axis__label">{{ col.label }}</span>
                    <span v-if="col.location" class="ping-heatmap-axis__meta">{{ col.location }}</span>
                    <span v-if="col.address" class="ping-heatmap-axis__address">{{ col.address }}</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="row in outboundRows" :key="row.key">
                  <td class="ping-heatmap-row-heading">
                    <strong>{{ row.label }}</strong>
                    <span v-if="row.location">{{ row.location }}</span>
                    <span v-if="row.address">{{ row.address }}</span>
                  </td>
                  <td
                    v-for="col in outboundCols"
                    :key="col.key"
                    :style="cellStyle(outboundCell(row.key, col.key))"
                    class="ping-heatmap-cell"
                    :title="outboundCell(row.key, col.key)?.title"
                  >
                    <span class="ping-heatmap-cell__value">{{ outboundCell(row.key, col.key)?.text ?? '-' }}</span>
                    <span v-if="outboundCell(row.key, col.key)?.method" class="ping-heatmap-cell__method">
                      {{ outboundCell(row.key, col.key)?.method }}
                    </span>
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
import { ref, computed, onMounted, reactive } from 'vue'
import { usePingStore } from '@/store/modules/ping'
import type { ExternalEndpoint, ExternalTestResult } from '@/types/ping'
import { latencyColor, latencyText, sortedByLatency } from '@/types/ping'

const store = usePingStore()
const activeTab = ref('inbound')
const selectedDomain = ref<string | null>(null)
const domainOptions = ref<{ title: string; value: string }[]>([])
const showKeys = reactive<Record<string, boolean>>({})
const inboundTargetHost = ref(defaultInboundTargetHost())
const inboundTargetPort = ref(defaultInboundTargetPort())

type EndpointAxis = {
  key: string
  label: string
  location: string
  address: string
}

type ExternalMatrixCell = {
  text: string
  success: boolean
  ms: number | null
  method: string | null
  title: string
}

function defaultInboundTargetHost() {
  const hostname = normalizeInboundTargetHost(globalThis.window?.location?.hostname ?? '')
  const normalized = hostname.toLowerCase()
  if (normalized === 'localhost' || normalized === '127.0.0.1' || normalized === '::1' || normalized === '[::1]') {
    return ''
  }
  return hostname
}

function defaultInboundTargetPort() {
  const location = globalThis.window?.location
  if (!location) return ''
  if (location.port) return location.port
  return location.protocol === 'https:' ? '443' : '80'
}

function normalizeInboundTargetHost(value: string) {
  const host = value.trim()
  if (!host) return ''
  if (host.startsWith('[') && host.endsWith(']')) {
    return host.slice(1, -1).trim()
  }
  return host
}

function normalizedInboundTargetPort(value: string) {
  const port = Number(value)
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    return undefined
  }
  return port
}

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
  const host = normalizeInboundTargetHost(inboundTargetHost.value)
  if (ids.length === 0 || !host) return
  const port = normalizedInboundTargetPort(inboundTargetPort.value)
  await store.triggerExternalPing({
    direction: 'inbound',
    source_ids: ids,
    target: {
      host,
      ...(port !== undefined ? { port } : {}),
      label: host,
    },
  })
}

async function runOutbound() {
  const ids = store.outboundSources.filter(s => s.enabled).map(s => s.id)
  if (ids.length === 0) return
  await store.triggerExternalPing({
    direction: 'outbound',
    source_ids: ids,
    methods: ['tcp', 'http', 'icmp'],
  })
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

function endpointLabel(endpoint: ExternalEndpoint | null | undefined, fallback = 'Unknown') {
  return endpoint?.label?.trim() || endpoint?.id?.trim() || fallback
}

function endpointLocation(endpoint: ExternalEndpoint | null | undefined) {
  const place = [endpoint?.city, endpoint?.region, endpoint?.country]
    .map(part => part?.trim())
    .filter(Boolean)
    .join(', ')
  const network = endpoint?.network?.trim() || endpoint?.provider?.trim() || ''
  return [place, network].filter(Boolean).join(' | ')
}

function endpointAddressText(endpoint: ExternalEndpoint | null | undefined) {
  const host = endpoint?.host?.trim()
  if (!host) return ''
  const port = endpoint?.port
  if (typeof port === 'number' && Number.isFinite(port) && port > 0) {
    return `${host}:${port}`
  }
  return host
}

function endpointKey(endpoint: ExternalEndpoint | null | undefined, fallback: string) {
  return [
    endpoint?.id?.trim(),
    endpointLabel(endpoint, fallback),
    endpointLocation(endpoint),
    endpointAddressText(endpoint),
  ].filter(Boolean).join('|')
}

function endpointAxis(endpoint: ExternalEndpoint | null | undefined, fallback: string): EndpointAxis {
  return {
    key: endpointKey(endpoint, fallback),
    label: endpointLabel(endpoint, fallback),
    location: endpointLocation(endpoint),
    address: endpointAddressText(endpoint),
  }
}

function externalAxes(
  results: ExternalTestResult[],
  endpoint: (result: ExternalTestResult) => ExternalEndpoint,
  fallback: (result: ExternalTestResult) => string,
) {
  const axes = new Map<string, EndpointAxis>()
  for (const result of results) {
    const axis = endpointAxis(endpoint(result), fallback(result))
    if (!axes.has(axis.key)) axes.set(axis.key, axis)
  }
  return [...axes.values()].sort((a, b) =>
    a.label.localeCompare(b.label) || a.location.localeCompare(b.location) || a.address.localeCompare(b.address)
  )
}

function endpointSummary(axis: EndpointAxis) {
  return [axis.label, axis.location, axis.address].filter(Boolean).join(' | ')
}

function externalCell(results: ExternalTestResult[], sourceKey: string, targetKey: string): ExternalMatrixCell | null {
  const matches = results.filter(result =>
    endpointKey(result.source, result.source_label) === sourceKey &&
    endpointKey(result.target, result.target_name) === targetKey
  )
  if (matches.length === 0) return null
  const r = [...matches].sort((a, b) => {
    if (a.success !== b.success) return a.success ? -1 : 1
    return (a.latency_ms ?? Infinity) - (b.latency_ms ?? Infinity)
  })[0]
  const source = endpointAxis(r.source, r.source_label)
  const target = endpointAxis(r.target, r.target_name)
  return {
    text: latencyText(r),
    success: r.success,
    ms: r.latency_ms,
    method: r.method,
    title: `${endpointSummary(source)} -> ${endpointSummary(target)}`,
  }
}

const inboundRows = computed(() => externalAxes(inboundResults.value, r => r.source, r => r.source_label))
const inboundCols = computed(() => externalAxes(inboundResults.value, r => r.target, r => r.target_name))

function inboundCell(source: string, target: string) {
  return externalCell(inboundResults.value, source, target)
}

// Heatmap — outbound
const outboundResults = computed(() =>
  store.externalResults?.results.filter(r => r.direction === 'outbound') ?? []
)

const outboundRows = computed(() => externalAxes(outboundResults.value, r => r.source, r => r.source_label))
const outboundCols = computed(() => externalAxes(outboundResults.value, r => r.target, r => r.target_name))

function outboundCell(source: string, target: string) {
  return externalCell(outboundResults.value, source, target)
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
.multi-location-ping__target-controls {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 14px;
}
.multi-location-ping__target-host {
  flex: 1 1 280px;
}
.multi-location-ping__target-port {
  flex: 0 0 120px;
}
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
  min-width: 72px;
}
.ping-heatmap-axis,
.ping-heatmap-row-heading {
  min-width: 150px;
  text-align: left;
  vertical-align: top;
}
.ping-heatmap-axis span,
.ping-heatmap-row-heading span {
  display: block;
}
.ping-heatmap-axis__label,
.ping-heatmap-row-heading strong {
  font-weight: 650;
}
.ping-heatmap-axis__meta,
.ping-heatmap-axis__address,
.ping-heatmap-row-heading span {
  color: color-mix(in srgb, currentColor 64%, transparent);
  font-size: 11px;
  line-height: 1.35;
}
.ping-heatmap-cell__value,
.ping-heatmap-cell__method {
  display: block;
}
.ping-heatmap-cell__method {
  color: color-mix(in srgb, currentColor 68%, transparent);
  font-size: 10px;
  line-height: 1.2;
  text-transform: uppercase;
}
@media (max-width: 640px) {
  .multi-location-ping__target-controls {
    align-items: stretch;
    flex-direction: column;
  }
  .multi-location-ping__target-host,
  .multi-location-ping__target-port {
    flex: 1 1 auto;
  }
}
</style>
