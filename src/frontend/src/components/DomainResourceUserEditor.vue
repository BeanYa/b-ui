<template>
  <v-card class="app-card-shell domain-resource-editor">
    <v-card-title>{{ $t('clusterCenter.domainResources.userDialogTitle') }}</v-card-title>
    <v-card-text class="domain-resource-editor__body">
      <v-row>
        <v-col cols="12" md="4">
          <v-text-field v-model="name" :label="$t('clusterCenter.domainResources.userName')" hide-details />
        </v-col>
        <v-col cols="12" md="4">
          <v-text-field v-model="hubUserUuid" :label="$t('clusterCenter.domainResources.userUuid')" hide-details />
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
        <v-row v-for="key in configKeys" :key="key">
          <v-col cols="12" md="3" class="domain-resource-editor__protocol">
            {{ key }}
          </v-col>
          <v-col cols="12" md="9">
            <div class="domain-resource-editor__config-line">
              <span v-if="previewConfig[key].uuid !== undefined">UUID: {{ configValueLabel(previewConfig[key].uuid) }}</span>
              <span v-if="previewConfig[key].password !== undefined">Password: {{ configValueLabel(previewConfig[key].password) }}</span>
              <span v-if="previewConfig[key].auth_str !== undefined">Auth: {{ configValueLabel(previewConfig[key].auth_str) }}</span>
              <span v-if="previewConfig[key].flow !== undefined">Flow: {{ previewConfig[key].flow }}</span>
            </div>
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

import type { CreateDomainUserResourcePayload } from '@/features/domainResourcesApi'
import {
  createDomainUserConfig,
  isLocalProvided,
  sanitizeDomainResourcePart,
  type DomainUserSecretSource,
} from '@/features/domainResourceLocalProvided'
import { i18n } from '@/locales'
import { randomConfigs } from '@/types/clients'
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
const errorMessage = ref('')

const sourceItems = computed(() => [
  { title: i18n.global.t('clusterCenter.domainResources.autoByTarget').toString(), value: 'auto' },
  { title: i18n.global.t('clusterCenter.domainResources.manualValue').toString(), value: 'manual' },
])

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

const previewConfig = computed(() => createDomainUserConfig(name.value || 'domain-user', secretSources.value, manualSecrets.value))
const configKeys = computed(() => Object.keys(randomConfigs(name.value || 'domain-user')))

const resetForm = () => {
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
  errorMessage.value = ''
}

const normalizeSelectedInboundGroupIds = () => {
  const available = new Set(inboundGroupItems.value.map((item) => item.value))
  selectedInboundGroupIds.value = selectedInboundGroupIds.value
    .map((item) => item.trim())
    .filter((item, index, items) => item && items.indexOf(item) === index && available.has(item))
}

const configValueLabel = (value: unknown): string => {
  if (isLocalProvided(value)) {
    return i18n.global.t('clusterCenter.domainResources.autoByTarget').toString()
  }
  return String(value ?? '')
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
      config: createDomainUserConfig(trimmedName, secretSources.value, manualSecrets.value),
      bound_inbound_group_ids: boundInboundGroupIds,
    },
    bound_inbound_group_ids: boundInboundGroupIds,
  }
  emit('submit', payload)
}

watch(() => props.domain.id, resetForm, { immediate: true })
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
