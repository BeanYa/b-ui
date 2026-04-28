<template>
  <div class="dashboard-root">
    <LogVue v-model="logModal.visible" :control="logModal" :visible="logModal.visible" />
    <Backup v-model="backupModal.visible" :control="backupModal" :visible="backupModal.visible" />
    <UsageStats v-model:visible="usageStatsModal.visible" />
    <v-dialog
      v-model="panelUpdateDialog.visible"
      max-width="720"
      :persistent="panelUpdateDialog.mode === 'progress'"
      :scrim="panelUpdateDialog.mode === 'progress' ? 'rgba(7, 12, 20, 0.72)' : true"
    >
      <v-card class="rounded-lg">
        <v-card-title>{{ $t('main.updatePanel.title') }}</v-card-title>
        <v-card-text>
          <div v-if="panelUpdateDialog.loading" class="panel-update__status">
            <v-progress-circular indeterminate color="primary" />
          </div>
          <template v-else>
            <div class="panel-update__versions">
              <div class="panel-update__version-row">
                <span>{{ $t('main.updatePanel.currentVersion') }}</span>
                <strong>{{ panelUpdateDialog.info?.currentVersion || '--' }}</strong>
              </div>
              <div class="panel-update__version-row">
                <span>{{ $t('main.updatePanel.latestVersion') }}</span>
                <strong>{{ panelUpdateDialog.info?.latestVersion || '--' }}</strong>
              </div>
            </div>
            <p class="panel-update__copy">{{ panelUpdateBody }}</p>
            <div v-if="panelUpdateDialog.mode === 'progress'" class="panel-update__progress">
              <v-progress-linear indeterminate color="primary" rounded />
              <p class="panel-update__copy panel-update__copy--muted">
                {{ $t('main.updatePanel.progressHint') }}
              </p>
              <p v-if="panelUpdateStepLabel" class="panel-update__step">
                <span class="panel-update__step-dot"></span>
                <span>{{ panelUpdateStepLabel }}</span>
              </p>
              <div class="panel-update__log-shell">
                <div class="panel-update__log-title">{{ $t('main.updatePanel.logFile') }}</div>
                <pre ref="panelUpdateLogPre" class="panel-update__log">{{ panelUpdateLogOutput }}</pre>
              </div>
            </div>
            <p
              v-if="panelUpdateDialog.info?.updateState?.logPath && panelUpdateDialog.info?.updateState?.phase === 'failed'"
              class="panel-update__copy panel-update__copy--muted"
            >
              {{ $t('main.updatePanel.logPath') }} {{ panelUpdateDialog.info?.updateState?.logPath }}
            </p>
            <div
              v-if="panelUpdateDialog.mode !== 'progress' && panelUpdateDialog.info?.updateState"
              class="panel-update__log-shell"
            >
              <div class="panel-update__log-title">{{ $t('main.updatePanel.logFile') }}</div>
              <pre ref="panelUpdateLogPre" class="panel-update__log">{{ panelUpdateLogOutput }}</pre>
            </div>
          </template>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn
            v-if="panelUpdateDialog.mode === 'confirm'"
            variant="outlined"
            @click="closePanelUpdateDialog">
            {{ $t('actions.close') }}
          </v-btn>
          <v-btn
            v-if="panelUpdateDialog.mode === 'confirm' && panelUpdateActionEnabled"
            color="primary"
            variant="tonal"
            :loading="panelUpdateDialog.submitting"
            @click="confirmPanelUpdate">
            {{ panelUpdateActionLabel }}
          </v-btn>
          <v-btn
            v-else-if="panelUpdateDialog.mode === 'progress'"
            color="primary"
            variant="tonal"
            disabled>
            {{ $t('main.updatePanel.progress') }}
          </v-btn>
          <v-btn
            v-else-if="panelUpdateDialog.mode === 'completed'"
            color="primary"
            prepend-icon="mdi-refresh"
            variant="tonal"
            @click="refreshPanelPage">
            {{ $t('actions.refresh') }}
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-container class="dashboard-shell" fluid>
      <v-row class="dashboard-shell__overview" density="comfortable">
        <v-col cols="12" xl="3" lg="5" class="dashboard-shell__overview-col dashboard-shell__overview-col--intro">
          <v-card class="overview-card overview-card--intro">
            <div class="overview-card__glow"></div>
            <div class="overview-card__eyebrow">Operations Console</div>
            <h1 class="overview-card__title">{{ $t('pages.home') }}</h1>
            <p class="overview-card__copy">
              Operate the control plane from one segmented workspace with stable runtime telemetry, quick interventions,
              and a chart deck below for deeper inspection.
            </p>
            <div class="overview-card__chips">
              <span>{{ hostAddress }}</span>
              <span>{{ selectedPanelLabel }}</span>
            </div>
            <div class="overview-card__actions">
              <v-btn
                color="primary"
                prepend-icon="mdi-refresh"
                variant="flat"
                :loading="panelUpdateButtonLoading"
                @click="openPanelUpdateDialog()">
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

        <v-col cols="12" xl="4" lg="7" class="dashboard-shell__overview-col dashboard-shell__overview-col--stats">
          <v-card class="overview-card overview-card--stats">
            <div class="section-head">
              <div>
                <div class="section-head__label">Control Map</div>
                <div class="section-head__title">Service footprint</div>
              </div>
              <div class="section-head__caption">Current objects and live operators in the plane.</div>
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

        <v-col cols="12" xl="5" lg="12" class="dashboard-shell__overview-col dashboard-shell__overview-col--probe">
          <v-card class="probe-card">
            <div class="probe-card__sheen"></div>
            <div class="probe-card__head">
              <div>
                <div class="probe-card__label">Server Probe</div>
                <div class="probe-card__title">{{ runtimeHost }}</div>
              </div>
              <div class="probe-card__head-actions">
                <span class="probe-card__status" :class="{ 'probe-card__status--danger': !isRuntimeHealthy }">
                  <span class="probe-card__status-dot"></span>
                  {{ isRuntimeHealthy ? 'Healthy' : 'Attention' }}
                </span>
                <v-btn density="comfortable" icon="mdi-refresh" size="small" variant="text" @click="syncDashboard()" />
              </div>
            </div>

            <div class="probe-card__rings">
              <div class="probe-ring" v-for="ring in probeRings" :key="ring.label" :style="ring.style">
                <div class="probe-ring__inner">
                  <div class="probe-ring__label">{{ ring.label }}</div>
                  <div class="probe-ring__value">{{ ring.value }}</div>
                  <div class="probe-ring__note">{{ ring.note }}</div>
                </div>
              </div>
            </div>

            <div class="probe-card__streams">
              <div class="probe-stream" v-for="stream in probeStreams" :key="stream.label">
                <div class="probe-stream__meta">
                  <span>{{ stream.label }}</span>
                  <strong>{{ stream.value }}</strong>
                </div>
                <div class="probe-stream__track">
                  <span class="probe-stream__fill" :style="{ width: `${stream.percent}%` }"></span>
                </div>
              </div>
            </div>

            <div class="probe-card__clusters">
              <div class="probe-cluster">
                <div class="probe-cluster__label">System</div>
                <div class="probe-cluster__facts">
                  <div
                    v-for="item in systemFacts"
                    :key="item.label"
                    class="probe-cluster__fact"
                  >
                    <span>{{ item.label }}</span>
                    <strong>{{ item.value }}</strong>
                  </div>
                </div>
              </div>
              <div class="probe-cluster">
                <div class="probe-cluster__label">Runtime</div>
                <div class="probe-cluster__value">{{ runtimeStateLabel }}</div>
                <div class="probe-cluster__meta">
                  <span>{{ runtimeMemory }}</span>
                  <span>{{ runtimeThreads }}</span>
                  <span>{{ onlineSummary }}</span>
                </div>
              </div>
            </div>
          </v-card>
        </v-col>
      </v-row>

      <section class="dashboard-shell__tiles">
        <div
          v-for="section in visibleTileSections"
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
              v-for="itemKey in section.items"
              :key="itemKey"
              class="tile-card"
              :class="tileCardClasses(itemKey)"
              height="100%"
            >
              <div class="tile-card__sheen"></div>
              <v-card-title class="tile-card__title">
                <div class="tile-card__title-text">{{ tileTitleMap[itemKey] }}</div>
                <div class="tile-card__controls">
                  <v-btn
                    v-if="itemKey === 'h-net'"
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
              <v-card-text :class="['tile-card__body', { 'tile-card__body--metric': itemKey.startsWith('g') }]">
                <Gauge v-if="itemKey.startsWith('g')" :tilesData="tilesData" :type="itemKey" />
                <History v-if="itemKey.startsWith('h')" :tilesData="tilesData" :type="itemKey" />
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
import { computed, nextTick, onActivated, onBeforeUnmount, onDeactivated, onMounted, ref, watch } from 'vue'
import { i18n } from '@/locales'
import { push } from 'notivue'
import LogVue from '@/layouts/modals/Logs.vue'
import Backup from '@/layouts/modals/Backup.vue'
import UsageStats from '@/layouts/modals/UsageStats.vue'
import { splitTileItemsByLayout } from '@/features/dashboard/layout'
import { mergeTilesData } from '@/features/dashboard/persistence'
import { filterTileSectionsByData, formatAppVersion, formatCpuRingNote } from '@/features/dashboard/probe'
import { buildPanelUpdateProgressLines, panelUpdateCompletionMessage } from '@/features/panelUpdate/status'

const loading = ref(false)
type TileSectionKey = 'metric' | 'detail'
type PanelUpdateDialogMode = 'confirm' | 'progress' | 'completed'
type PanelUpdateComparison = 'older' | 'same' | 'newer' | 'unknown'

interface PanelUpdateState {
  phase: 'running' | 'completed' | 'failed'
  targetVersion: string
  force: boolean
  startedAt: number
  updatedAt: number
  logPath?: string
  logText?: string
  message?: string
}

interface PanelUpdateInfo {
  supported: boolean
  unsupportedReason?: string
  currentVersion: string
  latestVersion?: string
  comparison: PanelUpdateComparison
  updateAvailable: boolean
  forceRequired: boolean
  updateState?: PanelUpdateState
}

const requestTileItems = [
  'g-cpu',
  'g-mem',
  'g-dsk',
  'g-swp',
  'h-net',
  'hp-net',
  'h-dio',
  'h-cpu',
  'h-mem',
  'i-sys',
  'i-sbd',
]

const tileCatalog = [
  { title: i18n.global.t('main.gauge.swp'), value: 'g-swp' },
  { title: i18n.global.t('main.chart.net'), value: 'h-net' },
  { title: i18n.global.t('main.chart.pnet'), value: 'hp-net' },
  { title: i18n.global.t('main.chart.dio'), value: 'h-dio' },
  { title: i18n.global.t('main.chart.cpu'), value: 'h-cpu' },
  { title: i18n.global.t('main.chart.mem'), value: 'h-mem' },
]

const tileTitleMap = Object.fromEntries(tileCatalog.map(item => [item.value, item.title]))
const tileLayoutGroups = splitTileItemsByLayout(tileCatalog.map(item => item.value))
const tileSections: Array<{ key: TileSectionKey, items: string[] }> = [
  { key: 'metric', items: tileLayoutGroups.metric },
  { key: 'detail', items: tileLayoutGroups.detail },
].filter(section => section.items.length > 0) as Array<{ key: TileSectionKey, items: string[] }>
const visibleTileSections = computed(() => filterTileSectionsByData(tileSections, tilesData.value))

const tileSectionMeta = {
  metric: {
    label: 'Reserve Layer',
    title: 'Capacity watch',
    caption: 'Secondary capacity that does not need permanent prominence in the probe card.',
  },
  detail: {
    label: 'Signal Layer',
    title: 'Traffic and historic telemetry',
    caption: 'Continuous curves for traffic, disk motion, CPU, and memory history.',
  },
} as const

const tilesData = ref<any>({})
const previousTelemetry = ref<any>({})
const liveActivity = ref({
  network: 0,
  packets: 0,
  disk: 0,
})
const panelUpdateDialog = ref({
  visible: false,
  loading: false,
  submitting: false,
  mode: 'confirm' as PanelUpdateDialogMode,
  info: null as PanelUpdateInfo | null,
  targetVersion: '',
  force: false,
})

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

const runtimeHost = computed(() => tilesData.value.sys?.hostName || document.location.hostname)
const hostAddress = computed(() =>
  tilesData.value.sys?.ipv4?.[0] || tilesData.value.sys?.ipv6?.[0] || 'Awaiting runtime status'
)
const appVersion = computed(() => formatAppVersion(tilesData.value.sys?.appVersion))
const systemUptime = computed(() =>
  tilesData.value.sys?.bootTime ? HumanReadable.formatSecond((Date.now() / 1000) - tilesData.value.sys.bootTime) : '--'
)
const runtimeStateLabel = computed(() => tilesData.value.sbd?.running ? 'Sing-box running' : 'Runtime unavailable')
const runtimeMemory = computed(() =>
  tilesData.value.sbd?.stats?.Alloc ? HumanReadable.sizeFormat(tilesData.value.sbd.stats.Alloc) : 'Memory pending'
)
const runtimeThreads = computed(() =>
  tilesData.value.sbd?.stats?.NumGoroutine ? `${tilesData.value.sbd.stats.NumGoroutine} goroutines` : 'Threads pending'
)
const cpuDescriptor = computed(() => {
  if (!tilesData.value.sys?.cpuCount) return 'Processor pending'
  return `${tilesData.value.sys.cpuCount} ${i18n.global.t('main.info.core')} · ${tilesData.value.sys?.cpuType || 'Unknown CPU'}`
})
const systemFacts = computed(() => [
  { label: 'IP', value: hostAddress.value },
  { label: 'CPU', value: cpuDescriptor.value },
  { label: 'Uptime', value: systemUptime.value },
  { label: 'Version', value: appVersion.value },
])
const onlineSummary = computed(() =>
  `${dataStore.onlines.user.length} users · ${dataStore.onlines.inbound.length} inbounds · ${dataStore.onlines.outbound.length} outbounds`
)
const selectedPanelLabel = computed(() => `${requestTileItems.length} live collectors`)
const isRuntimeHealthy = computed(() => Boolean(tilesData.value.sbd?.running))

const usagePercent = (section: any) => {
  if (!section?.total) return 0
  return Math.max(0, Math.min(100, Math.round((section.current * 100) / section.total)))
}

const scalePercent = (value: number, ceiling: number) => Math.max(0, Math.min(100, Math.round((value / ceiling) * 100)))

const networkSpeed = computed(() => liveActivity.value.network)
const packetRate = computed(() => liveActivity.value.packets)
const diskIoRate = computed(() => liveActivity.value.disk)

const probeRings = computed(() => [
  {
    label: 'CPU',
    value: `${Math.round(tilesData.value.cpu || 0)}%`,
    note: formatCpuRingNote(tilesData.value.sys?.cpuCount, i18n.global.t('main.info.core')),
    style: {
      '--ring-percent': `${Math.max(0, Math.min(100, Math.round(tilesData.value.cpu || 0)))}%`,
      '--ring-color': 'var(--app-state-info)',
    },
  },
  {
    label: 'RAM',
    value: `${usagePercent(tilesData.value.mem)}%`,
    note: tilesData.value.mem?.total
      ? `${HumanReadable.sizeFormat(tilesData.value.mem.current)} / ${HumanReadable.sizeFormat(tilesData.value.mem.total)}`
      : 'Memory pending',
    style: {
      '--ring-percent': `${usagePercent(tilesData.value.mem)}%`,
      '--ring-color': 'var(--app-state-success)',
    },
  },
  {
    label: 'Disk',
    value: `${usagePercent(tilesData.value.dsk)}%`,
    note: tilesData.value.dsk?.total
      ? `${HumanReadable.sizeFormat(tilesData.value.dsk.current)} / ${HumanReadable.sizeFormat(tilesData.value.dsk.total)}`
      : 'Storage pending',
    style: {
      '--ring-percent': `${usagePercent(tilesData.value.dsk)}%`,
      '--ring-color': 'var(--app-state-warning)',
    },
  },
])

const probeStreams = computed(() => [
  {
    label: 'Network',
    percent: scalePercent(networkSpeed.value, 8 * 1024 * 1024),
    value: networkSpeed.value > 0 ? `${HumanReadable.sizeFormat(networkSpeed.value)}/s` : '--',
  },
  {
    label: 'Packets',
    percent: scalePercent(packetRate.value, 5000),
    value: packetRate.value > 0 ? `${HumanReadable.packetFormat(packetRate.value)}/s` : '--',
  },
  {
    label: 'Disk I/O',
    percent: scalePercent(diskIoRate.value, 24 * 1024 * 1024),
    value: diskIoRate.value > 0 ? `${HumanReadable.sizeFormat(diskIoRate.value)}/s` : '--',
  },
])

const tileCardClasses = (type: string) => ({
  'tile-card--metric': type.startsWith('g'),
  'tile-card--chart': type.startsWith('h'),
})

const panelUpdateActionEnabled = computed(() => {
  return Boolean(panelUpdateDialog.value.info?.supported && panelUpdateDialog.value.targetVersion)
})

const panelUpdateActionLabel = computed(() => {
  return panelUpdateDialog.value.force
    ? i18n.global.t('actions.forceUpdate').toString()
    : i18n.global.t('actions.update').toString()
})

const panelUpdateButtonLoading = computed(() => {
  return panelUpdateDialog.value.loading || panelUpdateDialog.value.submitting
})

const panelUpdateProgressLines = computed(() => buildPanelUpdateProgressLines(
  {
    targetVersion: panelUpdateDialog.value.info?.updateState?.targetVersion || panelUpdateDialog.value.targetVersion,
    logPath: panelUpdateDialog.value.info?.updateState?.logPath,
    logText: panelUpdateDialog.value.info?.updateState?.logText,
  },
  {
    targetVersion: i18n.global.t('main.updatePanel.targetVersion').toString(),
    logPath: i18n.global.t('main.updatePanel.logFile').toString(),
  },
))

const panelUpdateLogOutput = computed(() => {
  if (panelUpdateProgressLines.value.length > 0) {
    return panelUpdateProgressLines.value.join('\n')
  }
  return i18n.global.t('main.updatePanel.logPending').toString()
})

const panelUpdateLogPre = ref<HTMLElement | null>(null)

watch(panelUpdateLogOutput, async () => {
  await nextTick()
  if (panelUpdateLogPre.value) {
    panelUpdateLogPre.value.scrollTo({
      top: panelUpdateLogPre.value.scrollHeight,
      behavior: 'smooth',
    })
  }
})

const panelUpdateStepLabel = computed(() => {
  const message = panelUpdateDialog.value.info?.updateState?.message
  if (!message) return ''
  const labels: Record<string, string> = {
    download_install_script: i18n.global.t('main.updatePanel.steps.downloading').toString(),
    execute_install_script: i18n.global.t('main.updatePanel.steps.installing').toString(),
  }
  return labels[message] || ''
})

const panelUpdateCompletedText = computed(() => {
  const targetVersion = panelUpdateDialog.value.info?.updateState?.targetVersion || panelUpdateDialog.value.targetVersion
  if (!targetVersion) {
    return i18n.global.t('main.updatePanel.completed').toString()
  }
  return i18n.global.t('main.updatePanel.completedWithRefresh', { version: targetVersion }).toString()
    || panelUpdateCompletionMessage(targetVersion)
})

const panelUpdateBody = computed(() => {
  const info = panelUpdateDialog.value.info
  if (!info) return ''
  if (!info.supported) {
    return info.unsupportedReason || i18n.global.t('main.updatePanel.unsupported').toString()
  }
  if (info.updateState?.phase === 'failed') {
    return i18n.global.t('main.updatePanel.failed').toString()
  }
  if (panelUpdateDialog.value.mode === 'completed') {
    return panelUpdateCompletedText.value
  }
  if (panelUpdateDialog.value.mode === 'progress') {
    return i18n.global.t('main.updatePanel.running').toString()
  }

  switch (info.comparison) {
    case 'older':
      return i18n.global.t('main.updatePanel.confirmUpdate').toString()
    case 'same':
      return i18n.global.t('main.updatePanel.confirmForce').toString()
    case 'newer':
      return i18n.global.t('main.updatePanel.confirmDowngrade').toString()
    default:
      return i18n.global.t('main.updatePanel.confirmUnknown').toString()
  }
})

const updateActivity = (nextData: any) => {
  const prev = previousTelemetry.value

  if (prev.net && nextData.net) {
    const bytesPerSecond = Math.max(0, ((nextData.net.recv - prev.net.recv) + (nextData.net.sent - prev.net.sent)) / 2)
    const packetsPerSecond = Math.max(0, ((nextData.net.precv - prev.net.precv) + (nextData.net.psent - prev.net.psent)) / 2)
    liveActivity.value.network = bytesPerSecond
    liveActivity.value.packets = packetsPerSecond
  }

  if (prev.dio && nextData.dio) {
    const diskBytesPerSecond = Math.max(0, ((nextData.dio.read - prev.dio.read) + (nextData.dio.write - prev.dio.write)) / 2)
    liveActivity.value.disk = diskBytesPerSecond
  }

  previousTelemetry.value = {
    net: nextData.net ?? prev.net,
    dio: nextData.dio ?? prev.dio,
  }
}

const reloadData = async () => {
  const request = [...new Set(requestTileItems.map(item => item.split('-')[1]))]
  const data = await HttpUtils.get('api/status', { r: request.join(',') })
  if (data.success) {
    updateActivity(data.obj)
    tilesData.value = mergeTilesData(tilesData.value, data.obj)
  }
}

let intervalId: ReturnType<typeof setInterval> | null = null

const startTimer = () => {
  if (intervalId) return

  intervalId = setInterval(() => {
    void reloadData()
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
let panelUpdatePollId: ReturnType<typeof setInterval> | null = null

const restartSingbox = async () => {
  loading.value = true
  await HttpUtils.post('api/restartSb', {})
  loading.value = false
}

const panelUpdateNeedsForce = (info: PanelUpdateInfo | null) => {
  return info?.comparison !== 'older'
}

const applyPanelUpdateInfo = (info: PanelUpdateInfo) => {
  panelUpdateDialog.value.info = info
  panelUpdateDialog.value.targetVersion = info.latestVersion ?? panelUpdateDialog.value.targetVersion
  panelUpdateDialog.value.force = panelUpdateNeedsForce(info)
}

const closePanelUpdateDialog = () => {
  if (panelUpdateDialog.value.mode === 'progress') return
  panelUpdateDialog.value.visible = false
}

const readPanelUpdateInfoQuietly = async (): Promise<PanelUpdateInfo | null> => {
  try {
    const response = await fetch('./api/panelUpdate', {
      headers: { 'X-Requested-With': 'XMLHttpRequest' },
      credentials: 'same-origin',
    })
    if (!response.ok) return null
    const data = await response.json() as { success?: boolean, obj?: PanelUpdateInfo }
    if (!data?.success || !data.obj) return null
    return data.obj
  } catch {
    return null
  }
}

const stopPanelUpdatePolling = () => {
  if (panelUpdatePollId) {
    clearInterval(panelUpdatePollId)
    panelUpdatePollId = null
  }
}

const handlePanelUpdateCompletion = async (info: PanelUpdateInfo, targetVersion: string) => {
  if (info.updateState?.phase !== 'completed') return

  stopPanelUpdatePolling()
  panelUpdateDialog.value.visible = true
  panelUpdateDialog.value.mode = 'completed'
  await syncDashboard()
  push.success({
    message: panelUpdateCompletedText.value,
  })
}

const refreshPanelPage = () => {
  window.location.reload()
}

const pollPanelUpdate = async (targetVersion: string) => {
  const info = await readPanelUpdateInfoQuietly()
  if (!info) return

  applyPanelUpdateInfo(info)
  if (info.updateState?.phase === 'failed') {
    stopPanelUpdatePolling()
    panelUpdateDialog.value.mode = 'confirm'
    startTimer()
    push.error({
      title: i18n.global.t('failed').toString(),
      message: i18n.global.t('main.updatePanel.failed').toString(),
    })
    return
  }

  await handlePanelUpdateCompletion(info, targetVersion)
}

const startPanelUpdatePolling = (targetVersion: string) => {
  stopPanelUpdatePolling()
  void pollPanelUpdate(targetVersion)
  panelUpdatePollId = setInterval(() => {
    void pollPanelUpdate(targetVersion)
  }, 2000)
}

const openPanelUpdateDialog = async () => {
  panelUpdateDialog.value.loading = true
  const msg = await HttpUtils.get('api/panelUpdate')
  panelUpdateDialog.value.loading = false
  if (!msg.success || !msg.obj) return

  const info = msg.obj as PanelUpdateInfo
  applyPanelUpdateInfo(info)
  panelUpdateDialog.value.mode = info.updateState?.phase === 'running' ? 'progress' : 'confirm'
  panelUpdateDialog.value.visible = true

  if (panelUpdateDialog.value.mode === 'progress' && panelUpdateDialog.value.targetVersion) {
    stopTimer()
    startPanelUpdatePolling(panelUpdateDialog.value.targetVersion)
  }
}

const confirmPanelUpdate = async () => {
  if (!panelUpdateActionEnabled.value) return

  panelUpdateDialog.value.submitting = true
  const msg = await HttpUtils.post('api/panelUpdate', {
    targetVersion: panelUpdateDialog.value.targetVersion,
    force: panelUpdateDialog.value.force ? 'true' : 'false',
  })
  panelUpdateDialog.value.submitting = false
  if (!msg.success) return

  const startedAt = Math.floor(Date.now() / 1000)
  panelUpdateDialog.value.mode = 'progress'
  panelUpdateDialog.value.info = panelUpdateDialog.value.info
    ? {
        ...panelUpdateDialog.value.info,
        updateState: {
          phase: 'running',
          targetVersion: panelUpdateDialog.value.targetVersion,
          force: panelUpdateDialog.value.force,
          startedAt,
          updatedAt: startedAt,
          logPath: msg.obj?.logPath as string | undefined,
          logText: msg.obj?.logText as string | undefined,
        },
      }
    : panelUpdateDialog.value.info

  stopTimer()
  startPanelUpdatePolling(panelUpdateDialog.value.targetVersion)
}

const resumePanelUpdateIfRunning = async () => {
  const info = await readPanelUpdateInfoQuietly()
  if (info?.updateState?.phase !== 'running') return

  applyPanelUpdateInfo(info)
  panelUpdateDialog.value.mode = 'progress'
  panelUpdateDialog.value.visible = true
  if (panelUpdateDialog.value.targetVersion) {
    stopTimer()
    startPanelUpdatePolling(panelUpdateDialog.value.targetVersion)
  }
}

const syncDashboard = async () => {
  await reloadData()
  startTimer()
}

onMounted(async () => {
  loading.value = true
  await syncDashboard()
  await resumePanelUpdateIfRunning()
  loading.value = false
})

onActivated(() => {
  if (!loading.value) {
    void syncDashboard()
    void resumePanelUpdateIfRunning()
  }
})

onDeactivated(() => {
  stopTimer()
  stopPanelUpdatePolling()
})

onBeforeUnmount(() => {
  stopTimer()
  stopPanelUpdatePolling()
})
</script>

<style scoped>
.dashboard-shell {
  padding: 4px 0 24px;
}

.dashboard-shell__overview {
  display: grid;
  gap: 16px;
  grid-template-areas: 'intro stats probe';
  grid-template-columns: minmax(280px, 3fr) minmax(360px, 4fr) minmax(420px, 5fr);
  margin: 0;
}

.dashboard-shell__tiles {
  display: flex;
  flex-direction: column;
  gap: 16px;
  margin-top: 16px;
}

.dashboard-shell__overview > .v-col,
.dashboard-shell__overview-col {
  display: flex;
  max-width: none;
  padding: 0;
  width: auto;
}

.dashboard-shell__overview-col--intro,
.dashboard-shell__overview-col--stats,
.dashboard-shell__overview-col--probe {
  min-width: 0;
}

.dashboard-shell__overview-col--intro {
  grid-area: intro;
}

.dashboard-shell__overview-col--stats {
  grid-area: stats;
}

.dashboard-shell__overview-col--probe {
  grid-area: probe;
}

.dashboard-shell__overview-col > * {
  flex: 1 1 auto;
  min-width: 0;
  width: 100%;
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
  color: var(--app-text-3);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.24em;
  margin-bottom: 12px;
  position: relative;
  text-transform: uppercase;
  z-index: 1;
}

.overview-card__title {
  font-size: clamp(28px, 2.6vw, 38px);
  font-weight: 600;
  letter-spacing: -0.03em;
  line-height: 1;
  margin: 0;
  position: relative;
  z-index: 1;
}

.overview-card__copy {
  color: var(--app-text-2);
  font-size: 13px;
  line-height: 1.58;
  margin: 12px 0 0;
  max-width: 36ch;
  position: relative;
  z-index: 1;
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
  margin-top: 16px;
  padding-top: 16px;
}

.overview-card__actions .v-btn {
  width: 100%;
}

.panel-update__status {
  display: flex;
  justify-content: center;
  padding: 20px 0 8px;
}

.panel-update__versions {
  border: 1px solid var(--app-border-1);
  border-radius: 18px;
  display: grid;
  gap: 10px;
  padding: 14px;
}

.panel-update__version-row {
  align-items: center;
  display: flex;
  font-size: 13px;
  gap: 12px;
  justify-content: space-between;
}

.panel-update__version-row span {
  color: var(--app-text-3);
}

.panel-update__copy {
  color: var(--app-text-1);
  font-size: 13px;
  line-height: 1.6;
  margin: 14px 0 0;
}

.panel-update__copy--muted {
  color: var(--app-text-3);
}

.panel-update__progress {
  display: grid;
  gap: 10px;
  margin-top: 14px;
}

.panel-update__progress .panel-update__copy {
  margin-top: 0;
}

.panel-update__step {
  align-items: center;
  color: #b0d4f0;
  display: flex;
  font-size: 12px;
  font-weight: 500;
  gap: 8px;
  line-height: 1.4;
  margin: 0;
}

.panel-update__step-dot {
  animation: panel-update-step-pulse 1.6s ease-in-out infinite;
  background: #60c0e8;
  border-radius: 50%;
  box-shadow: 0 0 6px color-mix(in srgb, #60c0e8 48%, transparent);
  flex-shrink: 0;
  height: 8px;
  width: 8px;
}

@keyframes panel-update-step-pulse {
  0%, 100% {
    opacity: 0.35;
    transform: scale(0.75);
  }
  50% {
    opacity: 1;
    transform: scale(1);
  }
}

.panel-update__details {
  color: var(--app-text-2);
  display: grid;
  font-size: 12px;
  gap: 4px;
  line-height: 1.5;
  margin: 0;
  padding-left: 18px;
  word-break: break-all;
}

.panel-update__log-shell {
  background: color-mix(in srgb, #05070c 92%, var(--app-surface-3));
  border: 1px solid color-mix(in srgb, var(--app-border-1) 72%, #ffffff 12%);
  border-radius: 14px;
  color: #d8e4f0;
  margin-top: 10px;
  overflow: hidden;
}

.panel-update__log-title {
  align-items: center;
  border-bottom: 1px solid color-mix(in srgb, #ffffff 9%, transparent);
  color: #8fb0c9;
  display: flex;
  font-size: 11px;
  font-weight: 700;
  justify-content: space-between;
  letter-spacing: 0;
  min-height: 32px;
  padding: 0 12px;
}

.panel-update__log {
  font-family: 'Geist Mono Variable', monospace;
  font-size: 12px;
  line-height: 1.55;
  margin: 0;
  max-height: 260px;
  overflow: auto;
  padding: 12px;
  white-space: pre-wrap;
  word-break: break-word;
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

.section-head__label,
.telemetry-section__label,
.probe-card__label {
  color: var(--app-text-3);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.18em;
  overflow: hidden;
  text-overflow: ellipsis;
  text-transform: uppercase;
  white-space: nowrap;
}

.section-head__title,
.telemetry-section__title {
  font-size: 16px;
  font-weight: 600;
  line-height: 1.2;
  margin-top: 2px;
}

.section-head__caption,
.telemetry-section__caption {
  color: var(--app-text-3);
  font-size: 12px;
  line-height: 1.5;
  max-width: 24ch;
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
  min-height: 112px;
  padding: 12px;
}

.overview-grid__meta {
  align-items: center;
  color: var(--app-text-3);
  display: inline-flex;
  font-size: 11px;
  flex-wrap: nowrap;
  gap: 7px;
  letter-spacing: 0.08em;
  min-width: 0;
  text-transform: uppercase;
}

.overview-grid__meta span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.overview-grid__value {
  font-size: clamp(22px, 1.8vw, 28px);
  font-variant-numeric: tabular-nums;
  font-weight: 700;
  line-height: 1;
}

.overview-grid__note {
  color: var(--app-text-3);
  font-size: 12px;
  line-height: 1.4;
}

.probe-card {
  border-radius: 28px !important;
  overflow: hidden;
  padding: 20px;
  position: relative;
}

.probe-card__sheen {
  background:
    radial-gradient(circle at top left, color-mix(in srgb, var(--app-state-danger) 14%, transparent), transparent 26%),
    radial-gradient(circle at top right, color-mix(in srgb, var(--app-state-info) 14%, transparent), transparent 24%),
    linear-gradient(130deg, color-mix(in srgb, #ffffff 5%, transparent), transparent 28%);
  inset: 0;
  pointer-events: none;
  position: absolute;
}

.probe-card__head,
.probe-card__rings,
.probe-card__streams,
.probe-card__clusters {
  position: relative;
  z-index: 1;
}

.probe-card__head {
  align-items: start;
  display: grid;
  gap: 10px;
  grid-template-columns: minmax(0, 1fr) auto;
}

.probe-card__title {
  font-size: clamp(22px, 2vw, 28px);
  font-weight: 600;
  line-height: 1.1;
  margin-top: 4px;
}

.probe-card__head-actions {
  align-items: center;
  display: flex;
  gap: 8px;
}

.probe-card__status {
  align-items: center;
  background: color-mix(in srgb, var(--app-state-success) 10%, var(--app-surface-3));
  border: 1px solid color-mix(in srgb, var(--app-state-success) 18%, var(--app-border-1));
  border-radius: 999px;
  color: var(--app-text-1);
  display: inline-flex;
  font-size: 12px;
  font-weight: 600;
  gap: 8px;
  min-height: 34px;
  white-space: nowrap;
  padding: 0 12px;
}

.probe-card__status--danger {
  background: color-mix(in srgb, var(--app-state-danger) 10%, var(--app-surface-3));
  border-color: color-mix(in srgb, var(--app-state-danger) 18%, var(--app-border-1));
}

.probe-card__status-dot {
  background: currentColor;
  border-radius: 999px;
  box-shadow: 0 0 0 4px color-mix(in srgb, currentColor 16%, transparent);
  height: 8px;
  width: 8px;
}

.probe-card__rings {
  display: grid;
  gap: 10px;
  grid-template-columns: repeat(auto-fit, minmax(132px, 1fr));
  margin-top: 16px;
}

.probe-ring {
  align-items: center;
  display: grid;
  justify-items: center;
  position: relative;
}

.probe-ring::before {
  background: conic-gradient(var(--ring-color) var(--ring-percent), color-mix(in srgb, var(--app-surface-4) 100%, transparent) 0);
  border-radius: 50%;
  content: '';
  height: 124px;
  width: 124px;
}

.probe-ring::after {
  background: color-mix(in srgb, var(--app-surface-1) 100%, transparent);
  border: 1px solid var(--app-border-1);
  border-radius: 50%;
  content: '';
  height: 92px;
  position: absolute;
  width: 92px;
}

.probe-ring__inner {
  display: grid;
  gap: 4px;
  inset: 0;
  justify-items: center;
  place-content: center;
  position: absolute;
  text-align: center;
  z-index: 1;
}

.probe-ring__label {
  color: var(--app-text-3);
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.probe-ring__value {
  font-family: 'Geist Mono Variable', monospace;
  font-size: 24px;
  font-weight: 700;
  line-height: 1;
  margin-top: 2px;
}

.probe-ring__note {
  color: var(--app-text-3);
  font-size: 10px;
  line-height: 1.25;
  margin: 0 auto;
  max-width: 100%;
  min-height: 2.5em;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.probe-card__streams {
  display: grid;
  gap: 10px;
  margin-top: 18px;
}

.probe-stream {
  background: color-mix(in srgb, var(--app-surface-3) 88%, transparent);
  border: 1px solid var(--app-border-1);
  border-radius: 16px;
  padding: 10px 12px;
}

.probe-stream__meta {
  align-items: center;
  display: flex;
  font-size: 12px;
  gap: 12px;
  justify-content: space-between;
  margin-bottom: 8px;
}

.probe-stream__meta span,
.probe-stream__meta strong {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.probe-stream__meta strong {
  font-family: 'Geist Mono Variable', monospace;
  font-size: 12px;
}

.probe-stream__track {
  background: color-mix(in srgb, var(--app-surface-4) 100%, transparent);
  border-radius: 999px;
  height: 8px;
  overflow: hidden;
}

.probe-stream__fill {
  background: linear-gradient(90deg, var(--app-state-info), var(--app-state-danger));
  border-radius: 999px;
  display: block;
  height: 100%;
}

.probe-card__clusters {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  margin-top: 18px;
}

.probe-cluster {
  background: color-mix(in srgb, var(--app-surface-3) 88%, transparent);
  border: 1px solid var(--app-border-1);
  border-radius: 18px;
  padding: 12px;
}

.probe-cluster__label {
  color: var(--app-text-3);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.12em;
  overflow: hidden;
  text-overflow: ellipsis;
  text-transform: uppercase;
  white-space: nowrap;
}

.probe-cluster__value {
  font-size: 17px;
  font-weight: 600;
  line-height: 1.2;
  margin-top: 8px;
  overflow-wrap: anywhere;
}

.probe-cluster__facts {
  display: grid;
  gap: 8px;
  grid-template-columns: repeat(auto-fit, minmax(132px, 1fr));
  margin-top: 10px;
}

.probe-cluster__fact {
  background: color-mix(in srgb, var(--app-surface-4) 68%, transparent);
  border: 1px solid color-mix(in srgb, var(--app-border-1) 72%, transparent);
  border-radius: 12px;
  min-width: 0;
  padding: 8px 10px;
}

.probe-cluster__fact span {
  color: var(--app-text-4);
  display: block;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.08em;
  line-height: 1.2;
  overflow: hidden;
  text-overflow: ellipsis;
  text-transform: uppercase;
  white-space: nowrap;
}

.probe-cluster__fact strong {
  color: var(--app-text-1);
  display: block;
  font-size: 12px;
  font-weight: 600;
  line-height: 1.35;
  margin-top: 4px;
  overflow-wrap: anywhere;
}

.probe-cluster__meta {
  color: var(--app-text-3);
  display: flex;
  flex-wrap: wrap;
  font-size: 12px;
  gap: 8px;
  line-height: 1.4;
  margin-top: 8px;
}

.probe-cluster__meta span + span::before {
  color: var(--app-text-4);
  content: '•';
  margin-inline-end: 8px;
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

.tile-card__sheen {
  background: linear-gradient(135deg, color-mix(in srgb, #ffffff 5%, transparent), transparent 40%);
  inset: 0;
  pointer-events: none;
  position: absolute;
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
  line-height: 1.2;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tile-card__controls {
  align-items: center;
  display: flex;
  flex-shrink: 0;
  gap: 4px;
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

.tile-card--chart .tile-card__body {
  aspect-ratio: auto;
  height: clamp(220px, 28vh, 320px);
  min-height: 220px;
  max-height: 320px;
}

.tile-card :deep(canvas) {
  height: 100% !important;
  max-width: 100%;
}

@media (max-width: 1680px) {
  .dashboard-shell__overview {
    grid-template-areas:
      'intro stats'
      'probe probe';
    grid-template-columns: minmax(320px, 2fr) minmax(420px, 3fr);
  }

  .section-head,
  .probe-card__head,
  .telemetry-section__head {
    grid-template-columns: 1fr;
  }

  .section-head__caption,
  .telemetry-section__caption {
    max-width: none;
    text-align: left;
  }

  .overview-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .telemetry-grid--detail {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 1380px) {
  .dashboard-shell__overview {
    grid-template-areas:
      'intro'
      'stats'
      'probe';
    grid-template-columns: minmax(0, 1fr);
  }

  .probe-card__rings {
    grid-template-columns: repeat(auto-fit, minmax(156px, 1fr));
  }

  .probe-ring::before {
    height: 118px;
    width: 118px;
  }

  .probe-ring::after {
    height: 88px;
    width: 88px;
  }
}

@media (max-width: 1280px) {
  .dashboard-shell__overview {
    gap: 14px;
  }

  .overview-card__actions {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .probe-card__rings {
    grid-template-columns: repeat(auto-fit, minmax(138px, 1fr));
  }

  .probe-card__clusters {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .tile-card--chart .tile-card__body {
    height: clamp(208px, 26vh, 288px);
    min-height: 208px;
    max-height: 288px;
  }
}

@media (max-width: 960px) {
  .dashboard-shell {
    padding: 0 0 18px;
  }

  .overview-card,
  .probe-card {
    padding: 18px;
  }

  .overview-grid,
  .probe-card__clusters {
    grid-template-columns: 1fr 1fr;
  }

  .overview-card__actions {
    grid-template-columns: 1fr;
  }

  .probe-card__rings {
    grid-template-columns: repeat(auto-fit, minmax(148px, 1fr));
  }

  .tile-card--chart .tile-card__body {
    height: clamp(196px, 24vh, 248px);
    min-height: 196px;
    max-height: 248px;
  }
}

@media (max-width: 720px) {
  .section-head,
  .telemetry-section__head,
  .probe-card__head {
    grid-template-columns: 1fr;
  }

  .section-head__caption,
  .telemetry-section__caption {
    max-width: none;
    text-align: left;
  }

  .overview-grid,
  .probe-card__clusters,
  .telemetry-grid {
    grid-template-columns: 1fr;
  }

  .overview-card__copy {
    max-width: none;
  }

  .probe-card__rings {
    grid-template-columns: repeat(auto-fit, minmax(132px, 1fr));
  }

  .probe-ring::before {
    height: 112px;
    width: 112px;
  }

  .probe-ring::after {
    height: 84px;
    width: 84px;
  }

  .tile-card--chart .tile-card__body {
    height: auto;
    min-height: 180px;
    max-height: none;
  }
}
</style>
