<template>
  <v-card class="app-card-shell domain-resource-editor">
    <v-card-title>{{ $t(mode === 'update' ? 'clusterCenter.domainResources.editInboundDialogTitle' : 'clusterCenter.domainResources.inboundDialogTitle') }}</v-card-title>
    <v-card-text class="domain-resource-editor__body">
      <v-row>
        <v-col cols="12" md="4">
          <v-text-field v-model="groupId" :label="$t('clusterCenter.domainResources.groupId')" :disabled="mode === 'update'" hide-details />
        </v-col>
        <v-col cols="12" md="4">
          <v-select
            v-model="inbound.type"
            :items="protocolItems"
            :label="$t('clusterCenter.domainResources.inboundType')"
            hide-details
            @update:model-value="changeType"
          />
        </v-col>
        <v-col cols="12" md="4">
          <v-text-field v-model="inbound.tag" :label="$t('clusterCenter.domainResources.inboundTag')" hide-details />
        </v-col>
      </v-row>

      <v-row>
        <v-col cols="12" md="4">
          <v-text-field v-model="prefix" :label="$t('clusterCenter.domainResources.prefix')" hide-details />
        </v-col>
        <v-col cols="12" md="4">
          <v-text-field v-model="tagSeed" :label="$t('clusterCenter.domainResources.tagSeed')" hide-details />
        </v-col>
        <v-col cols="12" md="4">
          <v-text-field v-model="suffix" :label="$t('clusterCenter.domainResources.suffix')" hide-details />
        </v-col>
      </v-row>

      <v-card class="domain-resource-editor__section" :subtitle="$t('clusterCenter.domainResources.targetNodes')">
        <v-row>
          <v-col cols="12" md="4">
            <v-select
              v-model="targetScope"
              :items="targetScopeItems"
              :label="$t('clusterCenter.domainResources.targetScope')"
              hide-details
            />
          </v-col>
          <v-col v-if="targetScope === 'pick'" cols="12" md="8">
            <v-select
              v-model="selectedTargetNodeIds"
              :items="targetMemberItems"
              :label="$t('clusterCenter.domainResources.pickList')"
              item-title="title"
              item-value="value"
              multiple
              chips
              closable-chips
              clearable
              hide-details
            />
          </v-col>
        </v-row>
      </v-card>

      <v-card class="domain-resource-editor__section" :subtitle="$t('objects.listen')">
        <v-row>
          <v-col cols="12" md="4">
            <v-text-field v-model="inbound.listen" :label="$t('clusterCenter.domainResources.listenAddress')" hide-details />
          </v-col>
          <v-col cols="12" md="4">
            <v-select
              v-model="listenPortSource"
              :items="sourceItems"
              :label="$t('clusterCenter.domainResources.listenPortSource')"
              hide-details
            />
          </v-col>
          <v-col v-if="listenPortSource === 'manual'" cols="12" md="4">
            <v-text-field
              v-model.number="manualListenPort"
              :label="$t('clusterCenter.domainResources.listenPort')"
              type="number"
              min="1"
              max="65535"
              hide-details
            />
          </v-col>
        </v-row>
      </v-card>

      <v-tabs
        v-if="hasClientOptions"
        v-model="side"
        density="compact"
        fixed-tabs
        align-tabs="center"
      >
        <v-tab value="server">{{ $t('in.sSide') }}</v-tab>
        <v-tab value="client">{{ $t('in.cSide') }}</v-tab>
      </v-tabs>

      <v-window v-model="side">
        <v-window-item value="server">
          <Direct v-if="inbound.type === InTypes.Direct" :data="inbound" />
          <Shadowsocks v-if="inbound.type === InTypes.Shadowsocks" direction="in" :data="inbound" />
          <Hysteria v-if="inbound.type === InTypes.Hysteria" direction="in" :data="inbound" />
          <Hysteria2 v-if="inbound.type === InTypes.Hysteria2" direction="in" :data="inbound" />
          <Naive v-if="inbound.type === InTypes.Naive" direction="in" :data="inbound" />
          <ShadowTls v-if="inbound.type === InTypes.ShadowTLS" direction="in" :data="inbound" />
          <Tuic v-if="inbound.type === InTypes.TUIC" direction="in" :data="inbound" />
          <Tun v-if="inbound.type === InTypes.Tun" :data="inbound" />
          <AnyTls v-if="inbound.type === InTypes.AnyTls" :data="inbound" direction="in" />
          <TProxy v-if="inbound.type === InTypes.TProxy" :inbound="inbound" />
          <Transport v-if="Object.hasOwn(inbound, 'transport')" :data="inbound" />
          <Multiplex v-if="muxAvailable.includes(inbound.type)" direction="in" :data="inbound" />
        </v-window-item>
        <v-window-item value="client">
          <OutJson v-if="hasClientOptions" :inData="inbound" :type="inbound.type" />
          <Multiplex v-if="Object.hasOwn(inbound, 'multiplex')" direction="out" :data="inbound.out_json" />
          <Dial v-if="inbound.out_json" :dial="inbound.out_json" mode="client" />
          <v-card v-if="hasClientOptions" class="domain-resource-editor__section" :subtitle="$t('in.multiDomain')">
            <v-card-text>
              <v-chip color="primary" density="compact" variant="elevated" @click="addAddr">
                <v-icon icon="mdi-plus" />
              </v-chip>
              <template v-for="addr, index in inbound.addrs" :key="index">
                <div class="domain-resource-editor__addr-heading">
                  {{ $t('in.addr') }} #{{ index + 1 }}
                  <v-icon icon="mdi-delete" color="error" @click="inbound.addrs?.splice(index, 1)" />
                </div>
                <v-divider />
                <Addr :addr="addr" :hasTls="hasTls" />
              </template>
            </v-card-text>
          </v-card>
        </v-window-item>
      </v-window>

      <v-card v-if="hasTls" class="domain-resource-editor__section" :subtitle="$t('objects.tls')">
        <v-row>
          <v-col cols="12" md="4">
            <v-select
              v-model="tlsTemplate"
              :items="tlsTemplateItems"
              :label="$t('clusterCenter.domainResources.tlsTemplate')"
              hide-details
            />
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
import { computed, ref, toRaw, watch } from 'vue'

import Direct from '@/components/protocols/Direct.vue'
import Shadowsocks from '@/components/protocols/Shadowsocks.vue'
import Hysteria from '@/components/protocols/Hysteria.vue'
import Hysteria2 from '@/components/protocols/Hysteria2.vue'
import Naive from '@/components/protocols/Naive.vue'
import ShadowTls from '@/components/protocols/ShadowTls.vue'
import Tuic from '@/components/protocols/Tuic.vue'
import Tun from '@/components/protocols/Tun.vue'
import AnyTls from '@/components/protocols/AnyTls.vue'
import TProxy from '@/components/protocols/TProxy.vue'
import Multiplex from '@/components/Multiplex.vue'
import Transport from '@/components/Transport.vue'
import OutJson from '@/components/OutJson.vue'
import Dial from '@/components/Dial.vue'
import Addr from '@/components/Addr.vue'
import type { CreateDomainInboundResourcePayload, DomainResourceInboundInstanceView, DomainResourceInboundView } from '@/features/domainResourcesApi'
import {
  createDomainInboundTls,
  localProvided,
  sanitizeDomainResourcePart,
  type DomainInboundTlsTemplate,
} from '@/features/domainResourceLocalProvided'
import { i18n } from '@/locales'
import { InTypes, createInbound, type Inbound } from '@/types/inbounds'
import type { ClusterDomain, ClusterMember } from '@/types/clusters'

const props = defineProps<{
  domain: ClusterDomain
  members?: ClusterMember[]
  loading?: boolean
  error?: string
  initialResource?: DomainResourceInboundView | null
  mode?: 'create' | 'update'
}>()

const emit = defineEmits<{
  cancel: []
  submit: [payload: CreateDomainInboundResourcePayload]
}>()

const groupId = ref('')
const tagSeed = ref('')
const prefix = ref('domain')
const suffix = ref('')
const inbound = ref<Inbound>(createInbound(InTypes.VLESS, { id: 0, tag: '', listen: '::', listen_port: 443 }))
const listenPortSource = ref<'auto' | 'manual'>('auto')
const manualListenPort = ref(443)
const tlsTemplate = ref<DomainInboundTlsTemplate>('standard')
const errorMessage = ref('')
const targetScope = ref<'all' | 'pick'>('all')
const selectedTargetNodeIds = ref<string[]>([])
const side = ref<'server' | 'client'>('server')

const hasTlsProtocols = [
  InTypes.HTTP,
  InTypes.VMess,
  InTypes.Trojan,
  InTypes.Naive,
  InTypes.Hysteria,
  InTypes.TUIC,
  InTypes.Hysteria2,
  InTypes.VLESS,
  InTypes.AnyTls,
]
const muxAvailable = [
  InTypes.VLESS,
  InTypes.VMess,
  InTypes.Trojan,
  InTypes.Shadowsocks,
]
const clientOptionlessProtocols = [
  InTypes.Direct,
  InTypes.Tun,
  InTypes.Redirect,
  InTypes.TProxy,
]

const protocolItems = Object.keys(InTypes).map((key, index) => ({
  title: key,
  value: Object.values(InTypes)[index],
}))

const sourceItems = computed(() => [
  { title: i18n.global.t('clusterCenter.domainResources.autoByTarget').toString(), value: 'auto' },
  { title: i18n.global.t('clusterCenter.domainResources.manualValue').toString(), value: 'manual' },
])

const targetScopeItems = computed(() => [
  { title: i18n.global.t('clusterCenter.domainResources.broadcastAll').toString(), value: 'all' },
  { title: i18n.global.t('clusterCenter.domainResources.pickList').toString(), value: 'pick' },
])

const targetMemberItems = computed(() => (props.members ?? [])
  .filter((member) => member.nodeId?.trim())
  .map((member) => ({
    title: targetMemberLabel(member),
    value: member.nodeId,
  })))

const tlsTemplateItems = computed(() => [
  { title: i18n.global.t('none').toString(), value: 'none' },
  { title: i18n.global.t('tls.presets.standard').toString(), value: 'standard' },
  { title: i18n.global.t('tls.presets.hysteria2').toString(), value: 'hysteria2' },
  { title: i18n.global.t('tls.presets.reality').toString(), value: 'reality' },
  { title: i18n.global.t('tls.presets.standardCert').toString(), value: 'standard-cert' },
  { title: i18n.global.t('tls.presets.hysteria2Cert').toString(), value: 'hysteria2-cert' },
])

const hasTls = computed(() => hasTlsProtocols.includes(inbound.value.type))
const hasClientOptions = computed(() => !clientOptionlessProtocols.includes(inbound.value.type))

const targetMemberLabel = (member: ClusterMember) => {
  const displayName = (member.displayName || member.name || member.nodeId).trim()
  const label = !displayName || displayName === member.nodeId
    ? member.nodeId
    : `${displayName} (${member.nodeId})`
  if (member.isLocal) return `${label} - ${i18n.global.t('clusterCenter.domainResources.localNode')}`
  return label
}

const buildTargetMembers = (): CreateDomainInboundResourcePayload['target_members'] => {
  if (targetScope.value !== 'pick') return undefined
  const selected = new Set(selectedTargetNodeIds.value)
  const existingTargets = initialTargetsByNodeId.value
  return (props.members ?? [])
    .filter((member) => selected.has(member.nodeId))
    .map((member) => {
      const existing = existingTargets.get(member.nodeId)
      return {
        member_id: member.nodeId,
        node_id: member.nodeId,
        display_name: member.displayName || member.name || member.nodeId,
        target_tag: existing ? resourceInstanceTargetTag(existing) || undefined : undefined,
        remote_inbound_id: existing ? resourceInstanceLocalResourceID(existing) || undefined : undefined,
      }
    })
}

const initialTargetsByNodeId = computed(() => {
  const targets = new Map<string, DomainResourceInboundInstanceView>()
  for (const instance of props.initialResource?.instances ?? []) {
    const nodeId = resourceInstanceNodeID(instance)
    if (nodeId) targets.set(nodeId, instance)
  }
  return targets
})

const resetForm = () => {
  if (props.mode === 'update' && props.initialResource) {
    applyInitialResource(props.initialResource)
    return
  }
  const domainPart = sanitizeDomainResourcePart(props.domain.domain, `domain-${props.domain.id}`)
  groupId.value = `domain-${props.domain.id}`
  tagSeed.value = domainPart
  prefix.value = 'domain'
  suffix.value = ''
  inbound.value = createInbound(InTypes.VLESS, {
    id: 0,
    tag: domainPart,
    listen: '::',
    listen_port: 443,
  }) as Inbound
  ensureClientOptions(inbound.value)
  listenPortSource.value = 'auto'
  manualListenPort.value = 443
  tlsTemplate.value = 'standard'
  errorMessage.value = ''
  targetScope.value = 'all'
  selectedTargetNodeIds.value = []
  side.value = 'server'
}

const applyInitialResource = (resource: DomainResourceInboundView) => {
  const trimmedGroupId = String(resource.group_id ?? '').trim()
  const parsedOptions = parseInboundOptions(resource.options_json)
  const resourceType = resource.type && Object.values(InTypes).includes(resource.type as any)
    ? resource.type
    : InTypes.VLESS
  const firstTargetTag = (resource.instances ?? [])
    .map((instance) => resourceInstanceTargetTag(instance))
    .find(Boolean)
  const seed = String(resource.tag_seed || trimmedGroupId || resource.type || InTypes.VLESS).trim()

  groupId.value = trimmedGroupId
  tagSeed.value = seed
  prefix.value = String(resource.prefix ?? 'domain').trim()
  suffix.value = String(resource.suffix ?? '').trim()
  inbound.value = createInbound(resourceType as any, {
    id: 0,
    tag: firstTargetTag || seed || trimmedGroupId,
    listen: '::',
    listen_port: 443,
    ...parsedOptions,
  }) as Inbound
  ensureClientOptions(inbound.value)

  const listenPort = Number((inbound.value as unknown as Record<string, unknown>).listen_port)
  if (Number.isFinite(listenPort) && listenPort > 0) {
    listenPortSource.value = 'manual'
    manualListenPort.value = listenPort
  } else {
    listenPortSource.value = 'auto'
    manualListenPort.value = 443
  }

  tlsTemplate.value = (resource.tls_template as DomainInboundTlsTemplate) || (hasTls.value ? 'standard' : 'none')
  errorMessage.value = ''
  const targetNodeIds = (resource.instances ?? [])
    .map((instance) => resourceInstanceNodeID(instance))
    .filter(Boolean)
  targetScope.value = targetNodeIds.length > 0 ? 'pick' : 'all'
  selectedTargetNodeIds.value = [...new Set(targetNodeIds)]
  side.value = 'server'
}

const parseInboundOptions = (value?: string): Record<string, unknown> => {
  const trimmed = String(value ?? '').trim()
  if (!trimmed) return {}
  try {
    const parsed = JSON.parse(trimmed)
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return parsed as Record<string, unknown>
    }
  } catch {
    return {}
  }
  return {}
}

const resourceInstanceNodeID = (instance: DomainResourceInboundInstanceView) =>
  String(instance.node_id ?? instance.nodeId ?? '').trim()

const resourceInstanceTargetTag = (instance: DomainResourceInboundInstanceView) =>
  String(instance.target_tag ?? instance.targetTag ?? '').trim()

const resourceInstanceLocalResourceID = (instance: DomainResourceInboundInstanceView) =>
  Number(instance.local_resource_id ?? instance.localResourceId ?? 0)

const ensureClientOptions = (target: Inbound) => {
  const raw = target as unknown as Record<string, unknown>
  if (clientOptionlessProtocols.includes(target.type)) {
    delete raw.out_json
    delete raw.addrs
    return
  }
  if (raw.out_json == null || typeof raw.out_json !== 'object' || Array.isArray(raw.out_json)) {
    raw.out_json = {}
  }
  if (!Array.isArray(raw.addrs)) {
    raw.addrs = []
  }
}

const changeType = () => {
  const previous = inbound.value as unknown as Record<string, unknown>
  inbound.value = createInbound(inbound.value.type, {
    id: 0,
    tag: String(previous.tag ?? tagSeed.value),
    listen: String(previous.listen ?? '::'),
    listen_port: Number(previous.listen_port || manualListenPort.value || 443),
  }) as Inbound
  ensureClientOptions(inbound.value)
  if (!hasTls.value) {
    tlsTemplate.value = 'none'
  } else if (tlsTemplate.value === 'none') {
    tlsTemplate.value = 'standard'
  }
  side.value = 'server'
}

const addAddr = () => {
  ensureClientOptions(inbound.value)
  inbound.value.addrs?.push({
    server: props.domain.domain || location.hostname,
    server_port: Number(manualListenPort.value || 443),
  })
}

const scrubInbound = (): Record<string, unknown> => {
  const raw = { ...(toRaw(inbound.value) as unknown as Record<string, unknown>) }
  raw.tag = String(raw.tag ?? '').trim() || tagSeed.value.trim() || groupId.value.trim()
  raw.listen = String(raw.listen ?? '').trim() || '::'
  raw.listen_port = listenPortSource.value === 'auto'
    ? localProvided('DomainInboundListenPort')
    : manualListenPort.value
  delete raw.id
  delete raw.tls_id
  delete raw.tls
  delete raw.users
  if (hasClientOptions.value) {
    raw.out_json = raw.out_json && typeof raw.out_json === 'object' && !Array.isArray(raw.out_json)
      ? raw.out_json
      : {}
    raw.addrs = Array.isArray(raw.addrs) ? raw.addrs : []
  } else {
    delete raw.out_json
    delete raw.addrs
  }
  return raw
}

const submit = () => {
  const trimmedGroupId = groupId.value.trim()
  if (!trimmedGroupId) {
    errorMessage.value = i18n.global.t('clusterCenter.domainResources.groupIdRequired').toString()
    return
  }
  if (listenPortSource.value === 'manual' && (manualListenPort.value < 1 || manualListenPort.value > 65535)) {
    errorMessage.value = i18n.global.t('clusterCenter.domainResources.listenPortInvalid').toString()
    return
  }
  if (targetScope.value === 'pick' && selectedTargetNodeIds.value.length === 0) {
    errorMessage.value = i18n.global.t('clusterCenter.domainResources.targetMembersRequired').toString()
    return
  }

  errorMessage.value = ''
  const tlsPayload = createDomainInboundTls(
    hasTls.value ? tlsTemplate.value : 'none',
    tagSeed.value || trimmedGroupId,
    props.domain.domain,
  )
  const payload: CreateDomainInboundResourcePayload = {
    group_id: trimmedGroupId,
    tag_seed: tagSeed.value.trim() || trimmedGroupId,
    prefix: prefix.value.trim(),
    suffix: suffix.value.trim(),
    target_members: buildTargetMembers(),
    inbound: scrubInbound(),
    tls_template: tlsPayload.tls_template,
    tls: tlsPayload.tls,
  }
  emit('submit', payload)
}

watch(() => [props.domain.id, props.mode, props.initialResource?.group_id, props.initialResource?.last_operation_id], resetForm, { immediate: true })
watch(() => props.error, (value) => {
  errorMessage.value = value ?? ''
})
watch(() => props.members, () => {
  const availableNodeIds = new Set((props.members ?? []).map((member) => member.nodeId))
  selectedTargetNodeIds.value = selectedTargetNodeIds.value.filter((nodeId) => availableNodeIds.has(nodeId))
}, { deep: true })
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

.domain-resource-editor__addr-heading {
  align-items: center;
  display: flex;
  gap: 8px;
  margin-top: 12px;
}
</style>
