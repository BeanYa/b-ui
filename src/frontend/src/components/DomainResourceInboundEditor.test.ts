import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

describe('DomainResourceInboundEditor source', () => {
  const source = readFileSync(fileURLToPath(new URL('./DomainResourceInboundEditor.vue', import.meta.url)), 'utf8')

  it('uses the same protocol inventory as panel inbounds instead of a restricted domain list', () => {
    expect(source).toContain("import { InTypes, createInbound")
    expect(source).toContain('Object.keys(InTypes)')
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
    expect(source).not.toContain('DOMAIN_INBOUND_TYPE_OPTIONS')
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
    expect(source).not.toContain('advancedJson')
    expect(source).not.toContain('JSON.parse')
  })
})
