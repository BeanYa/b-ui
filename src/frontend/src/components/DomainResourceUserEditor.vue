<template>
  <v-card class="app-card-shell domain-resource-editor">
    <v-card-title>{{ dialogTitle }}</v-card-title>
    <v-card-text class="domain-resource-editor__body">
      <v-tabs v-model="tab" align-tabs="center" class="domain-resource-editor__tabs">
        <v-tab value="basics">{{ $t('client.basics') }}</v-tab>
        <v-tab value="config">{{ $t('client.config') }}</v-tab>
        <v-tab value="links">{{ $t('client.links') }}</v-tab>
      </v-tabs>

      <v-window v-model="tab" class="domain-resource-editor__window">
        <v-window-item value="basics">
          <div class="domain-resource-editor__pane">
            <v-row>
              <v-col cols="12" sm="6" md="4">
                <v-switch v-model="enable" :label="$t('enable')" color="primary" hide-details />
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-text-field
                  v-model="hubUserUuid"
                  :label="$t('clusterCenter.domainResources.userUuid')"
                  :disabled="props.mode === 'update'"
                  hide-details
                />
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-combobox v-model="group" :items="groupItems" :label="$t('client.group')" hide-details />
              </v-col>
            </v-row>

            <v-row>
              <v-col cols="12" sm="6" md="4">
                <v-text-field v-model="name" :label="$t('client.name')" hide-details />
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-text-field v-model="desc" :label="$t('client.desc')" hide-details />
              </v-col>
            </v-row>

            <v-row>
              <v-col cols="12" sm="6" md="4">
                <v-text-field v-model.number="volumeGiB" type="number" min="0" :label="$t('stats.volume')" suffix="GiB" hide-details />
              </v-col>
              <v-col cols="12" sm="6" md="4" v-if="!(delayStart && !autoReset)">
                <DatePick :expiry="expiry" @submit="setExpiry" />
              </v-col>
              <v-col cols="12" sm="6" md="4" v-if="autoReset || delayStart">
                <v-text-field v-model.number="resetDaysModel" type="number" min="1" :label="$t('client.resetDays')" hide-details />
              </v-col>
            </v-row>

            <v-row>
              <v-col cols="12" sm="6" md="4">
                <v-switch v-model="delayStart" :disabled="up + down > 0" :label="$t('client.delayStart')" color="primary" hide-details />
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-switch v-model="autoReset" :label="$t('client.autoReset')" color="primary" hide-details />
              </v-col>
            </v-row>

            <v-row v-if="props.mode === 'update'">
              <v-col cols="12" sm="6" md="4" class="domain-resource-editor__usage">
                <div class="domain-resource-editor__usage-row">
                  <span>{{ $t('stats.usage') }}: {{ totalUsage }}</span>
                  <v-btn density="compact" variant="text" icon="mdi-restore" @click="resetUsage">
                    <v-tooltip activator="parent" location="top">{{ $t('reset') }}</v-tooltip>
                    <v-icon />
                  </v-btn>
                </div>
                <v-progress-linear v-if="volumeBytes > 0" :model-value="usagePercent" :color="usagePercentColor" />
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-icon icon="mdi-upload" color="orange" />
                <span class="text-orange">{{ upText }}</span>
                /
                <v-icon icon="mdi-download" color="success" />
                <span class="text-success">{{ downText }}</span>
              </v-col>
            </v-row>

            <v-row>
              <v-col>
                <v-select
                  v-model="selectedInboundGroupIds"
                  :items="inboundGroupItems"
                  :label="$t('clusterCenter.domainResources.userInboundGroups')"
                  clearable
                  multiple
                  chips
                  closable-chips
                  hide-details
                >
                  <template #append>
                    <v-icon @click="setAllInboundGroups" icon="mdi-set-all" v-tooltip:top="$t('all')" />
                  </template>
                </v-select>
              </v-col>
            </v-row>

            <div v-if="appliedNodes.length > 0" class="domain-resource-editor__section">
              <div class="domain-resource-editor__section-title">{{ $t('clusterCenter.domainResources.appliedNodes') }}</div>
              <div class="domain-resource-editor__applied-nodes">
                <div
                  v-for="node in appliedNodes"
                  :key="node.node_id || node.nodeId || node.display_name || node.displayName || node.client_id"
                  class="domain-resource-editor__applied-node"
                >
                  <span class="domain-resource-editor__applied-node-name">{{ nodeDisplayName(node) }}</span>
                  <span class="domain-resource-editor__applied-node-meta">{{ node.node_id || node.nodeId || '-' }}</span>
                </div>
              </div>
            </div>
          </div>
        </v-window-item>

        <v-window-item value="config">
          <div class="domain-resource-editor__pane">
            <v-row>
              <v-col cols="12" sm="6" md="4">
                <v-btn variant="tonal" @click="shuffleConfig()">
                  {{ $t('reset') + ' - ' + $t('all') }}
                  <v-icon icon="mdi-refresh" />
                </v-btn>
              </v-col>
            </v-row>

            <v-row v-for="protocol in clientConfigKeys" :key="protocol" class="domain-resource-editor__config-row">
              <v-col cols="12" md="3" class="domain-resource-editor__protocol" align="end" align-self="center">
                {{ protocol }}
                <v-icon @click="shuffleConfig(protocol)" icon="mdi-refresh" v-tooltip:top="$t('reset')" />
              </v-col>
              <v-col>
                <v-row dense>
                  <v-col
                    v-for="field in fieldsForProtocol(protocol)"
                    :key="`${protocol}-${field.key}`"
                    cols="12"
                    sm="6"
                    md="4"
                  >
                    <v-select
                      v-if="field.type === 'select'"
                      :model-value="configSelectValue(protocol, field.key)"
                      :items="field.items ?? []"
                      :label="field.label"
                      hide-details
                      @update:model-value="setConfigValue(protocol, field.key, $event)"
                    />
                    <v-text-field
                      v-else
                      :model-value="configInputValue(protocol, field.key)"
                      :type="field.type === 'number' ? 'number' : 'text'"
                      :label="field.label"
                      hide-details
                      @update:model-value="setConfigValue(protocol, field.key, $event, field.type)"
                    />
                  </v-col>
                </v-row>
              </v-col>
            </v-row>
          </div>
        </v-window-item>

        <v-window-item value="links">
          <div class="domain-resource-editor__pane">
            <v-row v-for="(lnk, index) in links" :key="`${lnk.type}-${index}-${lnk.uri}`">
              <v-col cols="auto">{{ index + 1 }}</v-col>
              <v-col class="domain-resource-editor__link-uri">{{ lnk.uri }}</v-col>
            </v-row>

            <v-row>
              <v-col>
                <v-btn color="primary" @click="extLinks.push({ type: 'external', uri: '' })">
                  {{ $t('actions.add') }} {{ $t('client.external') }}
                </v-btn>
              </v-col>
            </v-row>
            <v-row v-for="(lnk, index) in extLinks" :key="`external-${index}`">
              <v-col>
                <v-text-field
                  v-model="lnk.uri"
                  dir="ltr"
                  :label="$t('client.external') + ' ' + (index + 1)"
                  append-icon="mdi-delete"
                  placeholder="<protocol>://<data>"
                  @click:append="extLinks.splice(index, 1)"
                />
              </v-col>
            </v-row>

            <v-row>
              <v-col>
                <v-btn color="primary" @click="subLinks.push({ type: 'sub', uri: '' })">
                  {{ $t('actions.add') }} {{ $t('client.sub') }}
                </v-btn>
              </v-col>
            </v-row>
            <v-row v-for="(lnk, index) in subLinks" :key="`sub-${index}`">
              <v-col>
                <v-text-field
                  v-model="lnk.uri"
                  dir="ltr"
                  :label="$t('client.sub') + ' ' + (index + 1)"
                  append-icon="mdi-delete"
                  placeholder="http[s]://<domain>[:]<port>/<path>"
                  @click:append="subLinks.splice(index, 1)"
                />
              </v-col>
            </v-row>
          </div>
        </v-window-item>
      </v-window>

      <v-alert v-if="errorMessage" type="error" variant="tonal" density="compact">
        {{ errorMessage }}
      </v-alert>
    </v-card-text>
    <v-card-actions>
      <v-spacer />
      <v-btn variant="text" @click="emit('cancel')">{{ $t('clusterCenter.actions.cancel') }}</v-btn>
      <v-btn color="primary" :loading="loading" @click="submit">
        {{ $t('clusterCenter.domainResources.submit') }}
      </v-btn>
    </v-card-actions>
  </v-card>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'

import DatePick from '@/components/DateTime.vue'
import type { CreateDomainUserResourcePayload, DomainResourceUserView } from '@/features/domainResourcesApi'
import {
  cloneDomainUserConfig,
  domainUserProtocolFields,
  sanitizeDomainResourcePart,
  type DomainUserConfig,
  type DomainUserProtocolField,
  type DomainUserProtocolFieldType,
} from '@/features/domainResourceLocalProvided'
import { i18n } from '@/locales'
import RandomUtil from '@/plugins/randomUtil'
import { HumanReadable } from '@/plugins/utils'
import type { ClusterDomain } from '@/types/clusters'
import { randomConfigs, shuffleConfigs, updateConfigs, type Link as ClientLink } from '@/types/clients'

type Link = ClientLink

export interface DomainInboundGroupOption {
  groupId: string
  label?: string
}

const props = defineProps<{
  domain: ClusterDomain
  loading?: boolean
  error?: string
  defaultInboundGroup?: string
  availableInboundGroups?: DomainInboundGroupOption[]
  initialResource?: DomainResourceUserView | null
  mode?: 'create' | 'update'
}>()

const emit = defineEmits<{
  cancel: []
  submit: [payload: CreateDomainUserResourcePayload]
}>()

const GiB = 1024 ** 3

const tab = ref('basics')
const name = ref('')
const hubUserUuid = ref('')
const enable = ref(true)
const group = ref('')
const desc = ref('')
const selectedInboundGroupIds = ref<string[]>([])
const protocolConfig = ref<DomainUserConfig>(randomConfigs('domain-user'))
const links = ref<Link[]>([])
const extLinks = ref<Link[]>([])
const subLinks = ref<Link[]>([])
const volumeGiB = ref(0)
const expiry = ref(0)
const up = ref(0)
const down = ref(0)
const delayStart = ref(false)
const autoReset = ref(false)
const resetDays = ref(0)
const nextReset = ref(0)
const totalUp = ref(0)
const totalDown = ref(0)
const errorMessage = ref('')

const dialogTitle = computed(() => props.mode === 'update'
  ? i18n.global.t('clusterCenter.domainResources.editUserDialogTitle').toString()
  : i18n.global.t('clusterCenter.domainResources.userDialogTitle').toString())

const groupItems = computed(() => {
  const values = [props.domain.domain, props.initialResource?.group, group.value]
    .map((item) => String(item ?? '').trim())
    .filter(Boolean)
  return [...new Set(values)]
})

const inboundGroupItems = computed(() => {
  const items = new Map<string, string>()
  for (const item of props.availableInboundGroups ?? []) {
    const groupId = item.groupId.trim()
    if (groupId) items.set(groupId, item.label?.trim() || groupId)
  }
  const defaultGroupId = props.defaultInboundGroup?.trim() || `domain-${props.domain.id}`
  if (defaultGroupId && !items.has(defaultGroupId)) {
    items.set(defaultGroupId, defaultGroupId)
  }
  return [...items.entries()].map(([value, title]) => ({ title, value }))
})

const clientConfigKeys = computed(() => Object.keys(protocolConfig.value))
const appliedNodes = computed(() => props.initialResource?.applied_nodes ?? [])
const volumeBytes = computed(() => volumeGiB.value > 0 ? Math.round(volumeGiB.value * GiB) : 0)
const usagePercent = computed(() => volumeBytes.value > 0 ? Math.round((up.value + down.value) * 100 / volumeBytes.value) : 0)
const usagePercentColor = computed(() => (up.value + down.value) >= volumeBytes.value ? 'error' : usagePercent.value > 90 ? 'warning' : 'success')
const upText = computed(() => HumanReadable.sizeFormat(up.value))
const downText = computed(() => HumanReadable.sizeFormat(down.value))
const totalUsage = computed(() => HumanReadable.sizeFormat(up.value + down.value))
const resetDaysModel = computed({
  get: () => resetDays.value || 1,
  set: (value: number | null) => {
    const normalized = value || 1
    if (nextReset.value > 0) {
      nextReset.value += (normalized - (resetDays.value || 0)) * 24 * 60 * 60
    }
    resetDays.value = normalized
  },
})

const nodeDisplayName = (node: NonNullable<DomainResourceUserView['applied_nodes']>[number]) =>
  node.display_name || node.displayName || node.node_id || node.nodeId || `#${node.client_id ?? '-'}`

const resetForm = () => {
  if (props.mode === 'update' && props.initialResource) {
    applyInitialResource(props.initialResource)
    return
  }
  const domainPart = sanitizeDomainResourcePart(props.domain.domain, `domain-${props.domain.id}`)
  name.value = RandomUtil.randomSeq(8)
  hubUserUuid.value = ''
  enable.value = true
  group.value = props.domain.domain || domainPart
  desc.value = ''
  selectedInboundGroupIds.value = props.defaultInboundGroup ? [props.defaultInboundGroup] : [`domain-${props.domain.id}`]
  protocolConfig.value = randomConfigs(name.value)
  links.value = []
  extLinks.value = []
  subLinks.value = []
  volumeGiB.value = 0
  expiry.value = 0
  up.value = 0
  down.value = 0
  delayStart.value = false
  autoReset.value = false
  resetDays.value = 0
  nextReset.value = 0
  totalUp.value = 0
  totalDown.value = 0
  tab.value = 'basics'
  errorMessage.value = ''
}

const applyInitialResource = (resource: DomainResourceUserView) => {
  name.value = resource.name ?? ''
  hubUserUuid.value = resource.uuid ?? ''
  enable.value = resource.enable !== false
  group.value = resource.group ?? props.domain.domain
  desc.value = resource.desc ?? ''
  selectedInboundGroupIds.value = normalizeInitialInboundGroups(resource)
  protocolConfig.value = mergeDomainUserConfig(resource.config)
  links.value = normalizeLinks(resource.links).filter((link) => link.type === 'local')
  extLinks.value = normalizeLinks(resource.links).filter((link) => link.type === 'external')
  subLinks.value = normalizeLinks(resource.links).filter((link) => link.type === 'sub')
  volumeGiB.value = resource.volume && resource.volume > 0 ? resource.volume / GiB : 0
  expiry.value = Number(resource.expiry ?? 0)
  up.value = Number(resource.up ?? 0)
  down.value = Number(resource.down ?? 0)
  delayStart.value = resource.delay_start === true
  autoReset.value = resource.auto_reset === true
  resetDays.value = Number(resource.reset_days ?? 0)
  nextReset.value = Number(resource.next_reset ?? 0)
  totalUp.value = Number(resource.total_up ?? 0)
  totalDown.value = Number(resource.total_down ?? 0)
  tab.value = 'basics'
  errorMessage.value = ''
}

const normalizeInitialInboundGroups = (resource: DomainResourceUserView) => {
  const groups = resource.bound_inbound_group_ids && resource.bound_inbound_group_ids.length > 0
    ? resource.bound_inbound_group_ids
    : (resource.inbounds ?? []).map((item) => String(item).replace(/^domain:/, ''))
  return groups.map((item) => item.trim()).filter((item, index, items) => item && items.indexOf(item) === index)
}

const mergeDomainUserConfig = (config?: Record<string, unknown>): DomainUserConfig => {
  const merged = randomConfigs(name.value || 'domain-user')
  if (!config || typeof config !== 'object' || Array.isArray(config)) return merged
  for (const protocol of Object.keys(merged)) {
    const incoming = config[protocol]
    if (incoming && typeof incoming === 'object' && !Array.isArray(incoming)) {
      merged[protocol] = {
        ...merged[protocol],
        ...(incoming as Record<string, unknown>),
      }
    }
  }
  return merged
}

const normalizeLinks = (raw?: Link[]): Link[] => {
  if (!Array.isArray(raw)) return []
  const result: Link[] = []
  for (const link of raw) {
    if (link.type !== 'local' && link.type !== 'external' && link.type !== 'sub') continue
    const uri = link.uri?.trim() ?? ''
    if (!uri) continue
    const remark = link.remark?.trim()
    result.push({
      type: link.type,
      uri,
      ...(remark ? { remark } : {}),
    })
  }
  return result
}

const fieldsForProtocol = (protocol: string): DomainUserProtocolField[] => {
  const definition = domainUserProtocolFields.find((item) => item.protocol === protocol)
  if (definition) return definition.fields
  return Object.keys(protocolConfig.value[protocol] ?? {}).map((key) => ({ key, label: key }))
}

const configInputValue = (protocol: string, key: string): string | number => {
  const value = protocolConfig.value[protocol]?.[key]
  if (value == null || typeof value === 'object') return ''
  return value as string | number
}

const configSelectValue = (protocol: string, key: string): string => String(configInputValue(protocol, key))

const setConfigValue = (
  protocol: string,
  key: string,
  value: unknown,
  type?: DomainUserProtocolFieldType,
) => {
  const target = protocolConfig.value[protocol]
  if (!target) return
  if (type === 'number') {
    const parsed = Number(value)
    target[key] = Number.isFinite(parsed) ? parsed : 0
    return
  }
  target[key] = String(value ?? '')
}

const syncProtocolNames = () => {
  protocolConfig.value = updateConfigs(protocolConfig.value, name.value || 'domain-user') as DomainUserConfig
}

const shuffleConfig = (protocol?: string) => {
  shuffleConfigs(protocolConfig.value, protocol)
}

const normalizeSelectedInboundGroupIds = () => {
  const available = new Set(inboundGroupItems.value.map((item) => item.value))
  selectedInboundGroupIds.value = selectedInboundGroupIds.value
    .map((item) => item.trim())
    .filter((item, index, items) => item && items.indexOf(item) === index && available.has(item))
}

const setAllInboundGroups = () => {
  selectedInboundGroupIds.value = inboundGroupItems.value.map((item) => item.value).sort()
}

const setExpiry = (value: number) => {
  expiry.value = value
}

const resetUsage = () => {
  totalUp.value += up.value
  totalDown.value += down.value
  up.value = 0
  down.value = 0
}

const submit = () => {
  const trimmedName = name.value.trim()
  if (!trimmedName) {
    errorMessage.value = i18n.global.t('clusterCenter.domainResources.userNameRequired').toString()
    return
  }
  errorMessage.value = ''
  normalizeSelectedInboundGroupIds()
  const boundInboundGroupIds = [...selectedInboundGroupIds.value]
  const normalizedLinks = normalizeLinks([
    ...extLinks.value.map((link) => ({ ...link, type: 'external' as const })),
    ...subLinks.value.map((link) => ({ ...link, type: 'sub' as const })),
  ])
  const payload: CreateDomainUserResourcePayload = {
    user: {
      uuid: hubUserUuid.value.trim() || undefined,
      name: trimmedName,
      enable: enable.value,
      group: group.value.trim() || undefined,
      desc: desc.value.trim() || undefined,
      config: cloneDomainUserConfig(protocolConfig.value),
      links: normalizedLinks,
      volume: volumeBytes.value,
      expiry: expiry.value,
      down: down.value,
      up: up.value,
      delay_start: delayStart.value,
      auto_reset: autoReset.value,
      reset_days: resetDays.value,
      next_reset: nextReset.value,
      total_up: totalUp.value,
      total_down: totalDown.value,
      bound_inbound_group_ids: boundInboundGroupIds,
    },
    bound_inbound_group_ids: boundInboundGroupIds,
  }
  emit('submit', payload)
}

watch(() => [props.domain.id, props.mode, props.initialResource?.uuid], resetForm, { immediate: true })
watch(name, syncProtocolNames)
watch(delayStart, (value) => {
  resetDays.value = value ? 1 : 0
  if (value && !autoReset.value) expiry.value = 0
}, { flush: 'sync' })
watch(autoReset, (value) => {
  resetDays.value = value ? 1 : 0
  if (!value) nextReset.value = 0
  if (delayStart.value && !value) expiry.value = 0
}, { flush: 'sync' })
watch(() => props.defaultInboundGroup, (value) => {
  if (value && props.mode !== 'update') {
    selectedInboundGroupIds.value = [value]
  }
})
watch(inboundGroupItems, normalizeSelectedInboundGroupIds)
watch(() => props.error, (value) => {
  errorMessage.value = value ?? ''
})
</script>

<style scoped>
.domain-resource-editor__body {
  display: grid;
  gap: 14px;
  max-height: min(72vh, 760px);
  overflow-y: auto;
  padding-top: 0;
}

.domain-resource-editor__tabs {
  border: 1px solid var(--app-border-1);
  border-radius: 8px;
  margin-top: 2px;
}

.domain-resource-editor__window {
  min-height: 360px;
}

.domain-resource-editor__pane {
  display: grid;
  gap: 14px;
  padding-top: 16px;
}

.domain-resource-editor__section {
  border-top: 1px solid var(--app-border-1);
  display: grid;
  gap: 10px;
  padding-top: 12px;
}

.domain-resource-editor__section-title {
  color: var(--app-text-2);
  font-size: 13px;
  font-weight: 700;
}

.domain-resource-editor__usage {
  display: grid;
  gap: 6px;
}

.domain-resource-editor__usage-row {
  align-items: center;
  display: flex;
  justify-content: space-between;
}

.domain-resource-editor__applied-nodes {
  display: grid;
  gap: 8px;
}

.domain-resource-editor__applied-node {
  align-items: center;
  background: color-mix(in srgb, var(--app-surface-2) 76%, transparent);
  border: 1px solid var(--app-border-1);
  border-radius: 8px;
  display: flex;
  gap: 10px;
  justify-content: space-between;
  min-width: 0;
  padding: 10px 12px;
}

.domain-resource-editor__applied-node-name {
  font-weight: 700;
  min-width: 0;
  overflow-wrap: anywhere;
}

.domain-resource-editor__applied-node-meta {
  color: var(--app-text-3);
  font-family: var(--app-font-mono, ui-monospace, monospace);
  font-size: 12px;
  overflow-wrap: anywhere;
}

.domain-resource-editor__protocol {
  color: var(--app-text-2);
  font-family: var(--app-font-mono, ui-monospace, monospace);
  font-size: 12px;
  font-weight: 700;
}

.domain-resource-editor__link-uri {
  direction: ltr;
  min-width: 0;
  overflow-wrap: anywhere;
}
</style>
