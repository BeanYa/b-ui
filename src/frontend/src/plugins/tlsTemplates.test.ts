import { beforeAll, describe, expect, it } from 'vitest'

let createTlsPreset: typeof import('./tlsTemplates').createTlsPreset

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
})
