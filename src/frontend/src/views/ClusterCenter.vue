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
          <span class="app-page__hero-meta-item">{{ selectedDomain ? `v${selectedDomain.lastVersion}` : $t('clusterCenter.metaNoDomain') }}</span>
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

    <v-row class="cluster-center__grid">
      <v-col cols="12" xl="4" lg="5">
        <v-card class="app-card-shell cluster-center__domains" :loading="pageLoading">
          <v-card-title>{{ $t('clusterCenter.domainsTitle') }}</v-card-title>
          <v-card-text>
            <div v-if="domains.length === 0" class="cluster-center__empty">{{ $t('clusterCenter.noDomains') }}</div>
            <div v-else class="cluster-center__domain-list">
              <button
                v-for="domain in domains"
                :key="domain.id"
                type="button"
                :class="['cluster-center__domain-card', { 'cluster-center__domain-card--active': selectedDomainId === domain.id }]"
                @click="selectedDomainId = domain.id"
              >
                <div class="cluster-center__domain-head">
                  <strong>{{ domain.domain }}</strong>
                  <span class="cluster-center__version">v{{ domain.lastVersion }}</span>
                </div>
                <div class="cluster-center__domain-url">{{ domain.hubUrl || $t('clusterCenter.fields.hubUrl') }}</div>
                <div class="cluster-center__domain-meta">{{ domainMemberCount(domain.id) }} {{ $t('clusterCenter.mirroredMembers') }}</div>
              </button>
            </div>
          </v-card-text>
        </v-card>
      </v-col>

      <v-col cols="12" xl="8" lg="7">
        <v-card class="app-card-shell cluster-center__members" :loading="pageLoading">
          <v-card-title>{{ selectedDomain ? selectedDomain.domain : $t('clusterCenter.selectDomain') }}</v-card-title>
          <v-card-text>
            <div v-if="!selectedDomain" class="cluster-center__empty">{{ $t('clusterCenter.inspectPrompt') }}</div>
            <div v-else-if="selectedDomainMembers.length === 0" class="cluster-center__empty">{{ $t('clusterCenter.noMembers') }}</div>
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
                    <td>v{{ member.lastVersion }}</td>
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
      </v-col>
    </v-row>

    <v-dialog v-model="registerDialog" class="app-dialog app-dialog--compact" max-width="520">
      <v-card class="app-card-shell">
        <v-card-title>{{ $t('clusterCenter.dialogTitle') }}</v-card-title>
        <v-card-text class="cluster-center__dialog-body">
          <v-text-field v-model="form.domain" :label="$t('clusterCenter.fields.domain')" hide-details />
          <v-text-field v-model="form.hubUrl" :label="$t('clusterCenter.fields.hubUrl')" hide-details />
          <v-text-field v-model="form.token" :label="$t('clusterCenter.fields.token')" type="password" hide-details />
          <v-text-field v-model="form.baseUrl" :label="$t('clusterCenter.fields.baseUrl')" hide-details />
          <v-text-field v-model="form.name" :label="$t('clusterCenter.fields.name')" hide-details />
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="registerDialog = false">{{ $t('clusterCenter.actions.cancel') }}</v-btn>
          <v-btn color="primary" :loading="actionLoading" @click="registerDomain">{{ $t('clusterCenter.actions.submit') }}</v-btn>
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
import type { ClusterDomain, ClusterMember, ClusterOperationStatus } from '@/types/clusters'

const pageLoading = ref(false)
const actionLoading = ref(false)
const registerDialog = ref(false)
const domains = ref<ClusterDomain[]>([])
const members = ref<ClusterMember[]>([])
const selectedDomainId = ref<number | null>(null)
const deletingMemberId = ref<number | null>(null)

const form = ref({
  domain: '',
  hubUrl: '',
  token: '',
  baseUrl: '',
  name: '',
})

const selectedDomain = computed(() => domains.value.find((domain) => domain.id === selectedDomainId.value) ?? null)
const selectedDomainMembers = computed(() => members.value.filter((member) => member.domainId === selectedDomainId.value))

const domainMemberCount = (domainId: number) => members.value.filter((member) => member.domainId === domainId).length

const isUsableAbsoluteUrl = (value: string) => {
  try {
    const parsed = new URL(value)
    return parsed.protocol === 'https:' || parsed.hostname === 'localhost' || parsed.hostname === '127.0.0.1' || parsed.hostname === '::1'
  } catch {
    return false
  }
}

const loadData = async () => {
  pageLoading.value = true
  const [domainsMsg, membersMsg] = await Promise.all([
    HttpUtils.get('api/cluster/domains'),
    HttpUtils.get('api/cluster/members'),
  ])
  if (domainsMsg.success) {
    domains.value = Array.isArray(domainsMsg.obj) ? domainsMsg.obj : []
    if (!selectedDomainId.value || !domains.value.some((domain) => domain.id === selectedDomainId.value)) {
      selectedDomainId.value = domains.value[0]?.id ?? null
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

const registerDomain = async () => {
  if (!form.value.domain || !form.value.hubUrl || !form.value.token || !form.value.baseUrl) {
    push.error({ title: i18n.global.t('failed'), message: i18n.global.t('clusterCenter.validation.required') })
    return
  }
  if (!isUsableAbsoluteUrl(form.value.hubUrl)) {
    push.error({ title: i18n.global.t('failed'), message: i18n.global.t('clusterCenter.validation.hubUrl') })
    return
  }
  if (!isUsableAbsoluteUrl(form.value.baseUrl)) {
    push.error({ title: i18n.global.t('failed'), message: i18n.global.t('clusterCenter.validation.baseUrl') })
    return
  }
  actionLoading.value = true
  const registerMsg = await HttpUtils.post('api/cluster/register', {
    domain: form.value.domain,
    hubUrl: form.value.hubUrl,
    token: form.value.token,
    baseUrl: form.value.baseUrl,
    name: form.value.name,
  })

  if (registerMsg.success) {
    const operation = registerMsg.obj as ClusterOperationStatus
    if (operation?.id) {
      await pollOperation(operation.id)
    }
    await loadData()
    registerDialog.value = false
    form.value = { domain: '', hubUrl: '', token: '', baseUrl: '', name: '' }
    push.success({
      title: i18n.global.t('success'),
      message: i18n.global.t('clusterCenter.successRegistered'),
      duration: 5000,
    })
  }

  actionLoading.value = false
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

@media (max-width: 960px) {
  .cluster-center__actions {
    flex-direction: column;
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
</style>
