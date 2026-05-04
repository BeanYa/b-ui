<template>
  <div class="app-page">
    <v-overlay :model-value="isPageLoading" class="align-center justify-center" persistent>
      <v-progress-circular indeterminate size="64" />
    </v-overlay>

    <v-alert v-if="remoteNode.pageError" type="error" variant="tonal" closable class="node-detail__error">
      {{ remoteNode.pageError }}
    </v-alert>

    <template v-if="!isPageLoading && !remoteNode.pageError">
      <section class="app-page__hero">
        <div class="app-page__hero-head">
          <div class="app-page__hero-kicker">Cluster Node</div>
          <h1 class="app-page__hero-title">{{ nodeName }}</h1>
          <p class="app-page__hero-copy">
            Remote node detail view with the same management surface as the local panel when supported by the node.
          </p>
          <div class="app-page__hero-meta">
            <span class="app-page__hero-meta-item">{{ nodeActions.length }} actions</span>
            <span class="app-page__hero-meta-item">{{ remoteNode.baseURL }}</span>
          </div>
        </div>
      </section>

      <v-row class="app-page__toolbar">
        <v-col cols="12">
          <div class="app-page__toolbar-actions">
            <v-btn variant="outlined" prepend-icon="mdi-arrow-left" @click="goBack">
              {{ $t('clusterCenter.actions.back') }}
            </v-btn>
          </div>
        </v-col>
      </v-row>

      <v-card v-if="nodeConnection" class="app-card-shell node-detail__info-card">
        <v-card-text>
          <div class="node-detail__info-grid">
            <div class="node-detail__info-row">
              <span class="node-detail__info-label">Node ID</span>
              <strong class="node-detail__info-value node-detail__info-value--mono">{{ nodeConnection.nodeId }}</strong>
            </div>
            <div class="node-detail__info-row">
              <span class="node-detail__info-label">Panel URL</span>
              <strong class="node-detail__info-value">{{ nodeConnection.baseUrl }}</strong>
            </div>
            <div v-if="nodeMember" class="node-detail__info-row">
              <span class="node-detail__info-label">Status</span>
              <strong class="node-detail__info-value">
                <v-chip
                  size="small"
                  variant="flat"
                  :color="nodeMember.status === 'updating' ? 'orange' : nodeMember.status === 'offline' ? 'red' : 'green'"
                >
                  {{ nodeMember.status === 'updating' ? $t('clusterCenter.statuses.updating') : nodeMember.status === 'offline' ? $t('offline') : $t('online') }}
                </v-chip>
              </strong>
            </div>
            <div v-if="nodeMember" class="node-detail__info-row">
              <span class="node-detail__info-label">Version</span>
              <strong class="node-detail__info-value">{{ nodeMember.lastVersion }}</strong>
            </div>
            <div v-if="nodeMember" class="node-detail__info-row">
              <span class="node-detail__info-label">Panel Version</span>
              <strong class="node-detail__info-value">{{ nodeMember.panelVersion || '-' }}</strong>
            </div>
          </div>
        </v-card-text>
      </v-card>

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
                :style="{ color: latencyCSSColor(inboundAvg, true) }"
              >
                {{ inboundAvg !== null ? `${inboundAvg}ms` : '-' }}
              </div>
              <div class="node-detail__latency-stat-sub">{{ nodeInboundResults.length }}{{ $t('ping.sources') }}</div>
            </div>
            <div class="node-detail__latency-stat">
              <div class="node-detail__latency-stat-label">{{ $t('ping.outbound') }}</div>
              <div
                class="node-detail__latency-stat-value"
                :style="{ color: latencyCSSColor(outboundAvg, true) }"
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
                <div v-for="r in nodeInboundResults" :key="r.source_label + '-' + r.target_member_id" class="node-detail__latency-detail-row">
                  <span class="node-detail__latency-detail-name">{{ r.source_label }}</span>
                  <span
                    class="node-detail__latency-detail-value"
                    :style="{ color: latencyCSSColor(r.latency_ms, r.success) }"
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
                    :style="{ color: latencyCSSColor(r.latency_ms, r.success) }"
                  >
                    {{ latencyText(r) }}
                  </span>
                </div>
              </div>
            </div>
          </template>
        </v-card-text>
      </v-card>

      <template v-if="supportsPanelExperience">
        <v-tabs v-model="activeTab" class="node-detail__tabs" color="primary">
          <v-tab value="inbounds">{{ $t('pages.inbounds') }}</v-tab>
          <v-tab value="clients">{{ $t('pages.clients') }}</v-tab>
          <v-tab value="tls">{{ $t('pages.tls') }}</v-tab>
          <v-tab value="services">{{ $t('pages.services') }}</v-tab>
          <v-tab value="rules">{{ $t('pages.rules') }}</v-tab>
          <v-tab value="outbounds">{{ $t('pages.outbounds') }}</v-tab>
          <v-tab value="endpoints">{{ $t('pages.endpoints') }}</v-tab>
        </v-tabs>

        <v-window v-model="activeTab" class="node-detail__panel-window">
          <v-window-item value="inbounds">
            <InboundsView />
          </v-window-item>
          <v-window-item value="clients">
            <ClientsView />
          </v-window-item>
          <v-window-item value="tls">
            <TlsView />
          </v-window-item>
          <v-window-item value="services">
            <ServicesView />
          </v-window-item>
          <v-window-item value="rules">
            <RulesView />
          </v-window-item>
          <v-window-item value="outbounds">
            <OutboundsView />
          </v-window-item>
          <v-window-item value="endpoints">
            <EndpointsView />
          </v-window-item>
        </v-window>
      </template>

      <template v-else>
        <v-tabs v-model="activeTab" class="node-detail__tabs" color="primary" grow>
          <v-tab value="inbounds">{{ $t('pages.inbounds') }}</v-tab>
          <v-tab value="clients">{{ $t('pages.clients') }}</v-tab>
          <v-tab value="tls">{{ $t('pages.tls') }}</v-tab>
          <v-tab value="services">{{ $t('pages.services') }}</v-tab>
          <v-tab value="routes">{{ $t('pages.rules') }}</v-tab>
          <v-tab value="outbounds">{{ $t('pages.outbounds') }}</v-tab>
        </v-tabs>

        <v-progress-linear v-if="currentTab.loading" indeterminate color="primary" class="node-detail__loading-bar" />

        <v-row class="app-grid">
          <v-col cols="12" md="6" lg="4" xl="3" v-for="item in currentTab.items" :key="itemKey(item)">
            <v-card class="app-entity-card" elevation="5">
              <v-card-title class="node-detail__card-title">
                <div>
                  <div class="node-detail__card-name">{{ item.tag || item.name || item.id || '-' }}</div>
                  <div class="node-detail__card-type">{{ item.type || item.protocol || '-' }}</div>
                </div>
              </v-card-title>
              <v-card-text class="app-entity-card__text">
                <v-row v-for="(value, key) in cardFields(item)" :key="String(key)">
                  <v-col>{{ String(key) }}</v-col>
                  <v-col>{{ value ?? '-' }}</v-col>
                </v-row>
              </v-card-text>
            </v-card>
          </v-col>
        </v-row>

        <div v-if="!currentTab.loading && currentTab.loaded && currentTab.items.length === 0" class="node-detail__empty">
          No items found.
        </div>

        <div v-if="currentTab.total > pageSize" class="node-detail__pagination">
          <v-pagination
            v-model="currentPage"
            :length="totalPages"
            :total-visible="7"
            rounded
            @update:model-value="onPageChange"
          />
        </div>
      </template>
    </template>
  </div>
</template>

<script lang="ts" setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import HttpUtils from '@/plugins/httputil'
import { useRemoteNodeStore } from '@/store/modules/remoteNode'
import type { ClusterMember, ClusterMemberConnection } from '@/types/clusters'
import Data from '@/store/modules/data'
import InboundsView from '@/views/Inbounds.vue'
import ClientsView from '@/views/Clients.vue'
import TlsView from '@/views/Tls.vue'
import ServicesView from '@/views/Services.vue'
import RulesView from '@/views/Rules.vue'
import OutboundsView from '@/views/Outbounds.vue'
import EndpointsView from '@/views/Endpoints.vue'
import { usePingStore } from '@/store/modules/ping'
import { latencyColor, latencyCSSColor, latencyText, sortedExternalByLatency } from '@/types/ping'

const route = useRoute()
const router = useRouter()
const remoteNode = useRemoteNodeStore()
const pingStore = usePingStore()
const latencyExpanded = ref(false)
const latencyTesting = ref(false)

const pageSize = 10
const nodeConnection = ref<ClusterMemberConnection | null>(null)
const nodeMember = ref<ClusterMember | null>(null)
const panelLoading = ref(false)
const panelReady = ref(false)

const nodeName = computed(() => nodeConnection.value?.displayName || nodeConnection.value?.name || nodeConnection.value?.nodeId || 'Node')
const nodeActions = computed(() => remoteNode.info?.actions ?? [])
const advertisesPanelExperience = computed(() => nodeActions.value.includes('panel.load') && nodeActions.value.includes('panel.save'))
const supportsPanelExperience = computed(() => panelReady.value || advertisesPanelExperience.value)
const isPageLoading = computed(() => remoteNode.pageLoading || panelLoading.value)

const currentNodeId = computed(() => nodeConnection.value?.nodeId ?? '')

const nodeInboundResults = computed(() => {
  const results = pingStore.externalResults?.results ?? []
  return sortedExternalByLatency(results.filter(r => r.direction === 'inbound'))
})

const nodeOutboundResults = computed(() => {
  const results = pingStore.externalResults?.results ?? []
  return sortedExternalByLatency(results.filter(r => r.direction === 'outbound'))
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
    const enabledSourceIds = pingStore.externalConfig?.sources.filter(s => s.enabled).map(s => s.id) ?? []
    if (enabledSourceIds.length > 0) {
      await pingStore.triggerExternalPing({
        direction: 'outbound',
        source_ids: enabledSourceIds,
        methods: ['tcp', 'http', 'icmp'],
      })
    }
  } finally {
    latencyTesting.value = false
  }
}

const activeTab = ref('inbounds')

const tabMap: Record<string, { data: () => any; action: string }> = {
  inbounds:  { data: () => remoteNode.inbounds,  action: 'inbound.list' },
  clients:   { data: () => remoteNode.clients,   action: 'client.list' },
  tls:       { data: () => remoteNode.tlsConfigs, action: 'tls.list' },
  services:  { data: () => remoteNode.services,   action: 'service.list' },
  routes:    { data: () => remoteNode.routes,     action: 'route.list' },
  outbounds: { data: () => remoteNode.outbounds,  action: 'outbound.list' },
}

const currentTab = computed(() => {
  const entry = tabMap[activeTab.value]
  return entry ? entry.data() : { items: [], total: 0, page: 1, loaded: false, loading: false }
})

const currentPage = ref(1)
const totalPages = computed(() => Math.ceil(currentTab.value.total / pageSize))

function itemKey(item: any): string {
  return item.tag || item.name || item.id || JSON.stringify(item)
}

function cardFields(item: any): Record<string, any> {
  const skip = new Set(['tag', 'name', 'type', 'protocol', 'id'])
  const fields: Record<string, any> = {}
  for (const [key, value] of Object.entries(item)) {
    if (skip.has(key)) continue
    if (typeof value === 'object' && value !== null) continue
    fields[key] = value
  }
  return fields
}

async function ensureTabLoaded() {
  const entry = tabMap[activeTab.value]
  if (!entry) return
  const tab = entry.data()
  if (!tab.loaded && !tab.loading) {
    currentPage.value = 1
    await remoteNode.fetchTab(tab, entry.action, 1)
  }
  currentPage.value = tab.page
}

watch(activeTab, () => {
  if (!supportsPanelExperience.value) {
    ensureTabLoaded()
  }
})

async function onPageChange(page: number) {
  const entry = tabMap[activeTab.value]
  if (!entry) return
  const tab = entry.data()
  await remoteNode.fetchTab(tab, entry.action, page)
}

function goBack() {
  router.push('/clusters')
}

function resolveRouteNodeId() {
  return String(route.query.id || '').trim()
}

async function loadNodeConnection(nodeId: string) {
  const msg = await HttpUtils.get(`api/cluster/member-connection?node_id=${encodeURIComponent(nodeId)}`)
  if (!msg.success) {
    throw new Error(msg.msg || 'Failed to load cluster member connection.')
  }
  const connection = msg.obj as ClusterMemberConnection
  if (!connection?.nodeId || !connection?.baseUrl) {
    throw new Error('Cluster member connection is missing nodeId or baseUrl.')
  }
  return connection
}

async function loadNodeMember(nodeId: string) {
  try {
    const msg = await HttpUtils.get('api/cluster/members')
    if (msg.success && Array.isArray(msg.obj)) {
      nodeMember.value = msg.obj.find((m: ClusterMember) => m.nodeId === nodeId) ?? null
    }
  } catch {
    // non-blocking: member detail is supplementary
  }
}

async function tryEnterRemotePanel() {
  if (!nodeConnection.value) return false
  panelLoading.value = true
  try {
    Data().enterRemoteNode(nodeConnection.value.nodeId, nodeConnection.value.baseUrl)
    await Data().loadData()
    panelReady.value = true
    return true
  } catch {
    panelReady.value = false
    if (Data().isRemote()) {
      Data().exitRemoteNode()
    }
    return false
  } finally {
    panelLoading.value = false
  }
}

onMounted(async () => {
  const nodeId = resolveRouteNodeId()

  if (!nodeId) {
    remoteNode.pageError = 'Missing id query parameter.'
    return
  }

  remoteNode.pageLoading = true
  remoteNode.pageError = null

  try {
    nodeConnection.value = await loadNodeConnection(nodeId)
    pingStore.loadExternalResults().catch(() => {})
    await loadNodeMember(nodeId)
    await remoteNode.init(nodeConnection.value.nodeId, nodeConnection.value.baseUrl)
    if (remoteNode.pageError) return
    if (advertisesPanelExperience.value || nodeActions.value.length === 0) {
      const panelEntered = await tryEnterRemotePanel()
      if (panelEntered) {
        return
      }
    }
    if (advertisesPanelExperience.value) {
      remoteNode.pageError = 'Remote panel management is advertised but panel.load failed.'
      return
    }
  } catch (e: any) {
    remoteNode.pageError = e.message
    remoteNode.pageLoading = false
    return
  }

  const entry = tabMap[activeTab.value]
  if (entry) {
    currentPage.value = entry.data().page
  }
})

onUnmounted(() => {
  if (Data().isRemote()) {
    panelReady.value = false
    Data().exitRemoteNode()
    void Data().loadData()
  }
  remoteNode.reset()
})
</script>

<style scoped>
.node-detail__error {
  margin-bottom: 16px;
}

.node-detail__info-card {
  margin-bottom: 16px;
}

.node-detail__info-grid {
  display: grid;
  gap: 8px;
}

.node-detail__info-row {
  align-items: start;
  border-bottom: 1px solid var(--app-border-1);
  display: grid;
  gap: 10px;
  grid-template-columns: 120px minmax(0, 1fr);
  padding-bottom: 8px;
}

.node-detail__info-row:last-child {
  border-bottom: none;
  padding-bottom: 0;
}

.node-detail__info-label {
  color: var(--app-text-3);
  font-size: 12px;
  letter-spacing: 0.04em;
}

.node-detail__info-value {
  font-size: 14px;
  overflow-wrap: anywhere;
}

.node-detail__info-value--mono {
  font-family: var(--app-font-mono, ui-monospace, monospace);
  font-size: 13px;
  letter-spacing: 0.02em;
}

.node-detail__tabs {
  margin-bottom: 4px;
}

.node-detail__panel-window {
  overflow: visible;
}

.node-detail__loading-bar {
  margin-bottom: 12px;
}

.node-detail__card-title {
  align-items: start;
  display: flex;
  gap: 12px;
  justify-content: space-between;
}

.node-detail__card-name {
  font-size: 18px;
  font-weight: 600;
  line-height: 1.1;
}

.node-detail__card-type {
  color: var(--app-text-3);
  font-size: 12px;
  letter-spacing: 0.12em;
  margin-top: 6px;
  text-transform: uppercase;
}

.node-detail__empty {
  color: var(--app-text-3);
  padding: 24px 0;
  text-align: center;
}

.node-detail__pagination {
  display: flex;
  justify-content: center;
  margin-top: 12px;
  padding-bottom: 16px;
}

@media (max-width: 640px) {
  .node-detail__info-row {
    grid-template-columns: 1fr;
  }
}

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
</style>
