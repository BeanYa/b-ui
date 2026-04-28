import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const {
  mockHttpGet,
  mockHttpPost,
  mockRemoteLoad,
  mockRemotePartial,
  mockRemoteSave,
  mockRemoteKeypairs,
  mockRemoteLinkConvert,
  mockRemoteCheckOutbound,
  mockRemoteStats,
} = vi.hoisted(() => ({
  mockHttpGet: vi.fn(),
  mockHttpPost: vi.fn(),
  mockRemoteLoad: vi.fn(),
  mockRemotePartial: vi.fn(),
  mockRemoteSave: vi.fn(),
  mockRemoteKeypairs: vi.fn(),
  mockRemoteLinkConvert: vi.fn(),
  mockRemoteCheckOutbound: vi.fn(),
  mockRemoteStats: vi.fn(),
}))

vi.mock('@/plugins/httputil', () => ({
  default: {
    get: mockHttpGet,
    post: mockHttpPost,
  },
}))

vi.mock('@/features/remotePanelApi', () => ({
  remotePanelLoad: mockRemoteLoad,
  remotePanelPartial: mockRemotePartial,
  remotePanelSave: mockRemoteSave,
  remotePanelKeypairs: mockRemoteKeypairs,
  remotePanelLinkConvert: mockRemoteLinkConvert,
  remotePanelCheckOutbound: mockRemoteCheckOutbound,
  remotePanelStats: mockRemoteStats,
}))

vi.mock('notivue', () => ({
  push: {
    success: vi.fn(),
    error: vi.fn(),
  },
}))

describe('data store remote node mode', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.stubGlobal('localStorage', {
      getItem: vi.fn(() => null),
      setItem: vi.fn(),
      removeItem: vi.fn(),
    })
    mockHttpGet.mockReset()
    mockHttpPost.mockReset()
    mockRemoteLoad.mockReset()
    mockRemotePartial.mockReset()
    mockRemoteSave.mockReset()
    mockRemoteKeypairs.mockReset()
    mockRemoteLinkConvert.mockReset()
    mockRemoteCheckOutbound.mockReset()
    mockRemoteStats.mockReset()
  })

  it('loads remote panel data instead of local api/load when remote mode is active', async () => {
    mockRemoteLoad.mockResolvedValue({
      inbounds: [{ id: 1, tag: 'remote-in' }],
      onlines: { inbound: ['remote-in'] },
    })

    const { default: Data } = await import('./data')
    const data = Data()

    data.enterRemoteNode('node-a', 'https://node.example.com:8443/base/')
    await data.loadData()

    expect(mockRemoteLoad).toHaveBeenCalledWith('node-a', {
      hostname: 'node.example.com',
    })
    expect(mockHttpGet).not.toHaveBeenCalledWith('api/load', expect.anything())
    expect(data.inbounds).toEqual([{ id: 1, tag: 'remote-in' }])
    expect(data.onlines.inbound).toEqual(['remote-in'])
  })

  it('serializes remote load cursors as strings for panel.load', async () => {
    mockRemoteLoad.mockResolvedValue({ onlines: {} })

    const { default: Data } = await import('./data')
    const data = Data()

    data.enterRemoteNode('node-a', 'https://node.example.com')
    data.lastLoad = 123
    await data.loadData()

    expect(mockRemoteLoad).toHaveBeenCalledWith('node-a', {
      lu: '123',
      hostname: 'node.example.com',
    })
  })

  it('saves through panel.save and refreshes store data', async () => {
    mockRemoteSave.mockResolvedValue({
      inbounds: [{ id: 2, tag: 'created' }],
      onlines: {},
    })

    const { default: Data } = await import('./data')
    const data = Data()

    data.enterRemoteNode('node-a', 'https://node.example.com')
    const success = await data.save('inbounds', 'new', { tag: 'created' }, [1])

    expect(success).toBe(true)
    expect(mockRemoteSave).toHaveBeenCalledWith('node-a', {
      object: 'inbounds',
      action: 'new',
      data: { tag: 'created' },
      initUsers: [1],
      hostname: 'node.example.com',
    })
    expect(data.inbounds).toEqual([{ id: 2, tag: 'created' }])
  })

  it('loads remote partial clients and exits back to local mode', async () => {
    mockRemotePartial.mockResolvedValue({ clients: [{ id: 7, name: 'alice' }] })
    mockHttpGet.mockResolvedValue({ success: true, obj: { onlines: {} } })

    const { default: Data } = await import('./data')
    const data = Data()

    data.enterRemoteNode('node-a', 'https://node.example.com')
    await expect(data.loadClients(7)).resolves.toEqual({ id: 7, name: 'alice' })
    expect(mockRemotePartial).toHaveBeenCalledWith('node-a', {
      object: 'clients',
      id: '7',
      hostname: 'node.example.com',
    })

    data.exitRemoteNode()
    await data.loadData()

    expect(mockHttpGet).toHaveBeenCalledWith('api/load', {})
  })

  it('routes utility calls through panel actions in remote mode', async () => {
    mockRemoteKeypairs.mockResolvedValue(['PrivateKey: abc'])
    mockRemoteLinkConvert.mockResolvedValue({ tag: 'converted' })
    mockRemoteCheckOutbound.mockResolvedValue({ OK: true })
    mockRemoteStats.mockResolvedValue([{ traffic: 1 }])

    const { default: Data } = await import('./data')
    const data = Data()

    data.enterRemoteNode('node-a', 'https://node.example.com')

    await expect(data.keypairs('reality')).resolves.toEqual(['PrivateKey: abc'])
    await expect(data.linkConvert('vless://example')).resolves.toEqual({ tag: 'converted' })
    await expect(data.checkOutbound('proxy-a')).resolves.toEqual({ OK: true })
    await expect(data.stats('user', 'alice', 1)).resolves.toEqual([{ traffic: 1 }])
  })
})
