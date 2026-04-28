import api from '@/plugins/api'
import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  remotePanelCheckOutbound,
  remotePanelKeypairs,
  remotePanelLoad,
  remotePanelSave,
} from './remotePanelApi'

vi.mock('@/plugins/api', () => ({
  default: {
    post: vi.fn(),
  },
}))

describe('remote panel API actions', () => {
  afterEach(() => {
    vi.clearAllMocks()
  })

  it('sends panel.load with hostname through the cluster member action proxy', async () => {
    vi.mocked(api.post).mockResolvedValue({
      data: {
        success: true,
        msg: '',
        obj: { status: 'success', action: 'panel.load', data: { inbounds: [] } },
      },
    })

    const data = await remotePanelLoad('node-a', { lu: 123, hostname: 'node.example.com' })

    expect(data).toEqual({ inbounds: [] })
    expect(api.post).toHaveBeenCalledWith(
      'api/cluster/member-action',
      {
        node_id: 'node-a',
        request: expect.objectContaining({
          schema_version: 1,
          action: 'panel.load',
          payload: { lu: 123, hostname: 'node.example.com' },
        }),
      },
      { headers: { 'Content-Type': 'application/json' } },
    )
  })

  it('throws remote action error messages', async () => {
    vi.mocked(api.post).mockResolvedValue({
      data: {
        success: true,
        msg: '',
        obj: { status: 'error', action: 'panel.save', error_message: 'bad tag' },
      },
    })

    await expect(remotePanelSave('node-a', {
      object: 'inbounds',
      action: 'new',
      data: { tag: '' },
      hostname: 'node.example.com',
    })).rejects.toThrow('bad tag')
  })

  it('uses panel.keypairs and panel.checkOutbound action names', async () => {
    vi.mocked(api.post)
      .mockResolvedValueOnce({
        data: {
          success: true,
          msg: '',
          obj: { status: 'success', action: 'panel.keypairs', data: ['PrivateKey: abc'] },
        },
      })
      .mockResolvedValueOnce({
        data: {
          success: true,
          msg: '',
          obj: { status: 'success', action: 'panel.checkOutbound', data: { OK: true, Delay: 10 } },
        },
      })

    await expect(remotePanelKeypairs('node-a', { k: 'reality' })).resolves.toEqual(['PrivateKey: abc'])
    await expect(remotePanelCheckOutbound('node-a', { tag: 'proxy-a' })).resolves.toEqual({ OK: true, Delay: 10 })

    expect(api.post).toHaveBeenNthCalledWith(
      1,
      'api/cluster/member-action',
      expect.objectContaining({
        request: expect.objectContaining({ action: 'panel.keypairs', payload: { k: 'reality' } }),
      }),
      expect.any(Object),
    )
    expect(api.post).toHaveBeenNthCalledWith(
      2,
      'api/cluster/member-action',
      expect.objectContaining({
        request: expect.objectContaining({ action: 'panel.checkOutbound', payload: { tag: 'proxy-a' } }),
      }),
      expect.any(Object),
    )
  })
})
