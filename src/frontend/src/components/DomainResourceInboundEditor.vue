<template>
  <v-card class="app-card-shell domain-resource-editor">
    <v-card-title>{{ $t('clusterCenter.domainResources.inboundDialogTitle') }}</v-card-title>
    <v-card-text class="domain-resource-editor__body">
      <v-row>
        <v-col cols="12" md="4">
          <v-text-field v-model="groupId" :label="$t('clusterCenter.domainResources.groupId')" hide-details />
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
        <v-row>
          <v-col cols="12" md="4">
            <v-switch v-model="sniff" :label="$t('clusterCenter.domainResources.sniff')" color="primary" hide-details />
          </v-col>
        </v-row>
      </v-card>

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
import type { CreateDomainInboundResourcePayload } from '@/features/domainResourcesApi'
import {
  createDomainInboundTls,
  localProvided,
  sanitizeDomainResourcePart,
  type DomainInboundTlsTemplate,
} from '@/features/domainResourceLocalProvided'
import { i18n } from '@/locales'
import { InTypes, createInbound, type Inbound } from '@/types/inbounds'
import type { ClusterDomain } from '@/types/clusters'

const props = defineProps<{
  domain: ClusterDomain
  loading?: boolean
  error?: string
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
const sniff = ref(true)
const tlsTemplate = ref<DomainInboundTlsTemplate>('standard')
const errorMessage = ref('')

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

const protocolItems = Object.keys(InTypes).map((key, index) => ({
  title: key,
  value: Object.values(InTypes)[index],
}))

const sourceItems = computed(() => [
  { title: i18n.global.t('clusterCenter.domainResources.autoByTarget').toString(), value: 'auto' },
  { title: i18n.global.t('clusterCenter.domainResources.manualValue').toString(), value: 'manual' },
])

const tlsTemplateItems = computed(() => [
  { title: i18n.global.t('none').toString(), value: 'none' },
  { title: i18n.global.t('tls.presets.standard').toString(), value: 'standard' },
  { title: i18n.global.t('tls.presets.hysteria2').toString(), value: 'hysteria2' },
  { title: i18n.global.t('tls.presets.reality').toString(), value: 'reality' },
  { title: i18n.global.t('tls.presets.standardCert').toString(), value: 'standard-cert' },
  { title: i18n.global.t('tls.presets.hysteria2Cert').toString(), value: 'hysteria2-cert' },
])

const hasTls = computed(() => hasTlsProtocols.includes(inbound.value.type))

const resetForm = () => {
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
  listenPortSource.value = 'auto'
  manualListenPort.value = 443
  sniff.value = true
  tlsTemplate.value = 'standard'
  errorMessage.value = ''
}

const changeType = () => {
  const previous = inbound.value as Record<string, unknown>
  inbound.value = createInbound(inbound.value.type, {
    id: 0,
    tag: String(previous.tag ?? tagSeed.value),
    listen: String(previous.listen ?? '::'),
    listen_port: Number(previous.listen_port || manualListenPort.value || 443),
  }) as Inbound
  if (!hasTls.value) {
    tlsTemplate.value = 'none'
  } else if (tlsTemplate.value === 'none') {
    tlsTemplate.value = 'standard'
  }
}

const scrubInbound = (): Record<string, unknown> => {
  const raw = { ...(toRaw(inbound.value) as Record<string, unknown>) }
  raw.tag = String(raw.tag ?? '').trim() || tagSeed.value.trim() || groupId.value.trim()
  raw.listen = String(raw.listen ?? '').trim() || '::'
  raw.listen_port = listenPortSource.value === 'auto'
    ? localProvided('DomainInboundListenPort')
    : manualListenPort.value
  if (sniff.value) {
    raw.sniff = true
    raw.sniff_override_destination = true
  } else {
    delete raw.sniff
    delete raw.sniff_override_destination
  }
  delete raw.id
  delete raw.tls_id
  delete raw.tls
  delete raw.out_json
  delete raw.addrs
  delete raw.users
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
    inbound: scrubInbound(),
    tls_template: tlsPayload.tls_template,
    tls: tlsPayload.tls,
  }
  emit('submit', payload)
}

watch(() => props.domain.id, resetForm, { immediate: true })
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
</style>
