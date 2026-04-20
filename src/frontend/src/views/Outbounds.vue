<template>
  <div class="app-page">
    <section class="app-page__hero">
      <div class="app-page__hero-head">
        <div class="app-page__hero-kicker">{{ $t('pages.outbounds') }}</div>
        <h1 class="app-page__hero-title">{{ $t('pages.outbounds') }}</h1>
        <p class="app-page__hero-copy">
          Inspect egress policy cards with endpoint posture, TLS state, latency checks, and live availability without dropping into a raw table.
        </p>
        <div class="app-page__hero-meta">
          <span class="app-page__hero-meta-item">{{ outbounds.length }} policies</span>
          <span class="app-page__hero-meta-item">{{ onlineCount }} online</span>
          <span class="app-page__hero-meta-item">{{ testedCount }} tested</span>
        </div>
      </div>
      <div class="app-page__hero-side">
        <div class="app-page__hero-stats">
          <div class="app-page__hero-stat">
            <span class="app-page__hero-stat-label">Healthy checks</span>
            <strong class="app-page__hero-stat-value">{{ successfulChecks }}</strong>
            <span class="app-page__hero-stat-note">Outbounds returning an OK latency response</span>
          </div>
          <div class="app-page__hero-stat">
            <span class="app-page__hero-stat-label">{{ $t('online') }}</span>
            <strong class="app-page__hero-stat-value">{{ onlineCount }}</strong>
            <span class="app-page__hero-stat-note">Routes currently selected in live traffic</span>
          </div>
        </div>
      </div>
    </section>

    <OutboundVue
      v-model="modal.visible"
      :visible="modal.visible"
      :id="modal.id"
      :data="modal.data"
      :tags="outboundTags"
      @close="closeModal"
    />
    <OutboundBulk
      v-model="bulkModal.visible"
      :visible="bulkModal.visible"
      :outboundTags="outboundTags"
      @close="closeBulkModal"
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
          <v-btn color="primary" prepend-icon="mdi-playlist-plus" @click="showBulkModal">{{ $t('actions.addbulk') }}</v-btn>
          <v-btn
            color="secondary"
            variant="outlined"
            :loading="testingAll"
            append-icon="mdi-speedometer"
            :disabled="testingAll || outbounds.length === 0"
            @click="checkAllOutbounds"
          >
            {{ $t('actions.testAll') || 'Test all' }}
          </v-btn>
        </div>
      </v-col>
    </v-row>

    <v-row class="app-grid">
      <v-col cols="12" md="6" lg="4" xl="3" v-for="(item, index) in outbounds" :key="item.tag">
        <v-card class="app-entity-card outbound-card" elevation="5">
          <v-card-title class="outbound-card__title">
            <div>
              <div class="outbound-card__name">{{ item.tag }}</div>
              <div class="outbound-card__type">{{ item.type }}</div>
            </div>
            <div class="outbound-card__chips">
              <v-chip size="small" density="comfortable" :color="item.tls?.enabled ? 'info' : ''" variant="flat">
                {{ Object.hasOwn(item, 'tls') ? $t(item.tls?.enabled ? 'enable' : 'disable') : '-' }}
              </v-chip>
              <v-chip v-if="onlines.includes(item.tag)" size="small" density="comfortable" color="success" variant="flat">
                {{ $t('online') }}
              </v-chip>
            </div>
          </v-card-title>
          <v-card-text class="app-entity-card__text">
            <v-row>
              <v-col>{{ $t('in.addr') }}</v-col>
              <v-col>{{ item.server ?? '-' }}</v-col>
            </v-row>
            <v-row>
              <v-col>{{ $t('in.port') }}</v-col>
              <v-col class="font-mono">{{ item.server_port ?? '-' }}</v-col>
            </v-row>
            <v-row>
              <v-col>{{ $t('out.delay') }}</v-col>
              <v-col>
                <v-progress-circular v-if="checkResults[item.tag]?.loading" indeterminate size="20" />
                <template v-else-if="checkResults[item.tag]">
                  <v-chip
                    v-if="checkResults[item.tag].success"
                    density="compact"
                    size="small"
                    color="success"
                    variant="flat"
                  >
                    {{ checkResults[item.tag].data?.Delay + $t('date.ms') }}
                  </v-chip>
                  <v-tooltip v-else location="top" :text="checkResults[item.tag].errorMessage || $t('failed')">
                    <template #activator="{ props }">
                      <v-icon v-bind="props" size="small" color="error" icon="mdi-close-circle" />
                    </template>
                  </v-tooltip>
                </template>
                <v-btn v-else density="comfortable" icon="mdi-speedometer" size="x-small" variant="text" @click="checkOutbound(item.tag)" />
              </v-col>
            </v-row>
          </v-card-text>
          <v-divider />
          <v-card-actions class="app-card-actions">
            <v-btn icon="mdi-file-edit" @click="showModal(item.id)">
              <v-icon />
              <v-tooltip activator="parent" location="top" :text="$t('actions.edit')" />
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
                  <v-btn color="error" variant="outlined" @click="delOutbound(item.tag)">{{ $t('yes') }}</v-btn>
                  <v-btn color="success" variant="outlined" @click="delOverlay[index] = false">{{ $t('no') }}</v-btn>
                </v-card-actions>
              </v-card>
            </v-overlay>
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
import HttpUtils from '@/plugins/httputil'
import OutboundVue from '@/layouts/modals/Outbound.vue'
import OutboundBulk from '@/layouts/modals/OutboundBulk.vue'
import Stats from '@/layouts/modals/Stats.vue'
import { Outbound } from '@/types/outbounds'
import { computed, ref } from 'vue'

interface CheckResult {
  loading?: boolean
  success: boolean
  data?: { OK?: boolean; Delay?: number; Error?: string } | null
  errorMessage?: string
}

const checkResults = ref<Record<string, CheckResult>>({})

const checkOutbound = async (tag: string) => {
  checkResults.value = { ...checkResults.value, [tag]: { loading: true, success: false } }
  const msg = await HttpUtils.get('api/checkOutbound', { tag })
  const success = msg.success && msg.obj?.OK
  const errorMessage = success ? undefined : (msg.obj?.Error ?? msg.msg ?? '')
  checkResults.value = {
    ...checkResults.value,
    [tag]: { loading: false, success, data: msg.obj ?? null, errorMessage },
  }
}

const testingAll = ref(false)

const checkAllOutbounds = async () => {
  if (outbounds.value.length === 0) return
  testingAll.value = true
  try {
    await Promise.all(outbounds.value.map(item => checkOutbound(item.tag)))
  } finally {
    testingAll.value = false
  }
}

const outbounds = computed((): Outbound[] => <Outbound[]>Data().outbounds)
const outboundTags = computed((): string[] => [
  ...Data().outbounds?.map((outbound: Outbound) => outbound.tag),
  ...Data().endpoints?.map((endpoint: any) => endpoint.tag),
])
const onlines = computed(() => Data().onlines.outbound ?? [])
const onlineCount = computed(() => onlines.value.length)
const testedCount = computed(() => Object.keys(checkResults.value).length)
const successfulChecks = computed(() => Object.values(checkResults.value).filter(result => result.success).length)

const modal = ref({
  visible: false,
  id: 0,
  data: '',
})

const delOverlay = ref<boolean[]>([])

const showModal = (id: number) => {
  modal.value.id = id
  modal.value.data = id === 0 ? '' : JSON.stringify(outbounds.value.findLast(item => item.id === id))
  modal.value.visible = true
}

const closeModal = () => {
  modal.value.visible = false
}

const bulkModal = ref({ visible: false })

const showBulkModal = () => {
  bulkModal.value.visible = true
}

const closeBulkModal = () => {
  bulkModal.value.visible = false
}

const stats = ref({
  visible: false,
  resource: 'outbound',
  tag: '',
})

const delOutbound = async (tag: string) => {
  const index = outbounds.value.findIndex(item => item.tag === tag)
  const success = await Data().save('outbounds', 'del', tag)
  if (success) delOverlay.value[index] = false
}

const showStats = (tag: string) => {
  stats.value.tag = tag
  stats.value.visible = true
}

const closeStats = () => {
  stats.value.visible = false
}
</script>

<style scoped>
.outbound-card__title {
  align-items: start;
  display: flex;
  gap: 12px;
  justify-content: space-between;
}

.outbound-card__name {
  font-size: 18px;
  font-weight: 600;
  line-height: 1.1;
}

.outbound-card__type {
  color: var(--app-text-3);
  font-size: 12px;
  letter-spacing: 0.12em;
  margin-top: 6px;
  text-transform: uppercase;
}

.outbound-card__chips {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: flex-end;
}
</style>
