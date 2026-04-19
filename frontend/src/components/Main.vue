<template>
  <div class="dashboard-root">
  <LogVue v-model="logModal.visible" :control="logModal" :visible="logModal.visible" />
  <Backup v-model="backupModal.visible" :control="backupModal" :visible="backupModal.visible" />
  <UsageStats v-model:visible="usageStatsModal.visible" />
  <v-container class="dashboard-shell" fluid>
    <v-row class="dashboard-shell__overview" density="comfortable">
      <v-col cols="12" xl="3" lg="4">
        <v-card class="overview-card overview-card--intro">
          <div class="overview-card__glow"></div>
          <div class="overview-card__eyebrow">
            <span>Operations Console</span>
          </div>
          <h1 class="overview-card__title">{{ $t('pages.home') }}</h1>
          <p class="overview-card__copy">
            Keep runtime health, transport activity, and topology counts in one stable control surface tuned for scanning rather than hero-style dashboards.
          </p>
          <div class="overview-card__chips">
            <span>{{ selectedPanelLabel }}</span>
            <span>{{ hostAddress }}</span>
          </div>
          <div class="overview-card__actions">
            <v-btn color="primary" prepend-icon="mdi-refresh" variant="flat" @click="syncDashboard()">
              {{ $t('actions.update') }}
            </v-btn>
            <v-btn prepend-icon="mdi-backup-restore" variant="tonal" @click="backupModal.visible = true">
              {{ $t('main.backup.title') }}
            </v-btn>
            <v-btn prepend-icon="mdi-chart-box-outline" variant="tonal" @click="usageStatsModal.visible = true">
              {{ $t('main.stats.title') }}
            </v-btn>
            <v-btn prepend-icon="mdi-list-box-outline" variant="tonal" @click="logModal.visible = true">
              {{ $t('basic.log.title') }}
            </v-btn>
          </div>
        </v-card>
      </v-col>

      <v-col cols="12" xl="6" lg="5">
        <v-card class="overview-card overview-card--stats">
          <div class="section-head">
            <div>
              <div class="section-head__label">Control Map</div>
              <div class="section-head__title">Service footprint</div>
            </div>
            <div class="section-head__caption">Compact counts, no oversized hero tiles.</div>
          </div>
          <div class="overview-grid">
            <div
              v-for="item in overviewCards"
              :key="item.label"
              class="overview-grid__item"
            >
              <div class="overview-grid__meta">
                <v-icon :icon="item.icon" size="16" />
                <span>{{ item.label }}</span>
              </div>
              <strong class="overview-grid__value">{{ item.value }}</strong>
              <span class="overview-grid__note">{{ item.note }}</span>
            </div>
          </div>
        </v-card>
      </v-col>

      <v-col cols="12" xl="3" lg="3">
        <v-card class="overview-card overview-card--runtime">
          <div class="section-head section-head--runtime">
            <div>
              <div class="section-head__label">{{ $t('main.info.sys') }}</div>
              <div class="section-head__title" :title="runtimeHost">{{ runtimeHost }}</div>
            </div>
            <v-btn
              density="comfortable"
              icon="mdi-refresh"
              size="small"
              variant="text"
              @click="loadOverviewStatus()"
            />
          </div>
          <div class="runtime-card__address">{{ hostAddress }}</div>
          <div class="runtime-grid">
            <div
              v-for="item in runtimeCards"
              :key="item.label"
              :class="['runtime-grid__item', `runtime-grid__item--${item.tone}`]"
            >
              <span class="runtime-grid__label">{{ item.label }}</span>
              <strong class="runtime-grid__value" :title="item.value">{{ item.value }}</strong>
              <span class="runtime-grid__note" :title="item.note">{{ item.note }}</span>
            </div>
          </div>
          <p class="runtime-card__footer">{{ telemetryFooter }}</p>
        </v-card>
      </v-col>
    </v-row>

    <section class="dashboard-shell__tiles">
      <div
        v-for="section in tileSections"
        :key="section.key"
        class="telemetry-section"
      >
        <div class="telemetry-section__head">
          <div>
            <div class="telemetry-section__label">{{ tileSectionMeta[section.key].label }}</div>
            <div class="telemetry-section__title">{{ tileSectionMeta[section.key].title }}</div>
          </div>
          <div class="telemetry-section__caption">{{ tileSectionMeta[section.key].caption }}</div>
        </div>
        <div :class="['telemetry-grid', `telemetry-grid--${section.key}`]">
          <v-card
            v-for="i in section.items"
            :key="i"
            class="tile-card"
            :class="tileCardClasses(i)"
            height="100%"
          >
          <div class="tile-card__sheen"></div>
          <v-card-title class="tile-card__title">
            <div class="tile-card__title-text">
              {{ tileTitleMap[i] }}
            </div>
            <div class="tile-card__controls">
              <v-btn
                v-if="i == 'i-sys'"
                density="comfortable"
                icon="mdi-update"
                size="small"
                variant="text"
                @click="reloadSys()"
              />
              <v-btn
                v-if="i == 'h-net'"
                density="comfortable"
                icon="mdi-information-outline"
                size="small"
                variant="text"
              >
                <v-tooltip activator="parent" location="top">
                  {{ '↓' + HumanReadable.sizeFormat(tilesData.net?.recv) + ' - ' + HumanReadable.sizeFormat(tilesData.net?.sent) + '↑' }}
                </v-tooltip>
              </v-btn>
            </div>
          </v-card-title>
          <v-card-text :class="['tile-card__body', { 'tile-card__body--metric': i.charAt(0) == 'g' }]">
              <Gauge :tilesData="tilesData" :type="i" v-if="i.charAt(0) == 'g'" />
              <History :tilesData="tilesData" :type="i" v-if="i.charAt(0) == 'h'" />
              <template v-if="i == 'i-sys'">
                <v-row class="tile-info-grid">
                  <v-col cols="12" sm="4" class="tile-info-label">{{ $t('main.info.host') }}</v-col>
                  <v-col cols="12" sm="8" class="tile-info-value">{{ tilesData.sys?.hostName }}</v-col>
                  <v-col cols="12" sm="4" class="tile-info-label">{{ $t('main.info.cpu') }}</v-col>
                  <v-col cols="12" sm="8" class="tile-info-value">
                    <v-chip density="compact" variant="flat" class="tile-info-chip">
                      <v-tooltip activator="parent" location="top" style="direction: ltr;">
                        {{ tilesData.sys?.cpuType }}
                      </v-tooltip>
                     {{ tilesData.sys?.cpuCount }} {{ $t('main.info.core') }}
                    </v-chip>
                  </v-col>
                  <v-col cols="12" sm="4" class="tile-info-label">IP</v-col>
                  <v-col cols="12" sm="8" class="tile-info-value tile-info-chip-wrap">
                    <v-chip density="compact" color="primary" variant="flat" v-if="tilesData.sys?.ipv4?.length>0" class="tile-info-chip">
                      <v-tooltip activator="parent" location="top" style="direction: ltr;">
                        <span v-html="tilesData.sys?.ipv4?.join('<br />')"></span>
                      </v-tooltip>
                      IPv4
                    </v-chip>
                    <v-chip density="compact" color="primary" variant="flat" v-if="tilesData.sys?.ipv6?.length>0" class="tile-info-chip">
                      <v-tooltip activator="parent" location="top" style="direction: ltr;">
                        <span v-html="tilesData.sys?.ipv6?.join('<br />')"></span>
                      </v-tooltip>
                      IPv6
                    </v-chip>
                  </v-col>
                  <v-col cols="12" sm="4" class="tile-info-label">B-UI</v-col>
                  <v-col cols="12" sm="8" class="tile-info-value">
                    <v-chip density="compact" color="blue" class="tile-info-chip">
                      v{{ tilesData.sys?.appVersion }}
                    </v-chip>
                  </v-col>
                  <v-col cols="12" sm="4" class="tile-info-label">{{ $t('main.info.uptime') }}</v-col>
                  <v-col cols="12" sm="8" class="tile-info-value" v-tooltip:top="$t('main.info.startupTime')
                    + ': ' + new Date((tilesData.sys?.bootTime || 0) * 1000).toLocaleString(locale)">
                    {{ HumanReadable.formatSecond((Date.now()/1000) - tilesData.sys?.bootTime) }}
                  </v-col>
                </v-row>
              </template>
              <template v-if="i == 'i-sbd'">
                <v-row class="tile-info-grid">
                  <v-col cols="12" sm="4" class="tile-info-label">{{ $t('main.info.running') }}</v-col>
                  <v-col cols="12" sm="8" class="tile-info-value tile-info-chip-wrap">
                    <v-chip density="compact" color="success" variant="flat" v-if="tilesData.sbd?.running" class="tile-info-chip">{{ $t('yes') }}</v-chip> 
                    <v-chip density="compact" color="error" variant="flat" v-else class="tile-info-chip">{{ $t('no') }}</v-chip>
                    <v-chip density="compact" color="transparent" v-if="tilesData.sbd?.running && !loading" style="cursor: pointer;" class="tile-info-chip" @click="restartSingbox()">
                      <v-tooltip activator="parent" location="top">
                        {{ $t('actions.restartSb') }}
                      </v-tooltip>
                      <v-icon icon="mdi-restart" color="warning" />
                    </v-chip>
                  </v-col>
                  <v-col cols="12" sm="4" class="tile-info-label">{{ $t('main.info.memory') }}</v-col>
                  <v-col cols="12" sm="8" class="tile-info-value">
                    <v-chip density="compact" color="primary" variant="flat" v-if="tilesData.sbd?.stats?.Alloc" class="tile-info-chip">
                      {{ HumanReadable.sizeFormat(tilesData.sbd?.stats?.Alloc) }}
                    </v-chip> 
                  </v-col>
                  <v-col cols="12" sm="4" class="tile-info-label">{{ $t('main.info.threads') }}</v-col>
                  <v-col cols="12" sm="8" class="tile-info-value">
                    <v-chip density="compact" color="primary" variant="flat" v-if="tilesData.sbd?.stats?.NumGoroutine" class="tile-info-chip">
                      {{ tilesData.sbd?.stats?.NumGoroutine }}
                    </v-chip>
                  </v-col>
                  <v-col cols="12" sm="4" class="tile-info-label">{{ $t('main.info.uptime') }}</v-col>
                  <v-col cols="12" sm="8" class="tile-info-value">{{ HumanReadable.formatSecond(tilesData.sbd?.stats?.Uptime) }}</v-col>
                  <v-col cols="12" sm="4" class="tile-info-label">{{ $t('online') }}</v-col>
                  <v-col cols="12" sm="8" class="tile-info-value tile-info-chip-wrap">
                    <template v-if="tilesData.sbd?.running">
                      <v-chip density="compact" color="primary" variant="flat" v-if="Data().onlines.user" class="tile-info-chip">
                        <v-tooltip activator="parent" location="top" overflow="auto">
                          <span v-text="$t('pages.clients')" style="font-weight: bold;"></span><br/>
                          <span v-for="user in Data().onlines.user">{{ user }}<br /></span>
                        </v-tooltip>
                        {{ Data().onlines.user?.length }}
                      </v-chip>
                      <v-chip density="compact" color="success" variant="flat" v-if="Data().onlines.inbound" class="tile-info-chip">
                        <v-tooltip activator="parent" location="top" :text="$t('pages.inbounds')">
                          <span v-text="$t('pages.inbounds')" style="font-weight: bold;"></span><br/>
                          <span v-for="i in Data().onlines.inbound">{{ i }}<br /></span>
                        </v-tooltip>
                        {{ Data().onlines.inbound?.length }}
                      </v-chip>
                      <v-chip density="compact" color="info" variant="flat" v-if="Data().onlines.outbound" class="tile-info-chip">
                        <v-tooltip activator="parent" location="top" :text="$t('pages.outbounds')">
                          <span v-text="$t('pages.outbounds')" style="font-weight: bold;"></span><br/>
                          <span v-for="o in Data().onlines.outbound">{{ o }}<br /></span>
                        </v-tooltip>
                        {{ Data().onlines.outbound?.length }}
                      </v-chip>
                    </template>
                  </v-col>
                </v-row>
              </template>
          </v-card-text>
          </v-card>
        </div>
      </div>
    </section>
  </v-container>
  </div>
</template>

<script lang="ts" setup>
import HttpUtils from '@/plugins/httputil'
import { HumanReadable } from '@/plugins/utils'
import Data from '@/store/modules/data'
import Gauge from '@/components/tiles/Gauge.vue'
import History from '@/components/tiles/History.vue'
import { computed, onActivated, onBeforeUnmount, onDeactivated, onMounted, ref } from 'vue'
import { i18n, locale } from '@/locales'
import LogVue from '@/layouts/modals/Logs.vue'
import Backup from '@/layouts/modals/Backup.vue'
import UsageStats from '@/layouts/modals/UsageStats.vue'
import { splitTileItemsByLayout } from '@/features/dashboard/layout'
import { mergeTilesData } from '@/features/dashboard/persistence'

const loading = ref(false)
type TileSectionKey = 'metric' | 'detail'
const tileCatalog = [
  { title: i18n.global.t('main.gauge.cpu'), value: 'g-cpu' },
  { title: i18n.global.t('main.gauge.mem'), value: 'g-mem' },
  { title: i18n.global.t('main.gauge.dsk'), value: 'g-dsk' },
  { title: i18n.global.t('main.gauge.swp'), value: 'g-swp' },
  { title: i18n.global.t('main.chart.net'), value: 'h-net' },
  { title: i18n.global.t('main.chart.pnet'), value: 'hp-net' },
  { title: i18n.global.t('main.chart.dio'), value: 'h-dio' },
  { title: i18n.global.t('main.chart.cpu'), value: 'h-cpu' },
  { title: i18n.global.t('main.chart.mem'), value: 'h-mem' },
  { title: i18n.global.t('main.info.sys'), value: 'i-sys' },
  { title: i18n.global.t('main.info.sbd'), value: 'i-sbd' },
]
const fixedTileItems = tileCatalog.map(item => item.value)
const tileTitleMap = Object.fromEntries(tileCatalog.map(item => [item.value, item.title]))
const tileLayoutGroups = splitTileItemsByLayout(fixedTileItems)
const tileSections: Array<{ key: TileSectionKey, items: string[] }> = [
  { key: 'metric', items: tileLayoutGroups.metric },
  { key: 'detail', items: tileLayoutGroups.detail },
].filter(section => section.items.length > 0) as Array<{ key: TileSectionKey, items: string[] }>
const tileSectionMeta = {
  metric: {
    label: 'Pulse Layer',
    title: 'Primary gauges',
    caption: 'Always-visible resource pressure and capacity.',
  },
  detail: {
    label: 'Signal Layer',
    title: 'Traffic and runtime detail',
    caption: 'Historic curves and system state for deeper checks.',
  },
} as const

const tilesData = ref(<any>{})
const dataStore = Data()

const overviewCards = computed(() => [
  {
    label: i18n.global.t('pages.inbounds'),
    value: String(dataStore.inbounds.length),
    note: 'Ingress listeners',
    icon: 'mdi-arrow-collapse-down',
  },
  {
    label: i18n.global.t('pages.clients'),
    value: String(dataStore.clients.length),
    note: 'Provisioned identities',
    icon: 'mdi-account-multiple-outline',
  },
  {
    label: i18n.global.t('pages.outbounds'),
    value: String(dataStore.outbounds.length),
    note: 'Egress policies',
    icon: 'mdi-arrow-collapse-up',
  },
  {
    label: i18n.global.t('pages.services'),
    value: String(dataStore.services.length),
    note: 'Attached integrations',
    icon: 'mdi-server-outline',
  },
  {
    label: i18n.global.t('pages.endpoints'),
    value: String(dataStore.endpoints.length),
    note: 'Remote targets',
    icon: 'mdi-lan-connect',
  },
  {
    label: i18n.global.t('online'),
    value: String(dataStore.onlines.user.length),
    note: 'Active clients',
    icon: 'mdi-access-point',
  },
])

const selectedPanelLabel = computed(() =>
  `${fixedTileItems.length} live panels`
)

const runtimeHost = computed(() =>
  tilesData.value.sys?.hostName || document.location.hostname
)

const hostAddress = computed(() =>
  tilesData.value.sys?.ipv4?.[0] || tilesData.value.sys?.ipv6?.[0] || 'Awaiting runtime status'
)

const telemetryFooter = computed(() =>
  `CPU, memory, disk, swap, traffic, packets, IO, system and core state remain pinned for continuous review.`
)

const runtimeCards = computed(() => [
  {
    label: i18n.global.t('main.info.running'),
    value: tilesData.value.sbd?.running ? i18n.global.t('yes') : i18n.global.t('no'),
    note: tilesData.value.sbd?.running ? 'Core service healthy' : 'Action required',
    tone: tilesData.value.sbd?.running ? 'success' : 'danger',
  },
  {
    label: i18n.global.t('main.info.cpu'),
    value: tilesData.value.sys?.cpuCount ? `${tilesData.value.sys.cpuCount} ${i18n.global.t('main.info.core')}` : '--',
    note: tilesData.value.sys?.cpuType || 'Unknown processor',
    tone: 'neutral',
  },
  {
    label: i18n.global.t('main.info.uptime'),
    value: tilesData.value.sys?.bootTime ? HumanReadable.formatSecond((Date.now() / 1000) - tilesData.value.sys.bootTime) : '--',
    note: tilesData.value.sys?.appVersion ? `v${tilesData.value.sys.appVersion}` : 'Version pending',
    tone: 'neutral',
  },
  {
    label: i18n.global.t('online'),
    value: String(dataStore.onlines.user.length),
    note: `${dataStore.onlines.inbound.length} inbounds · ${dataStore.onlines.outbound.length} outbounds`,
    tone: 'accent',
  },
])

const tileCardClasses = (type: string) => ({
  'tile-card--metric': type.startsWith('g'),
  'tile-card--chart': type.startsWith('h'),
  'tile-card--info': type.startsWith('i'),
})

const reloadData = async () => {
  const request = [...new Set(fixedTileItems.map(r => r.split('-')[1]))]
    .filter(r => !(r === 'sys' && tilesData.value?.sys?.appVersion))
  const data = await HttpUtils.get('api/status',{ r: request.join(',')})
  if (data.success) {
    tilesData.value = mergeTilesData(tilesData.value, data.obj)
  }
}

const loadOverviewStatus = async () => {
  const data = await HttpUtils.get('api/status', { r: 'sys,sbd' })
  if (data.success) {
    tilesData.value = mergeTilesData(tilesData.value, data.obj)
  }
}

const reloadSys = async () => {
  await loadOverviewStatus()
}

let intervalId: ReturnType<typeof setInterval> | null = null

const startTimer = () => {
  if (intervalId) return

  intervalId = setInterval(() => {
    reloadData()
  }, 2000)
}

const stopTimer = () => {
  if (intervalId) {
    clearInterval(intervalId)
    intervalId = null
  }
}

const logModal = ref({ visible: false })

const backupModal = ref({ visible: false })

const usageStatsModal = ref({ visible: false })

const restartSingbox = async () => {
  loading.value = true
  await HttpUtils.post('api/restartSb',{})
  loading.value = false
}

const syncDashboard = async () => {
  await loadOverviewStatus()
  await reloadData()
  startTimer()
}

onMounted(async () => {
  loading.value = true
  await syncDashboard()
  loading.value = false
})

onActivated(() => {
  if (!loading.value) void syncDashboard()
})

onDeactivated(() => {
  stopTimer()
})

onBeforeUnmount(() => {
  stopTimer()
})
</script>

<style scoped>
.dashboard-shell {
  padding: 4px 0 24px;
}

.dashboard-shell__overview,
.dashboard-shell__tiles {
  margin-top: 0;
}

.dashboard-shell__tiles {
  display: flex;
  flex-direction: column;
  gap: 16px;
  margin-top: 16px;
}

.dashboard-shell__overview > .v-col {
  display: flex;
}

.overview-card {
  border-radius: 26px !important;
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  overflow: hidden;
  padding: clamp(16px, 1.35vw, 21px);
  position: relative;
}

.overview-card__glow {
  background:
    radial-gradient(circle at top left, color-mix(in srgb, var(--app-state-danger) 18%, transparent), transparent 30%),
    radial-gradient(circle at bottom right, color-mix(in srgb, var(--app-state-info) 16%, transparent), transparent 34%);
  inset: 0;
  pointer-events: none;
  position: absolute;
}

.overview-card__eyebrow {
  align-items: center;
  color: var(--app-text-3);
  display: inline-flex;
  font-size: 11px;
  gap: 10px;
  font-weight: 700;
  letter-spacing: 0.24em;
  margin-bottom: 12px;
  position: relative;
  text-transform: uppercase;
  z-index: 1;
}

.overview-card__title {
  font-size: clamp(26px, 2.45vw, 36px);
  font-weight: 700;
  letter-spacing: -0.04em;
  line-height: 1;
  margin: 0;
  position: relative;
  z-index: 1;
}

.overview-card__copy {
  color: var(--app-text-2);
  display: -webkit-box;
  font-size: 13px;
  line-height: 1.55;
  margin: 12px 0 0;
  max-width: 36ch;
  overflow: hidden;
  position: relative;
  z-index: 1;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 3;
}

.overview-card__chips,
.overview-card__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  position: relative;
  z-index: 1;
}

.overview-card__chips {
  margin-top: 16px;
}

.overview-card__chips span {
  background: color-mix(in srgb, var(--app-surface-3) 86%, transparent);
  border: 1px solid var(--app-border-1);
  border-radius: 999px;
  color: var(--app-text-1);
  font-size: 12px;
  font-weight: 500;
  padding: 7px 11px;
}

.overview-card__actions {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  margin-top: auto;
  padding-top: 16px;
}

.overview-card__actions .v-btn {
  width: 100%;
}

.section-head {
  align-items: flex-start;
  display: grid;
  gap: 10px;
  grid-template-columns: minmax(0, 1fr) auto;
  margin-bottom: 12px;
  position: relative;
  z-index: 1;
}

.section-head__label {
  color: var(--app-text-3);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.18em;
  text-transform: uppercase;
}

.section-head__title {
  font-size: 15px;
  font-weight: 600;
  line-height: 1.2;
  margin-top: 2px;
}

.section-head__caption {
  color: var(--app-text-3);
  font-size: 12px;
  line-height: 1.5;
  max-width: 16ch;
  text-align: right;
}

.overview-grid {
  display: grid;
  gap: 10px;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  position: relative;
  z-index: 1;
}

.overview-grid__item {
  background: color-mix(in srgb, var(--app-surface-3) 88%, transparent);
  border: 1px solid var(--app-border-1);
  border-radius: 18px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  justify-content: space-between;
  min-height: 112px;
  padding: 12px;
  transition:
    transform 180ms ease,
    border-color 180ms ease,
    background-color 180ms ease;
}

.overview-grid__item:hover,
.runtime-grid__item:hover {
  background: color-mix(in srgb, var(--app-surface-3) 94%, transparent);
  border-color: var(--app-border-2);
  transform: translateY(-2px);
}

.overview-grid__meta {
  align-items: center;
  color: var(--app-text-3);
  display: inline-flex;
  font-size: 11px;
  gap: 7px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.overview-grid__value {
  font-size: clamp(22px, 1.8vw, 28px);
  font-variant-numeric: tabular-nums;
  font-weight: 700;
  line-height: 1;
}

.overview-grid__note {
  color: var(--app-text-3);
  display: -webkit-box;
  font-size: 12px;
  line-height: 1.4;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 1;
}

.runtime-card__address {
  color: var(--app-text-2);
  font-size: 13px;
  line-height: 1.4;
  margin-bottom: 12px;
  position: relative;
  overflow-wrap: anywhere;
  z-index: 1;
}

.runtime-grid {
  display: grid;
  gap: 10px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  position: relative;
  z-index: 1;
}

.runtime-grid__item {
  background: color-mix(in srgb, var(--app-surface-3) 88%, transparent);
  border: 1px solid var(--app-border-1);
  border-radius: 16px;
  display: flex;
  flex-direction: column;
  gap: 5px;
  min-height: 74px;
  padding: 10px 11px;
  transition:
    transform 180ms ease,
    border-color 180ms ease,
    background-color 180ms ease;
}

.runtime-grid__item--success {
  background: color-mix(in srgb, var(--app-state-success) 10%, var(--app-surface-3));
  border-color: color-mix(in srgb, var(--app-state-success) 18%, var(--app-border-1));
}

.runtime-grid__item--danger {
  background: color-mix(in srgb, var(--app-state-danger) 10%, var(--app-surface-3));
  border-color: color-mix(in srgb, var(--app-state-danger) 18%, var(--app-border-1));
}

.runtime-grid__item--accent {
  background: color-mix(in srgb, var(--app-state-info) 10%, var(--app-surface-3));
  border-color: color-mix(in srgb, var(--app-state-info) 18%, var(--app-border-1));
}

.runtime-grid__label {
  color: var(--app-text-3);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.1em;
  text-transform: uppercase;
}

.runtime-grid__value {
  font-size: 18px;
  font-variant-numeric: tabular-nums;
  font-weight: 700;
  line-height: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.runtime-grid__note {
  color: var(--app-text-3);
  display: -webkit-box;
  font-size: 12px;
  line-height: 1.4;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.runtime-card__footer {
  color: var(--app-text-3);
  font-size: 12px;
  line-height: 1.5;
  margin-top: auto;
  min-width: 0;
  overflow-wrap: anywhere;
  padding-top: 14px;
  position: relative;
  z-index: 1;
  margin-bottom: 0;
}

.telemetry-section {
  display: grid;
  gap: 12px;
}

.telemetry-section__head {
  align-items: end;
  display: grid;
  gap: 12px;
  grid-template-columns: minmax(0, 1fr) auto;
}

.telemetry-section__label {
  color: var(--app-text-3);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.16em;
  text-transform: uppercase;
}

.telemetry-section__title {
  color: var(--app-text-1);
  font-size: 16px;
  font-weight: 650;
  line-height: 1.2;
  margin-top: 3px;
}

.telemetry-section__caption {
  color: var(--app-text-3);
  font-size: 12px;
  line-height: 1.5;
  max-width: 32ch;
  text-align: right;
}

.telemetry-grid {
  display: grid;
  gap: 16px;
}

.telemetry-grid--metric {
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
}

.telemetry-grid--detail {
  grid-template-columns: repeat(auto-fit, minmax(360px, 1fr));
}

.tile-card {
  backdrop-filter: blur(18px);
  background:
    linear-gradient(180deg, color-mix(in srgb, #ffffff 3%, transparent), color-mix(in srgb, #ffffff 1%, transparent)),
    color-mix(in srgb, var(--app-surface-2) 96%, transparent) !important;
  border: 1px solid var(--app-border-1);
  border-radius: 24px !important;
  height: 100%;
  overflow: hidden;
  padding-bottom: 2px;
  position: relative;
}

.tile-card--metric {
  min-height: 220px;
}

.tile-card__title {
  align-items: center;
  display: flex;
  justify-content: space-between;
  padding: 14px 16px 8px;
}

.tile-card__title-text {
  font-size: 14px;
  font-weight: 600;
  letter-spacing: 0.02em;
  line-height: 1.2;
  overflow-wrap: anywhere;
}

.tile-card__controls {
  align-items: center;
  display: flex;
  gap: 4px;
}

.tile-card__sheen {
  background: linear-gradient(135deg, color-mix(in srgb, #ffffff 5%, transparent), transparent 40%);
  inset: 0;
  pointer-events: none;
  position: absolute;
}

.tile-card__body {
  min-height: 196px;
  padding: 0 16px 14px;
  position: relative;
  z-index: 1;
}

.tile-card__body--metric {
  align-items: center;
  display: flex;
  justify-content: center;
  min-height: 154px;
}

.tile-card--metric .tile-card__title {
  padding-bottom: 2px;
}

.tile-card--chart .tile-card__body {
  min-height: 188px;
}

.tile-card--info .tile-card__body {
  min-height: auto;
}

.tile-info-grid {
  align-items: start;
  row-gap: 6px;
}

.tile-info-label {
  color: var(--app-text-3);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.05em;
  text-transform: uppercase;
}

.tile-info-value {
  font-size: 13px;
  line-height: 1.45;
  overflow-wrap: anywhere;
}

.tile-info-chip-wrap {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.tile-info-chip {
  max-width: 100%;
}

.tile-card :deep(canvas) {
  height: 100% !important;
  max-width: 100%;
}

@media (max-width: 960px) {
  .dashboard-shell {
    padding: 0 0 18px;
  }

  .overview-card {
    padding: 18px;
  }

  .overview-grid,
  .runtime-grid {
    grid-template-columns: 1fr 1fr;
  }

  .overview-card__actions {
    grid-template-columns: 1fr;
  }

  .tile-card__body {
    min-height: 188px;
  }
}

@media (max-width: 720px) {
  .section-head {
    grid-template-columns: 1fr;
  }

  .section-head__caption {
    max-width: none;
    text-align: left;
  }

  .telemetry-section__head {
    grid-template-columns: 1fr;
  }

  .telemetry-section__caption {
    max-width: none;
    text-align: left;
  }

  .overview-grid,
  .runtime-grid {
    grid-template-columns: 1fr;
  }

  .overview-card__copy {
    max-width: none;
  }

  .telemetry-grid {
    grid-template-columns: 1fr;
  }
}
</style>
