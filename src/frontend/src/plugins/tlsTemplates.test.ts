import { beforeAll, describe, expect, it } from 'vitest'

let createTlsPreset: typeof import('./tlsTemplates').createTlsPreset
let createMaterializedTlsPreset: typeof import('./tlsPresetMaterial').createMaterializedTlsPreset

beforeAll(async () => {
  Object.defineProperty(globalThis, 'window', {
    value: {
      crypto: {
        getRandomValues(values: Uint32Array) {
          values[0] = 1
          values[1] = 1
          return values
        },
      },
    },
    configurable: true,
  })

  ;({ createTlsPreset } = await import('./tlsTemplates'))
  ;({ createMaterializedTlsPreset } = await import('./tlsPresetMaterial'))
})

describe('createTlsPreset', () => {
  it('keeps only SNI and ALPN enabled for the standard preset and enables insecure', () => {
    const preset = createTlsPreset('standard')

    expect(preset.server.server_name).toBe('')
    expect(preset.server.alpn).toEqual(['h2', 'http/1.1'])
    expect(preset.server.min_version).toBeUndefined()
    expect(preset.server.max_version).toBeUndefined()
    expect(preset.client.insecure).toBe(true)
  })

  it('keeps only SNI enabled for the hysteria2 preset and enables insecure', () => {
    const preset = createTlsPreset('hysteria2')

    expect(preset.server.server_name).toBe('')
    expect(preset.server.alpn).toBeUndefined()
    expect(preset.server.min_version).toBeUndefined()
    expect(preset.server.max_version).toBeUndefined()
    expect(preset.client.insecure).toBe(true)
  })

  it('leaves max time difference disabled for the reality preset', () => {
    const preset = createTlsPreset('reality')

    expect(preset.server.reality?.max_time_difference).toBeUndefined()
  })

  it('materializes the standard preset with generated certificate text', async () => {
    const preset = await createMaterializedTlsPreset('standard', undefined, {
      async generateTlsKeypair(serverName) {
        expect(serverName).toBe("''")
        return [
          '-----BEGIN PRIVATE KEY-----',
          'private-line',
          '-----END PRIVATE KEY-----',
          '-----BEGIN CERTIFICATE-----',
          'cert-line',
          '-----END CERTIFICATE-----',
        ]
      },
      async generateRealityKeypair() {
        throw new Error('unexpected reality generation')
      },
    })

    expect(preset.server.key).toEqual([
      '-----BEGIN PRIVATE KEY-----',
      'private-line',
      '-----END PRIVATE KEY-----',
    ])
    expect(preset.server.certificate).toEqual([
      '-----BEGIN CERTIFICATE-----',
      'cert-line',
      '-----END CERTIFICATE-----',
    ])
    expect(preset.server.key_path).toBeUndefined()
    expect(preset.server.certificate_path).toBeUndefined()
  })

  it('materializes the hysteria2 preset with generated certificate text', async () => {
    const preset = await createMaterializedTlsPreset('hysteria2', undefined, {
      async generateTlsKeypair() {
        return [
          '-----BEGIN EC PRIVATE KEY-----',
          'private-line',
          '-----END EC PRIVATE KEY-----',
          '-----BEGIN CERTIFICATE-----',
          'cert-line',
          '-----END CERTIFICATE-----',
        ]
      },
      async generateRealityKeypair() {
        throw new Error('unexpected reality generation')
      },
    })

    expect(preset.server.key?.[0]).toBe('-----BEGIN EC PRIVATE KEY-----')
    expect(preset.server.certificate?.[0]).toBe('-----BEGIN CERTIFICATE-----')
  })

  it('materializes the reality preset with generated public and private keys', async () => {
    const preset = await createMaterializedTlsPreset('reality', undefined, {
      async generateTlsKeypair() {
        throw new Error('unexpected tls generation')
      },
      async generateRealityKeypair() {
        return ['PrivateKey: private-key', 'PublicKey: public-key']
      },
    })

    expect(preset.server.reality?.private_key).toBe('private-key')
    expect(preset.client.reality?.public_key).toBe('public-key')
  })
})
