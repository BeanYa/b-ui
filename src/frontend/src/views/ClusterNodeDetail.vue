<template>
  <div class="app-page">
    <!-- Full-page loading overlay during init -->
    <v-overlay v-model="remoteNode.pageLoading" class="align-center justify-center" persistent>
      <v-progress-circular indeterminate size="64" />
    </v-overlay>

    <!-- Error alert if init fails -->
    <v-alert v-if="remoteNode.pageError" type="error" variant="tonal" closable class="node-detail__error">
      {{ remoteNode.pageError }}
    </v-alert>

    <template v-if="!remoteNode.pageLoading && !remoteNode.pageError">
      <!-- Node info card -->
      <section class="app-page__hero">
        <div class="app-page__hero-head">
          <div class="app-page__hero-kicker">Cluster Node</div>
          <h1 class="app-page__hero-title">{{ nodeName }}</h1>
          <p class="app-page__hero-copy">
            Remote node detail view. Browse inbounds, clients, TLS configs, services, routes, and outbounds.
          </p>
          <div class="app-page__hero-meta">
            <span class="app-page__hero-meta-item">{{ nodeActions.length }} actions</span>
            <span class="app-page__hero-meta-item">{{ remoteNode.baseURL }}</span>
          </div>
        </div>
      </section>

      <!-- Toolbar with back button -->
      <v-row class="app-page__toolbar">
        <v-col cols="12">
          <div class="app-page__toolbar-actions">
            <v-btn variant="outlined" prepend-icon="mdi-arrow-left" @click="goBack">
              {{ $t('clusterCenter.actions.back') }}
            </v-btn>
          </div>
        </v-col>
      </v-row>

      <!-- Tabs -->
      <v-tabs v-model="activeTab" class="node-detail__tabs" color="primary" grow>
        <v-tab value="inbounds">入站</v-tab>
        <v-tab value="clients">用户</v-tab>
        <v-tab value="tls">TLS</v-tab>
        <v-tab value="services">服务</v-tab>
        <v-tab value="routes">路由</v-tab>
        <v-tab value="outbounds">出站</v-tab>
      </v-tabs>

      <!-- Tab loading bar -->
      <v-progress-linear v-if="currentTab.loading" indeterminate color="primary" class="node-detail__loading-bar" />

      <!-- Tab content: card grid -->
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

      <!-- Empty state -->
      <div v-if="!currentTab.loading && currentTab.loaded && currentTab.items.length === 0" class="node-detail__empty">
        No items found.
      </div>

      <!-- Pagination -->
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
  </div>
</template>

<script lang="ts" setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import HttpUtils from '@/plugins/httputil'
import { useRemoteNodeStore } from '@/store/modules/remoteNode'
import type { ClusterMemberConnection } from '@/types/clusters'

const route = useRoute()
const router = useRouter()
const remoteNode = useRemoteNodeStore()

const pageSize = 10
const nodeConnection = ref<ClusterMemberConnection | null>(null)

const nodeName = computed(() => nodeConnection.value?.displayName || nodeConnection.value?.name || nodeConnection.value?.nodeId || 'Node')
const nodeActions = computed(() => remoteNode.info?.actions ?? [])

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
  ensureTabLoaded()
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
  if (!connection?.baseUrl || !connection?.token) {
    throw new Error('Cluster member connection is missing baseUrl or token.')
  }
  return connection
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
    await remoteNode.init(nodeConnection.value.baseUrl, nodeConnection.value.token)
  } catch (e: any) {
    remoteNode.pageError = e.message
    remoteNode.pageLoading = false
    return
  }

  // TLS and clients are pre-fetched by init(), so mark current page
  const entry = tabMap[activeTab.value]
  if (entry) {
    currentPage.value = entry.data().page
  }
})

onUnmounted(() => {
  remoteNode.reset()
})
</script>

<style scoped>
.node-detail__error {
  margin-bottom: 16px;
}

.node-detail__tabs {
  margin-bottom: 4px;
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
</style>
