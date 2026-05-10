<template>
  <div class="app-page">
    <section class="app-page__hero">
      <div class="app-page__hero-head">
        <div class="app-page__hero-kicker">{{ $t('pages.inbounds') }}</div>
        <h1 class="app-page__hero-title">{{ $t('pages.inbounds') }}</h1>
        <p class="app-page__hero-copy">
          Manage ingress listeners as a card-first catalog with quick access to address, protocol, TLS posture, connected users, and traffic drill-down.
        </p>
        <div class="app-page__hero-meta">
          <span class="app-page__hero-meta-item">{{ inbounds.length }} listeners</span>
          <span class="app-page__hero-meta-item">{{ onlineCount }} active routes</span>
          <span class="app-page__hero-meta-item">{{ tlsEnabledCount }} TLS enabled</span>
        </div>
      </div>
      <div class="app-page__hero-side">
        <div class="app-page__hero-stats">
          <div class="app-page__hero-stat">
            <span class="app-page__hero-stat-label">Users mapped</span>
            <strong class="app-page__hero-stat-value">{{ totalUsers }}</strong>
            <span class="app-page__hero-stat-note">Inbound user bindings across the catalog</span>
          </div>
          <div class="app-page__hero-stat">
            <span class="app-page__hero-stat-label">{{ $t('online') }}</span>
            <strong class="app-page__hero-stat-value">{{ onlineCount }}</strong>
            <span class="app-page__hero-stat-note">Listeners currently carrying live users</span>
          </div>
        </div>
      </div>
    </section>

    <InboundVue
      v-model="modal.visible"
      :visible="modal.visible"
      :id="modal.id"
      :inTags="inTags"
      :tlsConfigs="tlsConfigs"
      :readonly="modal.readonly"
      @close="closeModal"
    />
    <Stats
      v-model="stats.visible"
      :visible="stats.visible"
      :resource="stats.resource"
      :tag="stats.tag"
      @close="closeStats"
    />

    <v-row class="app-page__toolbar">
      <v-col cols="12">
        <div class="app-page__toolbar-actions app-toolbar-cluster">
          <v-btn color="primary" prepend-icon="mdi-plus" @click="showModal(0)">{{ $t('actions.add') }}</v-btn>
        </div>
      </v-col>
    </v-row>

    <v-row class="app-grid">
      <v-col cols="12" md="6" lg="4" xl="3" v-for="(item, index) in inbounds" :key="item.tag">
        <v-card class="app-entity-card inbound-card" :class="{ 'inbound-card--domain': isClusterManaged(item) }" elevation="5">
          <v-card-title class="inbound-card__title">
            <div>
              <div class="inbound-card__name">{{ item.tag }}</div>
              <div class="inbound-card__type">{{ item.type }}</div>
            </div>
            <div class="inbound-card__chips">
              <v-chip size="small" density="comfortable" :color="item.tls_id > 0 ? 'info' : ''" variant="flat">
                {{ item.tls_id > 0 ? $t('enable') : $t('disable') }}
              </v-chip>
              <v-chip v-if="isClusterManaged(item)" class="inbound-card__managed-chip" size="small" density="comfortable" color="info" variant="flat">
                Domain Managed
                <v-tooltip activator="parent" location="top" text="Managed by domain resource operation" />
              </v-chip>
              <v-chip v-if="onlines.includes(item.tag)" size="small" density="comfortable" color="success" variant="flat">
                {{ $t('online') }}
              </v-chip>
            </div>
          </v-card-title>
          <v-card-text class="app-entity-card__text">
            <div v-if="isClusterManaged(item)" class="inbound-card__managed-banner">
              <span>Domain inbound managed by domain resource operation</span>
              <strong>{{ item.cluster_domain || 'Domain scope' }}</strong>
            </div>
            <v-row>
              <v-col>{{ $t('in.addr') }}</v-col>
              <v-col>{{ item.listen || '-' }}</v-col>
            </v-row>
            <v-row>
              <v-col>{{ $t('in.port') }}</v-col>
              <v-col class="font-mono">{{ item.listen_port }}</v-col>
            </v-row>
            <v-row>
              <v-col>{{ $t('pages.clients') }}</v-col>
              <v-col>
                <template v-if="inboundUsers(item).length > 0">
                  <v-tooltip activator="parent" dir="ltr" location="bottom">
                    <span v-for="user in inboundUsers(item)" :key="user">{{ user }}<br /></span>
                  </v-tooltip>
                  {{ inboundUsers(item).length }}
                </template>
                <template v-else>-</template>
              </v-col>
            </v-row>
          </v-card-text>
          <v-divider />
          <v-card-actions class="app-card-actions">
            <v-btn :icon="isClusterManaged(item) ? 'mdi-eye' : 'mdi-file-edit'" @click="showModal(item.id, isClusterManaged(item))">
              <v-icon />
              <v-tooltip activator="parent" location="top" :text="isClusterManaged(item) ? 'View actual inbound settings' : $t('actions.edit')" />
            </v-btn>
            <v-btn icon="mdi-file-remove" color="warning" @click="delOverlay[index] = true">
              <v-icon />
              <v-tooltip activator="parent" location="top" :text="$t('actions.del')" />
            </v-btn>
            <v-overlay
              v-model="delOverlay[index]"
              contained
              class="align-center justify-center"
            >
              <v-card :title="$t('actions.del')" rounded="lg">
                <v-divider />
                <v-card-text>{{ $t('confirm') }}</v-card-text>
                <v-card-actions>
                  <v-btn color="error" variant="outlined" @click="delInbound(item.id)">{{ $t('yes') }}</v-btn>
                  <v-btn color="success" variant="outlined" @click="delOverlay[index] = false">{{ $t('no') }}</v-btn>
                </v-card-actions>
              </v-card>
            </v-overlay>
            <v-btn icon="mdi-content-duplicate" :disabled="isClusterManaged(item)" :loading="cloneLoading" @click="clone(item.id)">
              <v-icon />
              <v-tooltip activator="parent" location="top" :text="isClusterManaged(item) ? 'Managed by domain resource operation' : $t('actions.clone')" />
            </v-btn>
            <v-btn v-if="Data().enableTraffic" icon="mdi-chart-line" @click="showStats(item.tag)">
              <v-icon />
              <v-tooltip activator="parent" location="top" :text="$t('stats.graphTitle')" />
            </v-btn>
          </v-card-actions>
        </v-card>
      </v-col>
    </v-row>
  </div>
</template>

<script lang="ts" setup>
import Data from '@/store/modules/data'
import InboundVue from '@/layouts/modals/Inbound.vue'
import Stats from '@/layouts/modals/Stats.vue'
import { computed, ref } from 'vue'
import { createInbound, Inbound } from '@/types/inbounds'
import RandomUtil from '@/plugins/randomUtil'

const inbounds = computed((): Inbound[] => <Inbound[]>Data().inbounds)
const tlsConfigs = computed((): any[] => <any[]>Data().tlsConfigs)
const inTags = computed((): string[] => [
  ...inbounds.value.map(item => item.tag),
  ...Data().endpoints?.filter((endpoint: any) => endpoint.listen_port > 0).map((endpoint: any) => endpoint.tag),
])
const onlines = computed(() => Data().onlines.inbound ?? [])
const onlineCount = computed(() => onlines.value.length)
const tlsEnabledCount = computed(() => inbounds.value.filter(item => item.tls_id > 0).length)
const inboundUsers = (item: any): string[] => item.users ?? []
const totalUsers = computed(() => inbounds.value.reduce((sum, item: any) => sum + inboundUsers(item).length, 0))
const isClusterManaged = (item: any): boolean => item?.cluster_managed === true || item?.cluster_read_only === true

const modal = ref({
  visible: false,
  id: 0,
  readonly: false,
})

const delOverlay = ref<boolean[]>([])

const showModal = (id: number, readonly = false) => {
  modal.value.id = id
  modal.value.readonly = readonly
  modal.value.visible = true
}

const closeModal = () => {
  modal.value.visible = false
  modal.value.readonly = false
}

const delInbound = async (id: number) => {
  const index = inbounds.value.findIndex(item => item.id === id)
  const tag = inbounds.value[index].tag
  const success = await Data().save('inbounds', 'del', tag)
  if (success) delOverlay.value[index] = false
}

const cloneLoading = ref(false)

const clone = async (id: number) => {
  cloneLoading.value = true
  const inboundArray = await Data().loadInbounds([id])
  const inbound = inboundArray[0]
  const newInbound = createInbound(inbound.type, {
    ...inbound,
    id: 0,
    tag: `${inbound.type}-${RandomUtil.randomSeq(3)}`,
    listen_port: RandomUtil.randomIntRange(10000, 60000),
  })
  await Data().save('inbounds', 'new', newInbound)
  cloneLoading.value = false
}

const stats = ref({
  visible: false,
  resource: 'inbound',
  tag: '',
})

const showStats = (tag: string) => {
  stats.value.tag = tag
  stats.value.visible = true
}

const closeStats = () => {
  stats.value.visible = false
}
</script>

<style scoped>
.inbound-card__title {
  align-items: start;
  display: flex;
  gap: 12px;
  justify-content: space-between;
}

.inbound-card__name {
  font-size: 18px;
  font-weight: 600;
  line-height: 1.1;
}

.inbound-card__type {
  color: var(--app-text-3);
  font-size: 12px;
  letter-spacing: 0.12em;
  margin-top: 6px;
  text-transform: uppercase;
}

.inbound-card__chips {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: flex-end;
}

.inbound-card--domain {
  background:
    linear-gradient(135deg, color-mix(in srgb, var(--app-state-info) 16%, transparent), transparent 42%),
    color-mix(in srgb, var(--app-surface-2) 92%, var(--app-state-info));
  border: 1px solid color-mix(in srgb, var(--app-state-info) 62%, transparent);
  box-shadow:
    0 18px 46px color-mix(in srgb, var(--app-state-info) 16%, transparent),
    inset 0 0 0 1px color-mix(in srgb, var(--app-state-info) 18%, transparent);
}

.inbound-card__managed-chip {
  font-weight: 700;
  letter-spacing: 0;
}

.inbound-card__managed-banner {
  align-items: center;
  background: color-mix(in srgb, var(--app-state-info) 12%, transparent);
  border: 1px solid color-mix(in srgb, var(--app-state-info) 34%, transparent);
  border-radius: 8px;
  color: var(--app-text-1);
  display: flex;
  gap: 8px;
  justify-content: space-between;
  margin-bottom: 14px;
  padding: 10px 12px;
}

.inbound-card__managed-banner span {
  color: var(--app-text-2);
  font-size: 12px;
}

.inbound-card__managed-banner strong {
  color: var(--app-state-info);
  font-size: 12px;
  font-weight: 700;
  max-width: 45%;
  overflow-wrap: anywhere;
  text-align: right;
}
</style>
