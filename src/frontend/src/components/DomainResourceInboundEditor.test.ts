import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

describe('DomainResourceInboundEditor source', () => {
  const source = readFileSync(fileURLToPath(new URL('./DomainResourceInboundEditor.vue', import.meta.url)), 'utf8')

  it('uses the same protocol inventory as panel inbounds instead of a restricted domain list', () => {
    expect(source).toContain("import { InTypes, createInbound")
    expect(source).toContain('Object.keys(InTypes)')
    expect(source).toContain('<v-tabs')
    expect(source).toContain("v-if=\"hasClientOptions\"")
    expect(source).toContain("value=\"server\"")
    expect(source).toContain("value=\"client\"")
    expect(source).toContain('<Direct')
    expect(source).toContain('<Shadowsocks')
    expect(source).toContain('<Hysteria')
    expect(source).toContain('<Hysteria2')
    expect(source).toContain('<Naive')
    expect(source).toContain('<ShadowTls')
    expect(source).toContain('<Tuic')
    expect(source).toContain('<Tun')
    expect(source).toContain('<AnyTls')
    expect(source).toContain('<TProxy')
    expect(source).toContain('<Transport')
    expect(source).toContain('<Multiplex')
    expect(source).toContain('<OutJson')
    expect(source).toContain('<Dial')
    expect(source).toContain('<Addr')
    expect(source).not.toContain('DOMAIN_INBOUND_TYPE_OPTIONS')
  })

  it('enables client-side settings for every protocol except direct tun redirect and tproxy', () => {
    expect(source).toContain('clientOptionlessProtocols')
    expect(source).toContain('InTypes.Direct')
    expect(source).toContain('InTypes.Tun')
    expect(source).toContain('InTypes.Redirect')
    expect(source).toContain('InTypes.TProxy')
    expect(source).toContain('const hasClientOptions = computed(() => !clientOptionlessProtocols.includes(inbound.value.type))')
    expect(source).toContain('ensureClientOptions(inbound.value)')
  })

  it('offers target-node generated values for local-only inbound fields', () => {
    expect(source).toContain("'DomainInboundListenPort'")
    expect(source).toContain('listenPortSource')
    expect(source).toContain("localProvided('DomainInboundListenPort')")
    expect(source).toContain('createDomainInboundTls')
  })

  it('builds a structured create payload without requiring hand-written inbound JSON', () => {
    expect(source).toContain('CreateDomainInboundResourcePayload')
    expect(source).toContain('defineEmits')
    expect(source).toContain("emit('submit', payload)")
    expect(source).toContain('group_id')
    expect(source).toContain('tag_seed')
    expect(source).toContain('tls_template')
    expect(source).toContain('if (hasClientOptions.value) {')
    expect(source).toContain("raw.out_json = raw.out_json && typeof raw.out_json === 'object' && !Array.isArray(raw.out_json)")
    expect(source).toContain('raw.addrs = Array.isArray(raw.addrs) ? raw.addrs : []')
    expect(source).not.toContain('advancedJson')
    expect(source).not.toContain('parseDomainResourceJson')
  })

  it('prefills existing resources for update and keeps the group id stable', () => {
    expect(source).toContain('initialResource?: DomainResourceInboundView | null')
    expect(source).toContain("mode?: 'create' | 'update'")
    expect(source).toContain(':disabled="mode === \'update\'"')
    expect(source).toContain('const applyInitialResource = (resource: DomainResourceInboundView) => {')
    expect(source).toContain('parseInboundOptions(resource.options_json)')
    expect(source).toContain('props.initialResource?.group_id')
  })

  it('lets operators broadcast all nodes or pick a target member list', () => {
    expect(source).toContain("targetScope = ref<'all' | 'pick'>('all')")
    expect(source).toContain('selectedTargetNodeIds')
    expect(source).toContain('targetScopeItems')
    expect(source).toContain("$t('clusterCenter.domainResources.targetScope')")
    expect(source).toContain("i18n.global.t('clusterCenter.domainResources.broadcastAll')")
    expect(source).toContain("$t('clusterCenter.domainResources.pickList')")
    expect(source).toContain('v-if="targetScope === \'pick\'"')
    expect(source).toContain('targetMemberItems')
    expect(source).toContain('buildTargetMembers')
    expect(source).toContain('target_members: buildTargetMembers()')
    expect(source).toContain('targetMembersRequired')
    expect(source).toContain('member.isLocal')
    expect(source).toContain("i18n.global.t('clusterCenter.domainResources.localNode')")
  })

  it('does not generate sing-box legacy inbound sniff fields', () => {
    expect(source).not.toContain('raw.sniff')
    expect(source).not.toContain('sniff_override_destination')
    expect(source).not.toContain('sniff_timeout')
    expect(source).not.toContain('domain_strategy')
  })
})
