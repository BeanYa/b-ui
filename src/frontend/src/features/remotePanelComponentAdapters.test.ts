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
    const tlsPresetMaterialSource = readSource('../plugins/tlsPresetMaterial.ts')

    expect(tlsSource).not.toContain("api/keypairs")
    expect(tlsSource).toContain('Data().keypairs')
    expect(endpointSource).not.toContain("api/keypairs")
    expect(endpointSource).toContain('Data().keypairs')
    expect(tlsPresetMaterialSource).not.toContain("api/keypairs")
    expect(tlsPresetMaterialSource).toContain('Data().keypairs')
  })

  it('routes stats graph data through the Data store', () => {
    const statsSource = readSource('../layouts/modals/Stats.vue')

    expect(statsSource).not.toContain("api/stats")
    expect(statsSource).toContain('Data().stats')
  })

  it('keeps TLS template domain combobox selections as strings', () => {
    const tlsSource = readSource('../layouts/modals/Tls.vue')

    expect(tlsSource).toContain('v-model="serverName"')
    expect(tlsSource).toContain('v-model="realityHandshakeServer"')
    expect(tlsSource.match(/:return-object="false"/g)?.length).toBeGreaterThanOrEqual(2)
    expect(tlsSource).toContain('normalizeDomainSelection')
  })

  it('generates client QR codes locally from returned link text', () => {
    const qrSource = readSource('../layouts/modals/QrCode.vue')

    expect(qrSource).toContain("import QrcodeVue from 'qrcode.vue'")
    expect(qrSource).toContain('<QrcodeVue :value="l.uri"')
    expect(qrSource).not.toContain('api/qrcode')
    expect(qrSource).toContain('finally')
    expect(qrSource).toContain('this.loading = false')
  })
})
