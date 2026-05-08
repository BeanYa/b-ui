import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

describe('DomainResourceUserEditor source', () => {
  const source = readFileSync(fileURLToPath(new URL('./DomainResourceUserEditor.vue', import.meta.url)), 'utf8')

  it('generates full protocol user config instead of asking for raw JSON', () => {
    expect(source).toContain('randomConfigs')
    expect(source).not.toContain('configJson')
    expect(source).not.toContain('JSON.parse')
  })

  it('uses the local client three-tab editing flow', () => {
    expect(source).toContain('<v-tabs')
    expect(source).toContain("$t('client.basics')")
    expect(source).toContain("$t('client.config')")
    expect(source).toContain("$t('client.links')")
    expect(source).toContain('<v-window')
    expect(source).toContain('value="basics"')
    expect(source).toContain('value="config"')
    expect(source).toContain('value="links"')
  })

  it('renders generated protocol config as editable controls with local-client reset actions', () => {
    expect(source).toContain('protocolConfig')
    expect(source).toContain('shuffleConfigs')
    expect(source).toContain('shuffleConfig')
    expect(source).toContain('clientConfigKeys')
    expect(source).toContain('config: cloneDomainUserConfig(protocolConfig.value)')
    expect(source).not.toContain('previewConfig[key].uuid')
  })

  it('keeps name edits from wiping generated protocol secrets', () => {
    expect(source).toContain('syncProtocolNames')
    expect(source).toContain('updateConfigs(protocolConfig.value')
    expect(source).not.toContain('syncProtocolConfig')
  })

  it('exposes local-client quota, expiry, delayed start, and auto reset controls', () => {
    expect(source).toContain('volumeGiB')
    expect(source).toContain('DatePick')
    expect(source).toContain('delayStart')
    expect(source).toContain('autoReset')
    expect(source).toContain('resetDays')
    expect(source).toContain('resetDaysModel')
    expect(source).toContain('resetDays.value = value ? 1 : 0')
    expect(source).toContain('nextReset.value += (normalized - (resetDays.value || 0)) * 24 * 60 * 60')
    expect(source).toContain("}, { flush: 'sync' })")
    expect(source).toContain("$t('stats.volume')")
    expect(source).toContain("$t('client.delayStart')")
    expect(source).toContain("$t('client.autoReset')")
  })

  it('allows domain users to keep external links and external subscriptions', () => {
    expect(source).toContain('type Link')
    expect(source).toContain('links')
    expect(source).toContain('extLinks')
    expect(source).toContain('subLinks')
    expect(source).toContain("type: 'external'")
    expect(source).toContain("type: 'sub'")
    expect(source).toContain('links: normalizedLinks')
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
    expect(source).toContain('volume: volumeBytes.value')
    expect(source).toContain('expiry: expiry.value')
    expect(source).toContain('delay_start: delayStart.value')
    expect(source).toContain('auto_reset: autoReset.value')
    expect(source).toContain('reset_days: resetDays.value')
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
