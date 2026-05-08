import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

describe('DomainResourceUserEditor source', () => {
  const source = readFileSync(fileURLToPath(new URL('./DomainResourceUserEditor.vue', import.meta.url)), 'utf8')

  it('generates full protocol user config instead of asking for raw JSON', () => {
    expect(source).toContain('createDomainUserConfig')
    expect(source).not.toContain('configJson')
    expect(source).not.toContain('JSON.parse')
  })

  it('renders generated protocol config as editable controls', () => {
    expect(source).toContain('protocolConfig')
    expect(source).toContain('domainUserProtocolFields')
    expect(source).toContain('configInputValue')
    expect(source).toContain('setConfigValue')
    expect(source).toContain('@update:model-value="setConfigValue')
    expect(source).toContain('config: cloneDomainUserConfig(protocolConfig.value)')
    expect(source).not.toContain('previewConfig[key].uuid')
  })

  it('keeps bulk secret controls from wiping unrelated protocol edits', () => {
    expect(source).toContain('syncProtocolSecrets')
    expect(source).toContain('syncProtocolNames')
    expect(source).toContain('watch([secretSources, manualSecrets], syncProtocolSecrets')
    expect(source).not.toContain('watch([secretSources, manualSecrets], syncProtocolConfig')
  })

  it('lets operators choose automatic target-node secrets or manual values', () => {
    expect(source).toContain('secretSources')
    expect(source).toContain('uuidSource')
    expect(source).toContain('passwordSource')
    expect(source).toContain('authSource')
    expect(source).toContain('manualSecrets')
  })

  it('renders domain inbound groups as a chip multi-select instead of raw selector text', () => {
    expect(source).toContain('availableInboundGroups')
    expect(source).toContain('v-select')
    expect(source).toContain('multiple')
    expect(source).toContain('chips')
    expect(source).toContain("$t('clusterCenter.domainResources.userInboundGroups')")
    expect(source).not.toContain('inboundsText')
    expect(source).not.toContain("$t('clusterCenter.domainResources.userInbounds')")
  })

  it('defaults group bindings from the latest domain inbound group', () => {
    expect(source).toContain('selectedInboundGroupIds')
    expect(source).toContain('[props.defaultInboundGroup]')
  })

  it('emits the typed domain user create payload with group bindings', () => {
    expect(source).toContain('CreateDomainUserResourcePayload')
    expect(source).toContain('defineEmits')
    expect(source).toContain("emit('submit', payload)")
    expect(source).toContain('const boundInboundGroupIds = [...selectedInboundGroupIds.value]')
    expect(source).toContain('bound_inbound_group_ids: boundInboundGroupIds')
    expect(source).not.toContain('inbounds:')
  })

  it('supports editing an existing domain user resource', () => {
    expect(source).toContain("mode?: 'create' | 'update'")
    expect(source).toContain('initialResource?: DomainResourceUserView | null')
    expect(source).toContain('props.initialResource')
    expect(source).toContain('hubUserUuid.value = resource.uuid')
    expect(source).toContain("props.mode === 'update'")
  })

  it('shows the node materializations for a deduplicated domain user while editing', () => {
    expect(source).toContain("$t('clusterCenter.domainResources.appliedNodes')")
    expect(source).toContain('appliedNodes')
    expect(source).toContain('nodeDisplayName')
    expect(source).toContain('v-for="node in appliedNodes"')
    expect(source).toContain('domain-resource-editor__applied-node')
  })
})
