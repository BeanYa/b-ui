import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

describe('Inbounds view source', () => {
  it('visually distinguishes Hub domain-managed inbounds from local panel inbounds', () => {
    const source = readFileSync(fileURLToPath(new URL('./Inbounds.vue', import.meta.url)), 'utf8')

    expect(source).toContain('inbound-card--domain')
    expect(source).toContain('Managed by Hub domain inbound group')
    expect(source).toContain('Domain')
  })
})
