import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

function readSource(relativePath: string): string {
  return readFileSync(fileURLToPath(new URL(relativePath, import.meta.url)), 'utf8')
}

describe('remote panel component adapters', () => {
  it('routes outbound checks and link conversion through the Data store', () => {
    const outboundsSource = readSource('../views/Outbounds.vue')
    const outboundModalSource = readSource('../layouts/modals/Outbound.vue')

    expect(outboundsSource).not.toContain("api/checkOutbound")
    expect(outboundsSource).toContain('Data().checkOutbound')
    expect(outboundModalSource).not.toContain("api/linkConvert")
    expect(outboundModalSource).toContain('Data().linkConvert')
  })

  it('routes generated keypairs through the Data store', () => {
    const tlsSource = readSource('../layouts/modals/Tls.vue')
    const endpointSource = readSource('../layouts/modals/Endpoint.vue')

    expect(tlsSource).not.toContain("api/keypairs")
    expect(tlsSource).toContain('Data().keypairs')
    expect(endpointSource).not.toContain("api/keypairs")
    expect(endpointSource).toContain('Data().keypairs')
  })

  it('routes stats graph data through the Data store', () => {
    const statsSource = readSource('../layouts/modals/Stats.vue')

    expect(statsSource).not.toContain("api/stats")
    expect(statsSource).toContain('Data().stats')
  })
})
