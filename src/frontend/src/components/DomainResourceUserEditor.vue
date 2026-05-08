<template>
  <v-card class="app-card-shell domain-resource-editor">
    <v-card-title>{{ dialogTitle }}</v-card-title>
    <v-card-text class="domain-resource-editor__body">
      <v-row>
        <v-col cols="12" md="4">
          <v-text-field v-model="name" :label="$t('clusterCenter.domainResources.userName')" hide-details />
        </v-col>
        <v-col cols="12" md="4">
          <v-text-field v-model="hubUserUuid" :label="$t('clusterCenter.domainResources.userUuid')" :disabled="props.mode === 'update'" hide-details />
        </v-col>
        <v-col cols="12" md="4">
          <v-switch v-model="enable" :label="$t('clusterCenter.domainResources.userEnable')" color="primary" hide-details />
        </v-col>
      </v-row>

      <v-row>
        <v-col cols="12" md="4">
          <v-text-field v-model="group" :label="$t('client.group')" hide-details />
        </v-col>
        <v-col cols="12" md="8">
          <v-text-field v-model="desc" :label="$t('client.desc')" hide-details />
        </v-col>
      </v-row>

      <v-select
        v-model="selectedInboundGroupIds"
        :items="inboundGroupItems"
        :label="$t('clusterCenter.domainResources.userInboundGroups')"
        multiple
        chips
        closable-chips
        hide-details
      />

      <v-card class="domain-resource-editor__section" :subtitle="$t('clusterCenter.domainResources.secretSources')">
        <v-row>
          <v-col cols="12" md="4">
            <v-select
              v-model="uuidSource"
              :items="sourceItems"
              label="UUID"
              hide-details
            />
          </v-col>
          <v-col cols="12" md="4">
            <v-select
              v-model="passwordSource"
              :items="sourceItems"
              label="Password"
              hide-details
            />
          </v-col>
          <v-col cols="12" md="4">
            <v-select
              v-model="authSource"
              :items="sourceItems"
              label="Auth"
              hide-details
            />
          </v-col>
        </v-row>
        <v-row>
          <v-col v-if="uuidSource === 'manual'" cols="12" md="4">
            <v-text-field v-model="manualSecrets.uuid" label="UUID" hide-details />
          </v-col>
          <v-col v-if="passwordSource === 'manual'" cols="12" md="4">
            <v-text-field v-model="manualSecrets.password" label="Password" hide-details />
          </v-col>
          <v-col v-if="authSource === 'manual'" cols="12" md="4">
            <v-text-field v-model="manualSecrets.auth" label="Auth" hide-details />
          </v-col>
        </v-row>
      </v-card>

      <v-card class="domain-resource-editor__section" :subtitle="$t('client.config')">
        <v-row v-for="definition in domainUserProtocolFields" :key="definition.protocol" class="domain-resource-editor__config-row">
          <v-col cols="12" md="2" class="domain-resource-editor__protocol">
            {{ definition.protocol }}
          </v-col>
          <v-col cols="12" md="10">
            <v-row dense>
              <v-col
                v-for="field in definition.fields"
                :key="`${definition.protocol}-${field.key}`"
                cols="12"
                sm="6"
                md="4"
              >
                <v-select
                  v-if="field.type === 'select'"
                  :model-value="configSelectValue(definition.protocol, field.key)"
                  :items="field.items ?? []"
                  :label="field.label"
                  hide-details
                  density="compact"
                  @update:model-value="setConfigValue(definition.protocol, field.key, $event)"
                />
                <v-text-field
                  v-else
                  :model-value="configInputValue(definition.protocol, field.key)"
                  :type="field.type === 'number' ? 'number' : 'text'"
                  :label="field.label"
                  :placeholder="field.secret && isConfigAuto(definition.protocol, field.key) ? autoByTargetLabel : undefined"
                  :hint="field.secret && isConfigAuto(definition.protocol, field.key) ? $t('clusterCenter.domainResources.autoByTarget') : undefined"
                  :persistent-hint="Boolean(field.secret && isConfigAuto(definition.protocol, field.key))"
                  hide-details="auto"
                  density="compact"
                  @update:model-value="setConfigValue(definition.protocol, field.key, $event, field.type)"
                />
              </v-col>
            </v-row>
          </v-col>
        </v-row>
      </v-card>

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

import type { CreateDomainUserResourcePayload, DomainResourceUserView } from '@/features/domainResourcesApi'
import {
  cloneDomainUserConfig,
  createDomainUserConfig,
  domainUserProtocolFields,
  isLocalProvided,
  localProvided,
  sanitizeDomainResourcePart,
  type DomainUserConfig,
  type DomainUserManualSecrets,
  type DomainUserProtocolFieldType,
  type DomainUserSecretSource,
} from '@/features/domainResourceLocalProvided'
import { i18n } from '@/locales'
import type { ClusterDomain } from '@/types/clusters'

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

const name = ref('')
const hubUserUuid = ref('')
const enable = ref(true)
const group = ref('')
const desc = ref('')
const selectedInboundGroupIds = ref<string[]>([])
const uuidSource = ref<DomainUserSecretSource>('auto')
const passwordSource = ref<DomainUserSecretSource>('auto')
const authSource = ref<DomainUserSecretSource>('auto')
const secretSources = computed(() => ({
  uuid: uuidSource.value,
  password: passwordSource.value,
  auth: authSource.value,
}))
const manualSecrets = ref({
  uuid: '',
  password: '',
  auth: '',
})
const protocolConfig = ref<DomainUserConfig>(createDomainUserConfig('domain-user', {
  uuid: 'auto',
  password: 'auto',
  auth: 'auto',
}))
const errorMessage = ref('')

const sourceItems = computed(() => [
  { title: i18n.global.t('clusterCenter.domainResources.autoByTarget').toString(), value: 'auto' },
  { title: i18n.global.t('clusterCenter.domainResources.manualValue').toString(), value: 'manual' },
])
const autoByTargetLabel = computed(() => i18n.global.t('clusterCenter.domainResources.autoByTarget').toString())
const dialogTitle = computed(() => props.mode === 'update'
  ? i18n.global.t('clusterCenter.domainResources.editUserDialogTitle').toString()
  : i18n.global.t('clusterCenter.domainResources.userDialogTitle').toString())

const inboundGroupItems = computed(() => {
  const items = new Map<string, string>()
  for (const group of props.availableInboundGroups ?? []) {
    const groupId = group.groupId.trim()
    if (groupId) items.set(groupId, group.label?.trim() || groupId)
  }
  const defaultGroupId = props.defaultInboundGroup?.trim() || `domain-${props.domain.id}`
  if (defaultGroupId && !items.has(defaultGroupId)) {
    items.set(defaultGroupId, defaultGroupId)
  }
  return [...items.entries()].map(([value, title]) => ({ title, value }))
})

const resetForm = () => {
  if (props.mode === 'update' && props.initialResource) {
    applyInitialResource(props.initialResource)
    return
  }
  const domainPart = sanitizeDomainResourcePart(props.domain.domain, `domain-${props.domain.id}`)
  name.value = `user-${domainPart}`
  hubUserUuid.value = ''
  enable.value = true
  group.value = props.domain.domain
  desc.value = ''
  selectedInboundGroupIds.value = props.defaultInboundGroup ? [props.defaultInboundGroup] : [`domain-${props.domain.id}`]
  uuidSource.value = 'auto'
  passwordSource.value = 'auto'
  authSource.value = 'auto'
  manualSecrets.value = { uuid: '', password: '', auth: '' }
  syncProtocolConfig()
  errorMessage.value = ''
}

const syncProtocolConfig = () => {
  protocolConfig.value = createDomainUserConfig(name.value || 'domain-user', secretSources.value, manualSecrets.value)
}

const applyInitialResource = (resource: DomainResourceUserView) => {
  name.value = resource.name ?? ''
  hubUserUuid.value = resource.uuid ?? ''
  enable.value = resource.enable !== false
  group.value = resource.group ?? props.domain.domain
  desc.value = resource.desc ?? ''
  selectedInboundGroupIds.value = normalizeInitialInboundGroups(resource)
  uuidSource.value = 'auto'
  passwordSource.value = 'auto'
  authSource.value = 'auto'
  manualSecrets.value = { uuid: '', password: '', auth: '' }
  protocolConfig.value = mergeDomainUserConfig(resource.config)
  errorMessage.value = ''
}

const normalizeInitialInboundGroups = (resource: DomainResourceUserView) => {
  const groups = resource.bound_inbound_group_ids && resource.bound_inbound_group_ids.length > 0
    ? resource.bound_inbound_group_ids
    : (resource.inbounds ?? []).map((item) => String(item))
  return groups.map((item) => item.trim()).filter((item, index, items) => item && items.indexOf(item) === index)
}

const mergeDomainUserConfig = (config?: Record<string, unknown>): DomainUserConfig => {
  const merged = createDomainUserConfig(name.value || 'domain-user', secretSources.value, manualSecrets.value)
  if (!config || typeof config !== 'object' || Array.isArray(config)) return merged
  for (const definition of domainUserProtocolFields) {
    const incoming = config[definition.protocol]
    if (incoming && typeof incoming === 'object' && !Array.isArray(incoming)) {
      merged[definition.protocol] = {
        ...merged[definition.protocol],
        ...(incoming as Record<string, unknown>),
      }
    }
  }
  return merged
}

const secretValue = (secret: keyof DomainUserManualSecrets) => {
  if (secretSources.value[secret] === 'manual') {
    return manualSecrets.value[secret]?.trim() || ''
  }
  const localProvidedKinds = {
    uuid: 'DomainUserUUID',
    password: 'DomainUserPassword',
    auth: 'DomainUserAuth',
  } as const
  return localProvided(localProvidedKinds[secret])
}

const syncProtocolSecrets = () => {
  for (const definition of domainUserProtocolFields) {
    const target = protocolConfig.value[definition.protocol]
    if (!target) continue
    for (const field of definition.fields) {
      if (field.secret && Object.hasOwn(target, field.key)) {
        target[field.key] = secretValue(field.secret)
      }
    }
  }
}

const syncProtocolNames = () => {
  const userName = name.value || 'domain-user'
  for (const definition of domainUserProtocolFields) {
    const target = protocolConfig.value[definition.protocol]
    if (!target) continue
    if (Object.hasOwn(target, 'name')) {
      target.name = userName
    }
    if (Object.hasOwn(target, 'username')) {
      target.username = userName
    }
  }
}

const normalizeSelectedInboundGroupIds = () => {
  const available = new Set(inboundGroupItems.value.map((item) => item.value))
  selectedInboundGroupIds.value = selectedInboundGroupIds.value
    .map((item) => item.trim())
    .filter((item, index, items) => item && items.indexOf(item) === index && available.has(item))
}

const configInputValue = (protocol: string, key: string): string | number => {
  const value = protocolConfig.value[protocol]?.[key]
  return isLocalProvided(value) ? '' : value ?? ''
}

const configSelectValue = (protocol: string, key: string): string => String(configInputValue(protocol, key))

const isConfigAuto = (protocol: string, key: string): boolean => isLocalProvided(protocolConfig.value[protocol]?.[key])

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

const submit = () => {
  const trimmedName = name.value.trim()
  if (!trimmedName) {
    errorMessage.value = i18n.global.t('clusterCenter.domainResources.userNameRequired').toString()
    return
  }
  errorMessage.value = ''
  normalizeSelectedInboundGroupIds()
  const boundInboundGroupIds = [...selectedInboundGroupIds.value]
  const payload: CreateDomainUserResourcePayload = {
    user: {
      uuid: hubUserUuid.value.trim() || undefined,
      name: trimmedName,
      enable: enable.value,
      group: group.value.trim() || undefined,
      desc: desc.value.trim() || undefined,
      config: cloneDomainUserConfig(protocolConfig.value),
      bound_inbound_group_ids: boundInboundGroupIds,
    },
    bound_inbound_group_ids: boundInboundGroupIds,
  }
  emit('submit', payload)
}

watch(() => [props.domain.id, props.mode, props.initialResource?.uuid], resetForm, { immediate: true })
watch([secretSources, manualSecrets], syncProtocolSecrets, { deep: true })
watch(name, syncProtocolNames)
watch(() => props.defaultInboundGroup, (value) => {
  if (value) {
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
}

.domain-resource-editor__section {
  border: 1px solid var(--app-border-1);
  border-radius: 8px;
}

.domain-resource-editor__protocol {
  color: var(--app-text-2);
  font-family: var(--app-font-mono, ui-monospace, monospace);
  font-size: 12px;
  font-weight: 700;
}

.domain-resource-editor__config-line {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 14px;
  font-size: 12px;
}
</style>
