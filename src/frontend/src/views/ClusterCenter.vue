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
          <v-btn variant="text" :loading="pageLoading" @click="loadData">{{ $t('clusterCenter.actions.refresh') }}</v-btn>
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
          <div class="cluster-center__info-grid">
            <div class="cluster-center__info-item">
              <span>{{ $t('clusterCenter.fields.domain') }}</span>
              <strong>{{ selectedDomain.domain }}</strong>
            </div>
            <div class="cluster-center__info-item">
              <span>{{ $t('clusterCenter.fields.hubUrl') }}</span>
              <strong>{{ selectedDomain.hubUrl || '-' }}</strong>
            </div>
            <div class="cluster-center__info-item">
              <span>{{ $t('clusterCenter.table.version') }}</span>
              <strong>{{ formatClusterVersionLabel(selectedDomain.lastVersion) }}</strong>
            </div>
            <div class="cluster-center__info-item">
              <span>{{ $t('clusterCenter.mirroredMembers') }}</span>
              <strong>{{ selectedDomainMembers.length }}</strong>
            </div>
          </div>
        </v-card-text>
      </v-card>

      <v-card class="app-card-shell cluster-center__members" :loading="pageLoading">
        <v-card-title>{{ $t('clusterCenter.registeredServers') }}</v-card-title>
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
                  <th>{{ $t('clusterCenter.table.action') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="member in selectedDomainMembers" :key="member.id">
                  <td>{{ member.nodeId }}</td>
                  <td>{{ member.name || '-' }}</td>
                  <td>{{ member.baseUrl || '-' }}</td>
                  <td>{{ formatClusterVersionLabel(member.lastVersion) }}</td>
                  <td>
                    <v-btn size="small" color="warning" variant="outlined" :loading="deletingMemberId === member.id" @click="deleteMember(member)">
                      {{ $t('clusterCenter.actions.delete') }}
                    </v-btn>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </v-card-text>
      </v-card>
    </section>

    <v-dialog v-model="registerDialog" class="app-dialog app-dialog--compact" max-width="520">
      <v-card class="app-card-shell">
        <v-card-title>{{ $t('clusterCenter.dialogTitle') }}</v-card-title>

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
          <v-btn color="primary" :loading="actionLoading" @click="prepareConfirm">{{ $t('clusterCenter.actions.submit') }}</v-btn>
        </v-card-actions>
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
  </div>
</template>

<script lang="ts" setup>
import { computed, onMounted, ref } from 'vue'
import { push } from 'notivue'

import HttpUtils from '@/plugins/httputil'
import { i18n } from '@/locales'
import { parseClusterHubJoinUri } from '@/features/clusterHubUri'
import type { ClusterDomain, ClusterMember, ClusterOperationStatus } from '@/types/clusters'

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
})

const confirmInfo = ref({
  hubUrl: '',
  domain: '',
  token: '',
  baseUrl: '',
})

const selectedDomain = computed(() => domains.value.find((domain) => domain.id === selectedDomainId.value) ?? null)
const selectedDomainMembers = computed(() => members.value.filter((member) => member.domainId === selectedDomainId.value))

const domainMemberCount = (domainId: number) => members.value.filter((member) => member.domainId === domainId).length
const formatClusterVersionLabel = (version: number) => `version-${version}`

const openDomainDetail = (domain: ClusterDomain) => {
  selectedDomainId.value = domain.id
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

const prepareConfirm = () => {
  if (registerMode.value === 'uri') {
    const uri = form.value.joinUri.trim()
    const parsed = parseClusterHubJoinUri(uri)
    if (!parsed) {
      push.error({ title: i18n.global.t('failed'), message: 'URI 格式无效，请检查后重试' })
      return
    }
    confirmInfo.value = {
      hubUrl: `${parsed.protocol}://${parsed.host}`,
      domain: parsed.domain,
      token: parsed.token,
      baseUrl: resolvePanelBaseUrl(),
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

    confirmInfo.value = {
      hubUrl,
      domain,
      token: form.value.token,
      baseUrl: resolvePanelBaseUrl(),
    }
  }

  confirmDialog.value = true
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
  })

  if (registerMsg.success) {
    const operation = registerMsg.obj as ClusterOperationStatus
    if (operation?.id) {
      await pollOperation(operation.id)
    }
    await loadData()
    registerDialog.value = false
    form.value = { joinUri: '', domain: '', hubUrlProtocol: 'https', hubUrlHost: '', token: '' }
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

const manualSync = async () => {
  actionLoading.value = true
  const msg = await HttpUtils.post('api/cluster/sync', {})
  if (msg.success) {
    await loadData()
  }
  actionLoading.value = false
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
  await loadData()
})
</script>

<style scoped>
.cluster-center__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
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
.cluster-center__domain-meta,
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

.cluster-center__info-grid {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.cluster-center__info-item {
  background: color-mix(in srgb, var(--app-surface-2) 82%, transparent);
  border: 1px solid var(--app-border-1);
  border-radius: 16px;
  display: grid;
  gap: 8px;
  min-width: 0;
  padding: 14px 16px;
}

.cluster-center__info-item span {
  color: var(--app-text-3);
  font-size: 12px;
}

.cluster-center__info-item strong {
  font-size: 14px;
  overflow-wrap: anywhere;
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

  .cluster-center__info-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
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

@media (max-width: 640px) {
  .cluster-center__info-grid {
    grid-template-columns: 1fr;
  }
}
</style>
