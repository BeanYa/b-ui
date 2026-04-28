<template>
  <div class="app-page">
    <section class="app-page__hero">
      <div class="app-page__hero-head">
        <div class="app-page__hero-kicker">{{ $t('clusterCenter.heroKicker') }}</div>
        <h1 class="app-page__hero-title">{{ $t('pages.clusterCenter') }}</h1>
        <p class="app-page__hero-copy">
          {{ $t('clusterCenter.heroCopy') }}
        </p>
        <div class="app-page__hero-meta">
          <span class="app-page__hero-meta-item">{{ domains.length }} {{ $t('clusterCenter.domainsTitle').toLowerCase() }}</span>
          <span class="app-page__hero-meta-item">{{ members.length }} {{ $t('clusterCenter.mirroredMembers') }}</span>
          <span class="app-page__hero-meta-item">{{ selectedDomain ? formatClusterVersionLabel(selectedDomain.lastVersion) : $t('clusterCenter.metaNoDomain') }}</span>
        </div>
      </div>
    </section>

    <v-row class="app-page__toolbar">
      <v-col cols="12">
        <div class="app-page__toolbar-actions cluster-center__actions">
          <v-btn color="primary" @click="registerDialog = true">{{ $t('clusterCenter.actions.register') }}</v-btn>
          <v-btn variant="outlined" color="warning" :loading="actionLoading" @click="manualSync">{{ $t('clusterCenter.actions.manualSync') }}</v-btn>
          <v-btn class="cluster-center__refresh-btn" variant="outlined" :loading="pageLoading" @click="loadData">{{ $t('clusterCenter.actions.refresh') }}</v-btn>
        </div>
      </v-col>
    </v-row>

    <template v-if="!selectedDomain">
      <v-card class="cluster-center__domains app-card-shell" :loading="pageLoading">
        <v-card-title>
          <div class="cluster-center__card-title">
            <span>{{ $t('clusterCenter.domainsTitle') }}</span>
            <span class="cluster-center__domain-prompt">{{ $t('clusterCenter.inspectPrompt') }}</span>
          </div>
        </v-card-title>
        <v-card-text>
          <div v-if="domains.length === 0" class="cluster-center__empty">{{ $t('clusterCenter.noDomains') }}</div>
          <div v-else class="cluster-center__domain-list">
            <button
              v-for="domain in domains"
              :key="domain.id"
              type="button"
              class="cluster-center__domain-card"
              @click="openDomainDetail(domain)"
            >
              <div class="cluster-center__domain-head">
                <strong>{{ domain.domain }}</strong>
                <span class="cluster-center__version">{{ formatClusterVersionLabel(domain.lastVersion) }}</span>
              </div>
              <div class="cluster-center__domain-url">{{ domain.hubUrl || $t('clusterCenter.fields.hubUrl') }}</div>
              <div class="cluster-center__domain-meta">{{ domainMemberCount(domain.id) }} {{ $t('clusterCenter.mirroredMembers') }}</div>
            </button>
          </div>
        </v-card-text>
      </v-card>
    </template>

    <section v-else class="cluster-center__detail">
      <div class="cluster-center__detail-actions">
        <v-btn variant="outlined" prepend-icon="mdi-arrow-left" @click="backToClusterCenter">
          {{ $t('clusterCenter.actions.back') }}
        </v-btn>
        <v-btn
          variant="outlined"
          color="error"
          :loading="leavingDomainId === selectedDomain.id"
          @click="leaveDomain(selectedDomain)"
        >
          {{ $t('clusterCenter.actions.leave') }}
        </v-btn>
      </div>

      <v-card class="app-card-shell cluster-center__domain-info" :loading="pageLoading">
        <v-card-title>
          <div class="cluster-center__selected-head">
            <span>{{ selectedDomain.domain }}</span>
            <span class="cluster-center__version cluster-center__selected-version">
              {{ formatClusterVersionLabel(selectedDomain.lastVersion) }}
            </span>
          </div>
        </v-card-title>
        <v-card-text>
          <div class="cluster-center__detail-panel">
            <div class="cluster-center__domain-meta">
              <div class="cluster-center__meta-row">
                <span class="cluster-center__meta-label">{{ $t('clusterCenter.fields.domain') }}</span>
                <strong class="cluster-center__meta-value">{{ selectedDomain.domain }}</strong>
              </div>
              <div class="cluster-center__meta-row">
                <span class="cluster-center__meta-label">{{ $t('clusterCenter.fields.hubUrl') }}</span>
                <strong class="cluster-center__meta-value">{{ selectedDomain.hubUrl || '-' }}</strong>
              </div>
              <div class="cluster-center__meta-row">
                <span class="cluster-center__meta-label">{{ $t('clusterCenter.table.version') }}</span>
                <strong class="cluster-center__meta-value">{{ formatClusterVersionLabel(selectedDomain.lastVersion) }}</strong>
              </div>
              <div class="cluster-center__meta-row">
                <span class="cluster-center__meta-label">{{ $t('clusterCenter.fields.communicationProtocol') }}</span>
                <strong class="cluster-center__meta-value">{{ selectedDomain.communicationProtocolVersion || '-' }}</strong>
              </div>
              <div class="cluster-center__meta-row">
                <span class="cluster-center__meta-label">{{ $t('clusterCenter.fields.communicationEndpoint') }}</span>
                <strong class="cluster-center__meta-value">{{ selectedDomain.communicationEndpointPath || '-' }}</strong>
              </div>
              <div class="cluster-center__meta-row">
                <span class="cluster-center__meta-label">{{ $t('clusterCenter.mirroredMembers') }}</span>
                <strong class="cluster-center__meta-value">{{ selectedDomainMembers.length }}</strong>
              </div>
            </div>

            <div class="cluster-center__actions-tree">
              <span class="cluster-center__meta-label cluster-center__meta-label--header">
                {{ $t('clusterCenter.fields.supportedActions') }}
              </span>
              <ClusterDomainActionTree
                :supported-actions="selectedDomain.supportedActions"
              />
            </div>
          </div>
        </v-card-text>
      </v-card>

      <v-card class="app-card-shell cluster-center__members" :loading="pageLoading">
        <v-card-title>
          <div style="display: flex; align-items: center; gap: 16px;">
            <span>{{ $t('clusterCenter.registeredServers') }}</span>
            <v-btn
              size="small"
              variant="outlined"
              color="primary"
              :loading="meshPingLoading"
              @click="pingAllDomainMembers"
            >
              Ping All
            </v-btn>
          </div>
        </v-card-title>
        <v-card-text>
          <div v-if="selectedDomainMembers.length === 0" class="cluster-center__empty">{{ $t('clusterCenter.noMembers') }}</div>
          <div v-else class="cluster-center__member-table-wrap">
            <table class="cluster-center__member-table">
              <thead>
                <tr>
                  <th>{{ $t('clusterCenter.table.node') }}</th>
                  <th>{{ $t('clusterCenter.table.name') }}</th>
                  <th>{{ $t('clusterCenter.table.baseUrl') }}</th>
                  <th>{{ $t('clusterCenter.table.version') }}</th>
                  <th>{{ $t('clusterCenter.table.panelVersion') }}</th>
                  <th>{{ $t('clusterCenter.table.status') }}</th>
                  <th>{{ $t('clusterCenter.table.latency') }}</th>
                  <th>{{ $t('clusterCenter.table.action') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="member in selectedDomainMembers" :key="member.id">
                  <td>
                    <div class="cluster-center__member-node">
                      <span>{{ member.nodeId }}</span>
                      <span v-if="member.isLocal" class="cluster-center__local-badge">{{ $t('clusterCenter.localNode') }}</span>
                    </div>
                  </td>
                  <td>{{ member.displayName || member.name || '-' }}</td>
                  <td>{{ member.baseUrl || '-' }}</td>
                  <td>{{ formatClusterVersionLabel(member.lastVersion) }}</td>
                  <td><span class="mono-copy">{{ member.panelVersion || '-' }}</span></td>
                  <td>
                    <v-chip
                      :color="member.status === 'offline' ? 'red' : 'green'"
                      size="small"
                      variant="flat"
                    >
                      {{ member.status === 'offline' ? $t('offline') : $t('online') }}
                    </v-chip>
                  </td>
                  <td>
                    <span
                      :style="memberLatencyStyle(member.nodeId)"
                      class="cluster-center__latency-cell"
                    >{{ memberLatency(member.nodeId) }}</span>
                  </td>
                  <td>
                    <div style="display: flex; gap: 8px; align-items: center;">
                      <v-btn
                        v-if="!member.isLocal"
                        size="small"
                        variant="tonal"
                        @click="goToNodeDetail(member)"
                      >
                        管理
                      </v-btn>
                      <v-btn
                        size="small"
                        :color="member.isLocal ? 'error' : 'warning'"
                        variant="outlined"
                        :loading="member.isLocal ? leavingDomainId === selectedDomain?.id : deletingMemberId === member.id"
                        @click="member.isLocal ? leaveDomain(selectedDomain) : deleteMember(member)"
                      >
                        {{ member.isLocal ? $t('clusterCenter.actions.leave') : $t('clusterCenter.actions.delete') }}
                      </v-btn>
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </v-card-text>
      </v-card>
    </section>

    <v-dialog v-model="registerDialog" class="app-dialog app-dialog--compact" max-width="520" @update:model-value="onRegisterDialogClose">
      <v-card class="app-card-shell">
        <v-card-title>{{ $t('clusterCenter.dialogTitle') }}</v-card-title>

        <template v-if="registerStep === 1">
          <v-tabs v-model="registerMode" class="cluster-center__register-tabs" color="primary" grow>
            <v-tab value="uri">URI</v-tab>
            <v-tab value="manual">{{ $t('clusterCenter.domainsTitle') }}</v-tab>
          </v-tabs>

          <v-card-text class="cluster-center__dialog-body">
            <template v-if="registerMode === 'uri'">
              <v-text-field
                v-model="form.joinUri"
                label="Join URI"
                placeholder="buihub://hub.example.com/domain/example.com?domain_token=..."
                persistent-hint
                hint="URI 以 buihub:// 开头，例如 buihub://hub.example.com/domain/example.com?domain_token=xxx"
              />
            </template>
            <template v-else>
              <v-text-field v-model="form.domain" :label="$t('clusterCenter.fields.domain')" hide-details />
              <div class="cluster-center__hub-url-field">
                <v-select
                  v-model="form.hubUrlProtocol"
                  :items="['https', 'http']"
                  variant="plain"
                  hide-details
                  density="compact"
                  class="cluster-center__hub-url-protocol"
                />
                <span class="cluster-center__hub-url-sep">://</span>
                <v-text-field
                  v-model="form.hubUrlHost"
                  :label="$t('clusterCenter.fields.hubUrl')"
                  hide-details
                  class="cluster-center__hub-url-host"
                />
              </div>
              <v-text-field v-model="form.token" :label="$t('clusterCenter.fields.token')" type="password" hide-details />
            </template>
          </v-card-text>
          <v-card-actions>
            <v-spacer />
            <v-btn variant="text" @click="registerDialog = false">{{ $t('clusterCenter.actions.cancel') }}</v-btn>
            <v-btn color="primary" :loading="checkingUrl" @click="validateAndCheckDomain">{{ $t('clusterCenter.actions.submit') }}</v-btn>
          </v-card-actions>
        </template>

        <template v-if="registerStep === 2">
          <v-card-text class="cluster-center__dialog-body">
            <div class="cluster-center__step-indicator">
              <span class="cluster-center__step-label">{{ $t('clusterCenter.stepDomainInfo') }}</span>
              <span class="cluster-center__step-value">{{ confirmInfo.domain }}</span>
            </div>
            <v-text-field
              v-model="form.displayName"
              :label="$t('clusterCenter.displayName')"
              :hint="$t('clusterCenter.displayNameHint')"
              persistent-hint
              hide-details
            />
          </v-card-text>
          <v-card-actions>
            <v-spacer />
            <v-btn variant="text" @click="registerStep = 1">{{ $t('clusterCenter.actions.cancel') }}</v-btn>
            <v-btn color="primary" @click="showConfirmDialog">{{ $t('clusterCenter.actions.submit') }}</v-btn>
          </v-card-actions>
        </template>
      </v-card>
    </v-dialog>

    <v-dialog v-model="confirmDialog" class="app-dialog" max-width="520">
      <v-card class="app-card-shell cluster-center__confirm-card">
        <v-card-title class="cluster-center__confirm-title">{{ $t('clusterCenter.confirmTitle') }}</v-card-title>
        <v-card-text class="cluster-center__confirm-body">
          <div class="cluster-center__confirm-table-wrap">
            <table class="cluster-center__confirm-table">
              <tbody>
                <tr>
                  <td class="cluster-center__confirm-label">Hub 地址</td>
                  <td class="cluster-center__confirm-value">{{ confirmInfo.hubUrl }}</td>
                </tr>
                <tr>
                  <td class="cluster-center__confirm-label">{{ $t('clusterCenter.fields.domain') }}</td>
                  <td class="cluster-center__confirm-value">{{ confirmInfo.domain }}</td>
                </tr>
                <tr>
                  <td class="cluster-center__confirm-label">{{ $t('clusterCenter.fields.token') }}</td>
                  <td class="cluster-center__confirm-value cluster-center__confirm-token">{{ confirmInfo.token }}</td>
                </tr>
                <tr>
                  <td class="cluster-center__confirm-label">{{ $t('clusterCenter.displayName') }}</td>
                  <td class="cluster-center__confirm-value">{{ confirmInfo.displayName }}</td>
                </tr>
                <tr>
                  <td class="cluster-center__confirm-label">本机地址</td>
                  <td class="cluster-center__confirm-value">{{ confirmInfo.baseUrl }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="confirmDialog = false">{{ $t('clusterCenter.actions.cancel') }}</v-btn>
          <v-btn color="primary" :loading="actionLoading" @click="confirmAndSubmit">确认注册</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-dialog v-model="alreadyExistsDialog" class="app-dialog" max-width="460">
      <v-card class="app-card-shell">
        <v-card-title>{{ $t('clusterCenter.alreadyExists') }}</v-card-title>
        <v-card-text>
          <p>{{ $t('clusterCenter.alreadyExistsHint') }}</p>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="alreadyExistsDialog = false">{{ $t('clusterCenter.actions.cancel') }}</v-btn>
          <v-btn color="primary" :loading="actionLoading" @click="pullExistingDomain">{{ $t('clusterCenter.pullDomain') }}</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>

<script lang="ts" setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { push } from 'notivue'

import ClusterDomainActionTree from '@/components/ClusterDomainActionTree.vue'
import { parseClusterHubJoinUri } from '@/features/clusterHubUri'
import { i18n } from '@/locales'
import HttpUtils from '@/plugins/httputil'
import { usePingStore } from '@/store/modules/ping'
import type { ClusterDomain, ClusterMember, ClusterOperationStatus } from '@/types/clusters'
import type { MeshPairResult } from '@/types/ping'

const router = useRouter()

const goToNodeDetail = (member: ClusterMember) => {
  router.push({
    name: 'pages.clusterNodeDetail',
    params: { nodeId: member.nodeId },
    query: { node_id: member.nodeId },
  })
}

const pageLoading = ref(false)
const actionLoading = ref(false)
const registerDialog = ref(false)
const confirmDialog = ref(false)
const registerMode = ref<'uri' | 'manual'>('uri')
const domains = ref<ClusterDomain[]>([])
const members = ref<ClusterMember[]>([])
const selectedDomainId = ref<number | null>(null)
const deletingMemberId = ref<number | null>(null)
const leavingDomainId = ref<number | null>(null)

const form = ref({
  joinUri: '',
  domain: '',
  hubUrlProtocol: 'https',
  hubUrlHost: '',
  token: '',
  displayName: '',
})

const confirmInfo = ref({
  hubUrl: '',
  domain: '',
  token: '',
  baseUrl: '',
  displayName: '',
})

const alreadyExistsDialog = ref(false)
const registerStep = ref(1)
const checkingUrl = ref(false)
const existingDomainData = ref<{ domain: string; hubUrl: string }>({ domain: '', hubUrl: '' })

const selectedDomain = computed(() => domains.value.find((domain) => domain.id === selectedDomainId.value) ?? null)
const selectedDomainMembers = computed(() => members.value.filter((member) => member.domainId === selectedDomainId.value))

const domainMemberCount = (domainId: number) => members.value.filter((member) => member.domainId === domainId).length
const formatClusterVersionLabel = (version: number) => `version-${version}`

const openDomainDetail = (domain: ClusterDomain) => {
  selectedDomainId.value = domain.id
  pingStore.loadMeshResult(domain.domain).then(result => {
    if (result) meshPingResults.value = result.results
  })
}

const backToClusterCenter = () => {
  selectedDomainId.value = null
}

const isUsableAbsoluteUrl = (value: string) => {
  try {
    new URL(value)
    return true
  } catch {
    return false
  }
}

const resolvePanelBaseUrl = () => {
  const rawBaseUrl = String((window as any).BASE_URL ?? '/')
  const normalizedBaseUrl = rawBaseUrl.endsWith('/') ? rawBaseUrl : `${rawBaseUrl}/`

  try {
    return new URL(normalizedBaseUrl, window.location.origin).toString()
  } catch {
    return ''
  }
}

const normalizeClusterBaseUrl = (value: string) => {
  const trimmed = value.trim()
  if (!trimmed) return ''

  try {
    const url = new URL(trimmed)
    url.protocol = url.protocol.toLowerCase()
    url.hostname = url.hostname.toLowerCase()
    url.hash = ''
    url.search = ''
    url.pathname = url.pathname.replace(/\/+$/, '')
    if ((url.protocol === 'https:' && url.port === '443') || (url.protocol === 'http:' && url.port === '80')) {
      url.port = ''
    }
    return url.toString().replace(/\/+$/, '')
  } catch {
    return trimmed.toLowerCase().replace(/\/+$/, '')
  }
}

const deriveDisplayNameFromBaseUrl = (baseUrl: string) => {
  const match = baseUrl.trim().match(/^https?:\/\/([^/:?#]+)(?::\d+)?(?:[/?#]|$)/i)
  return match?.[1]?.toLowerCase() ?? ''
}

const validateAndCheckDomain = async () => {
  if (registerMode.value === 'uri') {
    const uri = form.value.joinUri.trim()
    const parsed = parseClusterHubJoinUri(uri)
    if (!parsed) {
      push.error({ title: i18n.global.t('failed'), message: 'URI 格式无效，请检查后重试' })
      return
    }
    const panelBaseUrl = resolvePanelBaseUrl()
    confirmInfo.value = {
      hubUrl: `${parsed.protocol}://${parsed.host}`,
      domain: parsed.domain,
      token: parsed.token,
      baseUrl: panelBaseUrl,
      displayName: deriveDisplayNameFromBaseUrl(panelBaseUrl),
    }
  } else {
    const domain = form.value.domain.trim()
    const hubUrlHost = form.value.hubUrlHost.trim()
    const hubUrl = `${form.value.hubUrlProtocol}://${hubUrlHost}`

    if (!domain || !hubUrlHost || !form.value.token) {
      push.error({ title: i18n.global.t('failed'), message: i18n.global.t('clusterCenter.validation.required') })
      return
    }
    if (!isUsableAbsoluteUrl(hubUrl)) {
      push.error({ title: i18n.global.t('failed'), message: i18n.global.t('clusterCenter.validation.hubUrl') })
      return
    }
    if (!isUsableAbsoluteUrl(resolvePanelBaseUrl())) {
      push.error({ title: i18n.global.t('failed'), message: i18n.global.t('clusterCenter.validation.panelUrl') })
      return
    }

    const panelBaseUrl = resolvePanelBaseUrl()
    confirmInfo.value = {
      hubUrl,
      domain,
      token: form.value.token,
      baseUrl: panelBaseUrl,
      displayName: deriveDisplayNameFromBaseUrl(panelBaseUrl),
    }
  }

  form.value.displayName = confirmInfo.value.displayName

  await checkPanelUrlExists()
}

const checkPanelUrlExists = async () => {
  checkingUrl.value = true
  const panelBaseUrl = resolvePanelBaseUrl()
  const normalizedPanelBaseUrl = normalizeClusterBaseUrl(panelBaseUrl)

  try {
    const snapshotUrl = `${confirmInfo.value.hubUrl}/v1/domains/${encodeURIComponent(confirmInfo.value.domain)}/snapshot`
    const resp = await fetch(snapshotUrl, {
      headers: { 'X-Domain-Token': confirmInfo.value.token },
    })
    if (resp.ok) {
      const snapshot = await resp.json()
      const members = snapshot.members || []
      const existingMember = members.find(
        (m: any) => normalizeClusterBaseUrl(m.base_url || m.baseUrl || '') === normalizedPanelBaseUrl,
      )
      if (existingMember) {
        existingDomainData.value = {
          domain: confirmInfo.value.domain,
          hubUrl: confirmInfo.value.hubUrl,
        }
        alreadyExistsDialog.value = true
        checkingUrl.value = false
        return
      }
    }
  } catch {
    // If we can't reach the hub or domain doesn't exist, proceed with registration
  }

  checkingUrl.value = false
  registerStep.value = 2
}

const showConfirmDialog = () => {
  const displayName = form.value.displayName.trim()
  if (!displayName) {
    push.error({ title: i18n.global.t('failed'), message: i18n.global.t('clusterCenter.validation.displayName') })
    return
  }
  confirmInfo.value.displayName = displayName
  confirmDialog.value = true
}

const onRegisterDialogClose = () => {
  registerStep.value = 1
  form.value = { joinUri: '', domain: '', hubUrlProtocol: 'https', hubUrlHost: '', token: '', displayName: '' }
}

const pullExistingDomain = async () => {
  alreadyExistsDialog.value = false
  registerDialog.value = false
  registerStep.value = 1
  form.value = { joinUri: '', domain: '', hubUrlProtocol: 'https', hubUrlHost: '', token: '', displayName: '' }
  actionLoading.value = true

  const panelBaseUrl = resolvePanelBaseUrl()
  const displayName = deriveDisplayNameFromBaseUrl(panelBaseUrl)
  const registerMsg = await HttpUtils.post('api/cluster/register', {
    domain: existingDomainData.value.domain,
    hubUrl: existingDomainData.value.hubUrl,
    token: confirmInfo.value.token,
    baseUrl: panelBaseUrl,
    name: '',
    displayName,
  })

  if (registerMsg.success) {
    const operation = registerMsg.obj as ClusterOperationStatus
    if (operation?.id) {
      await pollOperation(operation.id)
    }
    await loadData()
    push.success({
      title: i18n.global.t('success'),
      message: i18n.global.t('clusterCenter.successRegistered'),
      duration: 5000,
    })
  }

  actionLoading.value = false
}

const confirmAndSubmit = async () => {
  confirmDialog.value = false

  const panelBaseUrl = resolvePanelBaseUrl()
  if (!isUsableAbsoluteUrl(panelBaseUrl)) {
    push.error({ title: i18n.global.t('failed'), message: i18n.global.t('clusterCenter.validation.panelUrl') })
    return
  }

  actionLoading.value = true
  const registerMsg = await HttpUtils.post('api/cluster/register', {
    domain: confirmInfo.value.domain,
    hubUrl: confirmInfo.value.hubUrl,
    token: confirmInfo.value.token,
    baseUrl: panelBaseUrl,
    name: '',
    displayName: confirmInfo.value.displayName,
  })

  if (registerMsg.success) {
    const operation = registerMsg.obj as ClusterOperationStatus
    if (operation?.id) {
      await pollOperation(operation.id)
    }
    await loadData()
    registerDialog.value = false
    registerStep.value = 1
    form.value = { joinUri: '', domain: '', hubUrlProtocol: 'https', hubUrlHost: '', token: '', displayName: '' }
    push.success({
      title: i18n.global.t('success'),
      message: i18n.global.t('clusterCenter.successRegistered'),
      duration: 5000,
    })
  }

  actionLoading.value = false
}

const loadData = async () => {
  pageLoading.value = true
  const [domainsMsg, membersMsg] = await Promise.all([
    HttpUtils.get('api/cluster/domains'),
    HttpUtils.get('api/cluster/members'),
  ])
  if (domainsMsg.success) {
    domains.value = Array.isArray(domainsMsg.obj) ? domainsMsg.obj : []
    if (selectedDomainId.value && !domains.value.some((domain) => domain.id === selectedDomainId.value)) {
      selectedDomainId.value = null
    }
  }
  if (membersMsg.success) {
    members.value = Array.isArray(membersMsg.obj) ? membersMsg.obj : []
  }
  pageLoading.value = false
}

const pollOperation = async (operationId: string) => {
  let current: ClusterOperationStatus | null = null
  for (const delay of [0, 300, 700, 1500, 3000]) {
    if (delay > 0) {
      await new Promise((resolve) => setTimeout(resolve, delay))
    }
    const operationMsg = await HttpUtils.get(`api/cluster/operations/${operationId}`)
    if (!operationMsg.success) {
      return null
    }
    current = operationMsg.obj as ClusterOperationStatus
    if (!current || current.state === 'completed') {
      return current
    }
  }
  return current
}

const syncClusterState = async () => {
  const msg = await HttpUtils.post('api/cluster/sync', {})
  const operation = msg.obj as ClusterOperationStatus | null
  if (operation?.message) {
    push.error({ title: i18n.global.t('failed'), message: operation.message })
  }
  await loadData()
  return msg
}

const manualSync = async () => {
  actionLoading.value = true
  try {
    await syncClusterState()
  } finally {
    actionLoading.value = false
  }
}

const deleteMember = async (member: ClusterMember) => {
  deletingMemberId.value = member.id
  const msg = await HttpUtils.delete(`api/cluster/members/${member.id}`)
  if (msg.success) {
    await loadData()
  }
  deletingMemberId.value = null
}

const leaveDomain = async (domain: ClusterDomain | null) => {
  if (!domain) return

  leavingDomainId.value = domain.id
  const msg = await HttpUtils.delete(`api/cluster/domains/${domain.id}`)
  if (msg.success) {
    selectedDomainId.value = null
    await loadData()
  }
  leavingDomainId.value = null
}

onMounted(async () => {
  await syncClusterState()
})

const pingStore = usePingStore()
const meshPingLoading = ref(false)
const meshPingResults = ref<MeshPairResult[]>([])

function memberLatency(nodeId: string): string {
  const results = meshPingResults.value.filter(r => r.target_member_id === nodeId && r.success)
  if (results.length === 0) {
    const any = meshPingResults.value.filter(r => r.target_member_id === nodeId)
    if (any.length > 0) return 'ERROR'
    return '-'
  }
  const avg = results.reduce((s, r) => s + (r.latency_ms ?? 0), 0) / results.length
  return `${avg.toFixed(0)}ms`
}

function memberLatencyStyle(nodeId: string): Record<string, string> {
  const results = meshPingResults.value.filter(r =>
    r.target_member_id === nodeId && (r.success || !r.success)
  )
  if (results.length === 0) return { color: 'var(--app-text-3)' }

  const allFailed = results.every(r => !r.success)
  if (allFailed) return { color: '#721c24', fontWeight: 'bold' }

  const successResults = results.filter(r => r.success)
  if (successResults.length === 0) return { color: 'var(--app-text-3)' }

  const avg = successResults.reduce((s, r) => s + (r.latency_ms ?? 0), 0) / successResults.length
  if (avg < 50) return { color: '#155724', fontWeight: '600' }
  if (avg < 150) return { color: '#856404', fontWeight: '600' }
  if (avg < 300) return { color: '#b45309', fontWeight: '600' }
  return { color: '#721c24', fontWeight: '600' }
}

async function pingAllDomainMembers() {
  if (!selectedDomain.value) return
  meshPingLoading.value = true
  try {
    const result = await pingStore.triggerMeshPing(selectedDomain.value.domain)
    meshPingResults.value = result.results
  } catch {
    // error handled by store
  } finally {
    meshPingLoading.value = false
  }
}
</script>

<style scoped>
.cluster-center__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}

.cluster-center__refresh-btn {
  backdrop-filter: blur(var(--app-blur-panel));
  background: linear-gradient(
    180deg,
    color-mix(in srgb, var(--app-surface-3) 94%, transparent),
    color-mix(in srgb, var(--app-surface-2) 98%, transparent)
  ) !important;
  border: 1px solid color-mix(in srgb, var(--app-text-2) 24%, var(--app-border-2)) !important;
  box-shadow: var(--app-shadow-button) !important;
  color: var(--app-text-1) !important;
}

.cluster-center__refresh-btn:hover {
  background: linear-gradient(
    180deg,
    color-mix(in srgb, var(--app-state-info) 10%, var(--app-surface-3)),
    color-mix(in srgb, var(--app-state-info) 6%, var(--app-surface-2))
  ) !important;
  border-color: color-mix(in srgb, var(--app-state-info) 42%, var(--app-border-2)) !important;
}

.cluster-center__refresh-btn:focus-visible {
  outline: none;
  box-shadow: var(--app-shadow-button), 0 0 0 4px color-mix(in srgb, var(--app-state-info) 18%, transparent) !important;
}

.cluster-center__refresh-btn :deep(.v-btn__overlay) {
  opacity: 0.04;
}

.cluster-center__grid {
  align-items: stretch;
}

.cluster-center__card-title {
  align-items: baseline;
  display: flex;
  flex-wrap: wrap;
  gap: 10px 14px;
  justify-content: space-between;
  width: 100%;
}

.cluster-center__domain-prompt {
  color: var(--app-text-3);
  font-size: 13px;
  font-weight: 500;
  line-height: 1.5;
}

.cluster-center__detail {
  display: grid;
  gap: 16px;
}

.cluster-center__detail-actions {
  align-items: center;
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  justify-content: flex-start;
}

.cluster-center__domains,
.cluster-center__members {
  height: 100%;
}

.cluster-center__empty {
  color: var(--app-text-3);
  padding: 20px 0;
}

.cluster-center__domain-list {
  display: grid;
  gap: 12px;
}

.cluster-center__domain-card {
  background: color-mix(in srgb, var(--app-surface-2) 86%, transparent);
  border: 1px solid var(--app-border-1);
  border-radius: 20px;
  color: inherit;
  cursor: pointer;
  display: grid;
  gap: 8px;
  padding: 16px;
  text-align: left;
  transition: border-color var(--app-motion-fast) var(--app-ease-standard), transform var(--app-motion-fast) var(--app-ease-standard);
}

.cluster-center__domain-card:hover,
.cluster-center__domain-card--active {
  border-color: color-mix(in srgb, var(--app-state-info) 36%, var(--app-border-2));
  transform: translateY(-1px);
}

.cluster-center__domain-head {
  align-items: center;
  display: flex;
  gap: 12px;
  justify-content: space-between;
}

.cluster-center__version,
.cluster-center__domain-card .cluster-center__domain-meta,
.cluster-center__domain-url {
  color: var(--app-text-3);
  font-size: 13px;
}

.cluster-center__selected-head {
  align-items: center;
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  min-width: 0;
}

.cluster-center__selected-version {
  border: 1px solid var(--app-border-1);
  border-radius: 999px;
  line-height: 1;
  padding: 6px 9px;
}

.cluster-center__detail-panel {
  display: grid;
  gap: 14px;
  grid-template-columns: minmax(0, 1fr) 280px;
}

.cluster-center__domain-meta {
  display: grid;
  gap: 8px;
}

.cluster-center__meta-row {
  align-items: start;
  border-bottom: 1px solid var(--app-border-1);
  display: grid;
  gap: 10px;
  grid-template-columns: 112px minmax(0, 1fr);
  padding-bottom: 8px;
}

.cluster-center__meta-row:last-child {
  border-bottom: none;
  padding-bottom: 0;
}

.cluster-center__meta-label {
  color: var(--app-text-3);
  font-size: 12px;
  letter-spacing: 0.04em;
}

.cluster-center__meta-label--header {
  display: inline-flex;
  margin-bottom: 10px;
  text-transform: uppercase;
}

.cluster-center__meta-value {
  font-size: 14px;
  overflow-wrap: anywhere;
}

.cluster-center__actions-tree {
  background: color-mix(in srgb, var(--app-surface-2) 82%, transparent);
  border: 1px solid var(--app-border-1);
  border-radius: 16px;
  min-width: 0;
  padding: 14px 16px;
}

.cluster-center__member-table-wrap {
  border: 1px solid var(--app-border-1);
  border-radius: 18px;
  overflow: hidden;
}

.cluster-center__member-table {
  border-collapse: collapse;
  width: 100%;
}

.cluster-center__member-table th,
.cluster-center__member-table td {
  border-bottom: 1px solid var(--app-border-1);
  padding: 14px 16px;
  text-align: left;
}

.cluster-center__member-table tbody tr:last-child td {
  border-bottom: none;
}

.cluster-center__member-table th {
  color: var(--app-text-3);
  font-size: 12px;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.cluster-center__member-node {
  align-items: center;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.cluster-center__local-badge {
  border: 1px solid color-mix(in srgb, var(--app-state-info) 40%, var(--app-border-1));
  border-radius: 999px;
  color: var(--app-state-info);
  font-size: 12px;
  font-weight: 700;
  line-height: 1;
  padding: 5px 8px;
}

.cluster-center__dialog-body {
  display: grid;
  gap: 12px;
}

.cluster-center__register-tabs {
  margin: 0 16px;
}

.cluster-center__register-tabs :deep(.v-tab) {
  font-size: 14px;
  letter-spacing: 0.04em;
  text-transform: none;
}

.cluster-center__hub-url-field {
  align-items: center;
  background: color-mix(in srgb, var(--app-surface-2) 86%, transparent);
  border: 1px solid var(--app-border-1);
  border-radius: 8px;
  display: flex;
  gap: 0;
  padding: 0 12px;
  transition: border-color var(--app-motion-fast) var(--app-ease-standard);
}

.cluster-center__hub-url-field:focus-within {
  border-color: var(--app-state-info);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--app-state-info) 15%, transparent);
}

.cluster-center__hub-url-protocol {
  flex: 0 0 72px;
  max-width: 72px;
  min-width: 72px;
  width: 72px;
}

.cluster-center__hub-url-protocol :deep(.v-field) {
  min-height: unset;
  padding: 0;
}

.cluster-center__hub-url-protocol :deep(.v-field__input) {
  align-items: center;
  display: flex;
  font-size: 14px;
  line-height: 1;
  min-height: unset;
  padding: 6px 2px 6px 0;
}

.cluster-center__hub-url-protocol :deep(.v-field__append-inner) {
  padding-inline-start: 0;
}

.cluster-center__hub-url-protocol :deep(.v-field__outline),
.cluster-center__hub-url-protocol :deep(.v-field__overlay) {
  display: none;
}

.cluster-center__hub-url-protocol :deep(.v-select__selection) {
  text-overflow: clip;
}

.cluster-center__hub-url-sep {
  color: var(--app-text-3);
  flex-shrink: 0;
  font-size: 14px;
  line-height: 1;
  margin-right: 4px;
  pointer-events: none;
  user-select: none;
}

.cluster-center__hub-url-host {
  flex: 1 1 auto;
  min-width: 0;
}

.cluster-center__hub-url-host :deep(.v-field) {
  min-height: unset;
  padding: 0;
}

.cluster-center__hub-url-host :deep(.v-field__input) {
  align-items: center;
  display: flex;
  min-height: unset;
  padding: 6px 0;
}

.cluster-center__hub-url-host :deep(.v-field__outline) {
  display: none;
}

.cluster-center__confirm-title {
  line-height: 1.3;
  padding: 24px 24px 10px;
}

.cluster-center__confirm-body {
  padding-top: 8px;
}

.cluster-center__confirm-table-wrap {
  border: 1px solid var(--app-border-1);
  border-radius: 18px;
  overflow: hidden;
}

.cluster-center__confirm-table {
  border-collapse: collapse;
  width: 100%;
}

.cluster-center__confirm-table tr {
  border-bottom: 1px solid var(--app-border-1);
}

.cluster-center__confirm-table tbody tr:last-child {
  border-bottom: none;
}

.cluster-center__confirm-table td {
  padding: 14px 16px;
}

.cluster-center__confirm-label {
  color: var(--app-text-3);
  font-size: 13px;
  white-space: nowrap;
  width: 1px;
}

.cluster-center__confirm-value {
  font-size: 14px;
  word-break: break-all;
}

.cluster-center__confirm-token {
  font-family: var(--app-font-mono, ui-monospace, monospace);
  letter-spacing: 0.06em;
}

@media (max-width: 960px) {
  .cluster-center__actions {
    flex-direction: column;
  }

  .cluster-center__detail-panel {
    grid-template-columns: 1fr;
  }

  .cluster-center__member-table,
  .cluster-center__member-table thead,
  .cluster-center__member-table tbody,
  .cluster-center__member-table tr,
  .cluster-center__member-table th,
  .cluster-center__member-table td {
    display: block;
  }

  .cluster-center__member-table thead {
    display: none;
  }

  .cluster-center__member-table tr {
    border-bottom: 1px solid var(--app-border-1);
  }

  .cluster-center__member-table tbody tr:last-child {
    border-bottom: none;
  }
}

.cluster-center__step-indicator {
  align-items: center;
  background: color-mix(in srgb, var(--app-surface-2) 86%, transparent);
  border: 1px solid var(--app-border-1);
  border-radius: 12px;
  display: flex;
  gap: 10px;
  padding: 12px 14px;
}

.cluster-center__step-label {
  color: var(--app-text-3);
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.cluster-center__step-value {
  font-size: 14px;
  font-weight: 600;
  word-break: break-all;
}

@media (max-width: 640px) {
  .cluster-center__meta-row {
    gap: 6px;
    grid-template-columns: 1fr;
  }
}
</style>
