import { beforeEach, describe, expect, it, vi } from 'vitest'

const remotePanelPartial = vi.fn()
const httpGet = vi.fn()

let remoteState = {
  isRemote: true,
  remoteNodeId: 'node-a',
  remoteHostname: 'node-a.example.com',
}

vi.mock('@/features/remotePanelApi', () => ({
  remotePanelPartial,
}))

vi.mock('@/plugins/httputil', () => ({
  default: {
    get: httpGet,
  },
}))

vi.mock('@/store/modules/data', () => ({
  default: () => ({
    isRemote: () => remoteState.isRemote,
    remoteNodeId: remoteState.remoteNodeId,
    remoteHostname: remoteState.remoteHostname,
    keypairs: vi.fn(),
  }),
}))

describe('default TLS preset material provider in remote node mode', () => {
  beforeEach(() => {
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
    remotePanelPartial.mockReset()
    httpGet.mockReset()
    remoteState = {
      isRemote: true,
      remoteNodeId: 'node-a',
      remoteHostname: 'node-a.example.com',
    }
  })

  it('reads panel certificate settings from the selected remote node', async () => {
    remotePanelPartial.mockResolvedValue({
      settings: {
        webDomain: 'target.example.com',
        webCertFile: '/target/fullchain.pem',
        webKeyFile: '/target/privkey.pem',
      },
    })

    const { createMaterializedTlsPreset } = await import('./tlsPresetMaterial')
    const preset = await createMaterializedTlsPreset('standard-cert')

    expect(remotePanelPartial).toHaveBeenCalledWith('node-a', {
      object: 'settings',
      hostname: 'node-a.example.com',
    })
    expect(httpGet).not.toHaveBeenCalled()
    expect(preset.server.server_name).toBe('target.example.com')
    expect(preset.server.certificate_path).toBe('/target/fullchain.pem')
    expect(preset.server.key_path).toBe('/target/privkey.pem')
  })
})
