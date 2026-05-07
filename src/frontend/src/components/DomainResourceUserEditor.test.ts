import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

describe('DomainResourceUserEditor source', () => {
  const source = readFileSync(fileURLToPath(new URL('./DomainResourceUserEditor.vue', import.meta.url)), 'utf8')

  it('generates full protocol user config instead of asking for raw JSON', () => {
    expect(source).toContain('createDomainUserConfig')
    expect(source).toContain('randomConfigs')
    expect(source).not.toContain('configJson')
    expect(source).not.toContain('JSON.parse')
  })

  it('lets operators choose automatic target-node secrets or manual values', () => {
    expect(source).toContain('secretSources')
    expect(source).toContain('uuidSource')
    expect(source).toContain('passwordSource')
    expect(source).toContain('authSource')
    expect(source).toContain('manualSecrets')
  })

  it('emits the typed domain user create payload', () => {
    expect(source).toContain('CreateDomainUserResourcePayload')
    expect(source).toContain('defineEmits')
    expect(source).toContain("emit('submit', payload)")
    expect(source).toContain('inbounds')
  })
})
