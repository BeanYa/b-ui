import { afterEach, describe, expect, it, vi } from 'vitest'
import { fetchNodeInfo, sendAction } from './clusterPeerApi'

describe('cluster peer API URL building', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('loads node info without adding a double slash after a mounted panel base URL', async () => {
    const fetchMock = vi.fn(async () => ({
      ok: true,
      json: async () => ({ actions: [] }),
    }))
    vi.stubGlobal('fetch', fetchMock as any)

    await fetchNodeInfo('https://node.example.com/beanui/', 'peer-token')

    expect(fetchMock).toHaveBeenCalledWith('https://node.example.com/beanui/_cluster/v1/info', {
      headers: { 'X-Cluster-Token': 'peer-token' },
    })
  })

  it('sends actions without adding a double slash after a mounted panel base URL', async () => {
    const fetchMock = vi.fn(async () => ({
      ok: true,
      json: async () => ({ status: 'success' }),
    }))
    vi.stubGlobal('fetch', fetchMock as any)

    await sendAction('https://node.example.com/beanui/', 'peer-token', {
      schema_version: 1,
      sourceNodeId: '',
      domain: '',
      sentAt: 1,
      signature: '',
      action: 'inbound.list',
      payload: { page: 1, page_size: 10 },
    })

    expect(fetchMock).toHaveBeenCalledWith('https://node.example.com/beanui/_cluster/v1/action', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Cluster-Token': 'peer-token',
      },
      body: JSON.stringify({
        schema_version: 1,
        sourceNodeId: '',
        domain: '',
        sentAt: 1,
        signature: '',
        action: 'inbound.list',
        payload: { page: 1, page_size: 10 },
      }),
    })
  })
})
