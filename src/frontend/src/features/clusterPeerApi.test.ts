import api from '@/plugins/api'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { fetchNodeInfo, sendAction } from './clusterPeerApi'

vi.mock('@/plugins/api', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
  },
}))

describe('cluster peer API proxying', () => {
  afterEach(() => {
    vi.clearAllMocks()
    vi.unstubAllGlobals()
  })

  it('loads node info through the local API proxy instead of browser-fetching the remote node', async () => {
    vi.mocked(api.get).mockResolvedValue({
      data: { success: true, msg: '', obj: { actions: ['inbound.list'] } },
    })
    const fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock as any)

    const info = await fetchNodeInfo('node-a')

    expect(info).toEqual({ actions: ['inbound.list'] })
    expect(api.get).toHaveBeenCalledWith('api/cluster/member-info', {
      params: { node_id: 'node-a' },
    })
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('sends actions through the local API proxy instead of exposing peer tokens to the browser', async () => {
    vi.mocked(api.post).mockResolvedValue({
      data: { success: true, msg: '', obj: { status: 'success', action: 'inbound.list' } },
    })
    const fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock as any)

    await sendAction('node-a', {
      schema_version: 1,
      sourceNodeId: '',
      domain: '',
      sentAt: 1,
      signature: '',
      action: 'inbound.list',
      payload: { page: 1, page_size: 10 },
    })

    expect(api.post).toHaveBeenCalledWith('api/cluster/member-action', {
      node_id: 'node-a',
      request: {
        schema_version: 1,
        sourceNodeId: '',
        domain: '',
        sentAt: 1,
        signature: '',
        action: 'inbound.list',
        payload: { page: 1, page_size: 10 },
      },
    })
    expect(fetchMock).not.toHaveBeenCalled()
  })
})
