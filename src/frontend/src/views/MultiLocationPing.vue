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

    <v-card class="app-card-shell multi-location-ping__workspace">
      <div class="multi-location-ping__tabs-bar">
        <v-tabs v-model="activeTab" color="primary" class="multi-location-ping__tabs">
          <v-tab value="inbound">去程测速 (Inbound)</v-tab>
          <v-tab value="outbound">回程测速 (Outbound)</v-tab>
          <v-tab value="mesh">域内 Mesh</v-tab>
        </v-tabs>
      </div>

      <!-- ===== INBOUND TAB ===== -->
      <section v-if="activeTab === 'inbound'" class="multi-location-ping__source-pane">
        <div class="multi-location-ping__source-header">
          <h2>Inbound Data Sources (External → Cluster)</h2>
        </div>
        <div class="multi-location-ping__source-body">
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
        </div>
        <div class="multi-location-ping__source-actions">
          <v-btn color="primary" :loading="store.loading" @click="runInbound">Start Inbound Test</v-btn>
        </div>
      </section>

      <!-- ===== OUTBOUND TAB ===== -->
      <section v-if="activeTab === 'outbound'" class="multi-location-ping__source-pane">
        <div class="multi-location-ping__source-header">
          <h2>Outbound Target Groups (Cluster → External)</h2>
        </div>
        <div class="multi-location-ping__source-body">
          <div class="multi-location-ping__catalog-actions">
            <v-btn
              size="small"
              variant="tonal"
              color="primary"
              :loading="store.loading"
              @click="refreshOutboundTargets"
            >
              Refresh Targets
            </v-btn>
            <span class="multi-location-ping__selection-count">
              {{ selectedOutboundTargetIds.length }} selected
            </span>
          </div>

          <v-alert
            v-if="outboundProviderGroups.length === 0"
            type="info"
            variant="tonal"
            density="compact"
          >
            No outbound targets are available.
          </v-alert>

          <v-expansion-panels
            v-else
            v-model="expandedProviders"
            multiple
            variant="accordion"
            density="compact"
          >
            <v-expansion-panel
              v-for="provider in outboundProviderGroups"
              :key="provider.id"
              :value="provider.id"
            >
              <v-expansion-panel-title>
                <div class="multi-location-ping__provider-title">
                  <v-checkbox-btn
                    :model-value="areAllTargetsSelected(provider.targets)"
                    :indeterminate="areSomeTargetsSelected(provider.targets)"
                    density="compact"
                    @click.stop="toggleProviderTargets(provider)"
                  />
                  <span>{{ provider.name }}</span>
                  <v-chip size="x-small" variant="tonal">{{ provider.targets.length }}</v-chip>
                </div>
              </v-expansion-panel-title>
              <v-expansion-panel-text>
                <v-expansion-panels
                  v-model="expandedTargetGroups"
                  multiple
                  variant="accordion"
                  density="compact"
                >
                  <v-expansion-panel
                    v-for="targetGroup in provider.targetGroups"
                    :key="targetGroup.key"
                    :value="targetGroup.key"
                  >
                    <v-expansion-panel-title>
                      <div class="multi-location-ping__group-title">
                        <v-checkbox-btn
                          :model-value="areAllTargetsSelected(targetGroup.targets)"
                          :indeterminate="areSomeTargetsSelected(targetGroup.targets)"
                          density="compact"
                          @click.stop="toggleTargetGroup(targetGroup)"
                        />
                        <span>{{ targetGroup.label }}</span>
                        <v-chip size="x-small" variant="tonal">{{ targetGroup.targets.length }}</v-chip>
                      </div>
                    </v-expansion-panel-title>
                    <v-expansion-panel-text>
                      <div class="multi-location-ping__target-grid">
                        <v-checkbox
                          v-for="target in targetGroup.targets"
                          :key="target.id"
                          v-model="selectedOutboundTargetIds"
                          :value="target.id"
                          density="compact"
                          hide-details
                        >
                          <template #label>
                            <span class="multi-location-ping__target-label">
                              <strong>{{ endpointLabel(target) }}</strong>
                              <span>{{ endpointLocation(target) }}</span>
                              <span>{{ endpointAddressText(target) }}</span>
                            </span>
                          </template>
                        </v-checkbox>
                      </div>
                    </v-expansion-panel-text>
                  </v-expansion-panel>
                </v-expansion-panels>
              </v-expansion-panel-text>
            </v-expansion-panel>
          </v-expansion-panels>
        </div>
        <div class="multi-location-ping__source-actions">
          <v-btn
            color="primary"
            :loading="store.loading"
            :disabled="selectedOutboundTargetIds.length === 0"
            @click="runOutbound"
          >
            Start Outbound Test
          </v-btn>
        </div>
      </section>

      <!-- ===== MESH TAB ===== -->
      <section v-if="activeTab === 'mesh'" class="multi-location-ping__source-pane">
        <div class="multi-location-ping__source-header">
          <h2>Intra-Domain Mesh Ping</h2>
        </div>
        <div class="multi-location-ping__source-body">
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
        </div>
        <div class="multi-location-ping__source-actions">
          <v-btn
            color="primary"
            :loading="store.loading"
            :disabled="!selectedDomain"
            @click="runMeshPing"
          >
            Re-run Mesh Ping
          </v-btn>
        </div>
      </section>
    </v-card>

    <!-- Results heatmap -->
    <template v-if="activeTab === 'inbound'">
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

    <template v-if="activeTab === 'outbound'">
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

    <template v-if="activeTab === 'mesh'">
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
const selectedOutboundTargetIds = ref<string[]>([])
const expandedProviders = ref<string[]>([])
const expandedTargetGroups = ref<string[]>([])

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

type OutboundTargetGroup = { key: string; label: string; targets: ExternalEndpoint[] }
type OutboundProviderGroup = {
  id: string
  name: string
  enabled: boolean
  targetGroups: OutboundTargetGroup[]
  targets: ExternalEndpoint[]
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
  await store.loadExternalTargetCatalog()
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
  const ids = selectedOutboundProviderIds.value
  if (ids.length === 0 || selectedOutboundTargetIds.value.length === 0) return
  await store.triggerExternalPing({
    direction: 'outbound',
    source_ids: ids,
    target_node_ids: selectedOutboundTargetIds.value,
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

function outboundTargetGroupLabel(target: ExternalEndpoint) {
  return target.group?.trim() || target.region?.trim() || target.city?.trim() || 'Other'
}

const outboundProviderGroups = computed<OutboundProviderGroup[]>(() => {
  const outboundSourcesById = new Map(store.outboundSources.map(source => [source.id, source]))
  return (store.externalTargetCatalog?.providers ?? [])
    .map(provider => {
      const targetGroupsByLabel = new Map<string, ExternalEndpoint[]>()
      for (const target of provider.targets ?? []) {
        const label = outboundTargetGroupLabel(target)
        const targets = targetGroupsByLabel.get(label) ?? []
        targets.push(target)
        targetGroupsByLabel.set(label, targets)
      }
      const targetGroups = [...targetGroupsByLabel.entries()]
        .map(([label, targets]) => ({
          key: `${provider.provider_id}:${label}`,
          label,
          targets: [...targets].sort((a, b) =>
            endpointLabel(a).localeCompare(endpointLabel(b)) ||
            endpointAddressText(a).localeCompare(endpointAddressText(b))
          ),
        }))
        .sort((a, b) => a.label.localeCompare(b.label))
      const targets = targetGroups.flatMap(group => group.targets)
      return {
        id: provider.provider_id,
        name: provider.provider_name,
        enabled: outboundSourcesById.get(provider.provider_id)?.enabled ?? true,
        targetGroups,
        targets,
      }
    })
    .filter(provider => provider.targets.length > 0)
    .sort((a, b) => a.name.localeCompare(b.name))
})

const selectedOutboundTargetSet = computed(() => new Set(selectedOutboundTargetIds.value))

const selectedOutboundProviderIds = computed(() =>
  outboundProviderGroups.value
    .filter(provider => provider.targets.some(target => selectedOutboundTargetSet.value.has(target.id)))
    .map(provider => provider.id)
)

function setSelectedTargets(targets: ExternalEndpoint[], selected: boolean) {
  const targetIds = new Set(targets.map(target => target.id))
  const next = new Set(selectedOutboundTargetIds.value)
  for (const id of targetIds) {
    if (selected) {
      next.add(id)
    } else {
      next.delete(id)
    }
  }
  selectedOutboundTargetIds.value = [...next]
}

function areAllTargetsSelected(targets: ExternalEndpoint[]) {
  return targets.length > 0 && targets.every(target => selectedOutboundTargetSet.value.has(target.id))
}

function areSomeTargetsSelected(targets: ExternalEndpoint[]) {
  return targets.some(target => selectedOutboundTargetSet.value.has(target.id)) && !areAllTargetsSelected(targets)
}

function toggleProviderTargets(provider: OutboundProviderGroup) {
  setSelectedTargets(provider.targets, !areAllTargetsSelected(provider.targets))
}

function toggleTargetGroup(targetGroup: OutboundTargetGroup) {
  setSelectedTargets(targetGroup.targets, !areAllTargetsSelected(targetGroup.targets))
}

async function refreshOutboundTargets() {
  await store.refreshExternalTargetCatalog([])
  await store.loadExternalTargetCatalog()
  const availableTargetIds = new Set(outboundProviderGroups.value.flatMap(provider => provider.targets.map(target => target.id)))
  selectedOutboundTargetIds.value = selectedOutboundTargetIds.value.filter(id => availableTargetIds.has(id))
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
.multi-location-ping__workspace {
  display: grid;
  gap: 0;
}

.multi-location-ping__tabs-bar {
  padding: 10px 16px 0;
}

.multi-location-ping__tabs {
  height: 48px;
  min-height: 48px;
  padding: 4px;
}

.multi-location-ping__tabs :deep(.v-slide-group__content) {
  align-items: stretch;
}

.multi-location-ping__tabs :deep(.v-tab) {
  flex: 1 1 0;
  min-height: 40px !important;
}

.multi-location-ping__source-pane {
  display: grid;
  gap: 14px;
  padding: 14px 16px 18px;
}

.multi-location-ping__source-header h2 {
  font-size: 1.2rem;
  line-height: 1.25;
  margin: 0;
}

.multi-location-ping__source-body {
  min-width: 0;
}

.multi-location-ping__source-actions {
  align-items: center;
  display: flex;
  justify-content: flex-start;
}

.multi-location-ping__catalog-actions {
  align-items: center;
  display: flex;
  gap: 12px;
  justify-content: space-between;
  margin-bottom: 12px;
}

.multi-location-ping__selection-count {
  color: color-mix(in srgb, currentColor 68%, transparent);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}

.multi-location-ping__provider-title,
.multi-location-ping__group-title {
  align-items: center;
  display: flex;
  gap: 8px;
  min-width: 0;
}

.multi-location-ping__provider-title span,
.multi-location-ping__group-title span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.multi-location-ping__target-grid {
  display: grid;
  gap: 2px 12px;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
}

.multi-location-ping__target-label {
  display: grid;
  gap: 1px;
  line-height: 1.25;
  min-width: 0;
}

.multi-location-ping__target-label span {
  color: color-mix(in srgb, currentColor 64%, transparent);
  font-size: 11px;
}

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
  .multi-location-ping__tabs-bar {
    padding-inline: 12px;
  }

  .multi-location-ping__tabs :deep(.v-tab) {
    min-width: max-content;
  }

  .multi-location-ping__source-pane {
    padding-inline: 12px;
  }

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
