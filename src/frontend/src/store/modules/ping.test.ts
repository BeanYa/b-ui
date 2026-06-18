import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import api from '@/plugins/api'
import type { ExternalResultData, MeshPairResult, MeshResult } from '@/types/ping'

vi.mock('@/plugins/api', () => ({
  default: {
    post: vi.fn(),
    get: vi.fn(),
    put: vi.fn(),
  },
}))

function waitUntil(predicate: () => boolean): Promise<void> {
  return new Promise((resolve, reject) => {
    let attempts = 0
    const tick = () => {
      if (predicate()) {
        resolve()
        return
      }
      attempts += 1
      if (attempts > 30) {
        reject(new Error('timed out waiting for condition'))
        return
      }
      setTimeout(tick, 0)
    }
    tick()
  })
}

describe('ping store mesh stream', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    vi.unstubAllGlobals()
  })

  it('emits mesh pair results before the final stream result resolves', async () => {
    const first: MeshPairResult = {
      source_member_id: 'local-1',
      source_name: 'local',
      target_member_id: 'peer-1',
      target_name: 'peer one',
      method: 'icmp',
      latency_ms: 21,
      success: true,
      error: null,
    }
    const finalResult: MeshResult = {
      domain_id: 'domain.test',
      tested_at: 1710000000,
      results: [first],
    }
    const encoder = new TextEncoder()
    let controller!: ReadableStreamDefaultController<Uint8Array>
    const stream = new ReadableStream<Uint8Array>({
      start(c) {
        controller = c
      },
    })
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(new Response(stream, { status: 200 }))))

    const { usePingStore } = await import('./ping')
    const store = usePingStore()
    const progress: MeshPairResult[] = []
    let resolved = false
    const promise = store.triggerMeshPingStream('domain.test', result => {
      progress.push(result)
    }).then(result => {
      resolved = true
      return result
    })

    controller.enqueue(encoder.encode(`${JSON.stringify({ type: 'result', result: first })}\n`))
    await waitUntil(() => progress.length === 1)

    expect(resolved).toBe(false)
    expect(progress).toEqual([first])
    expect(store.meshResult?.results).toEqual([first])

    controller.enqueue(encoder.encode(`${JSON.stringify({ type: 'done', result: finalResult })}\n`))
    controller.close()

    await expect(promise).resolves.toEqual(finalResult)
    expect(resolved).toBe(true)
    expect(store.meshResult).toEqual(finalResult)
  })

  it('sends current-node outbound ping requests without cluster node filters', async () => {
    const result: ExternalResultData = {
      tested_at: 1710000000,
      results: [],
    }
    vi.mocked(api.post).mockResolvedValueOnce({ data: { success: true, obj: result } })

    const { usePingStore } = await import('./ping')
    const store = usePingStore()

    const request = { direction: 'outbound' as const, source_ids: ['public_dns'], methods: ['tcp'] }

    await expect(store.triggerExternalPing(request)).resolves.toEqual(result)

    expect(api.post).toHaveBeenCalledTimes(1)
    expect(api.post).toHaveBeenCalledWith(
      'api/ping/external',
      request,
      { headers: { 'Content-Type': 'application/json' } },
    )
  })

  it('sends explicit inbound target details', async () => {
    const result: ExternalResultData = {
      tested_at: 1710000000,
      results: [],
    }
    vi.mocked(api.post).mockResolvedValueOnce({ data: { success: true, obj: result } })

    const { usePingStore } = await import('./ping')
    const store = usePingStore()

    const request = {
      direction: 'inbound' as const,
      source_ids: ['check_host'],
      target: { host: 'panel.example.com', port: 443, label: 'Panel' },
    }

    await expect(store.triggerExternalPing(request)).resolves.toEqual(result)

    expect(api.post).toHaveBeenCalledTimes(1)
    expect(api.post).toHaveBeenCalledWith(
      'api/ping/external',
      request,
      { headers: { 'Content-Type': 'application/json' } },
    )
  })

  it('loads external target catalog', async () => {
    const catalog = {
      updated_at: 1710000000,
      providers: [
        {
          provider_id: 'public_dns',
          provider_name: 'Public DNS',
          static: true,
          targets: [
            { id: 'public_dns:cloudflare-dns', label: 'Cloudflare DNS', provider: 'public_dns', group: 'Global', host: '1.1.1.1', port: 53, methods: ['tcp'] },
          ],
        },
      ],
    }
    vi.mocked(api.get).mockResolvedValueOnce({ data: { success: true, obj: catalog } })

    const { usePingStore } = await import('./ping')
    const store = usePingStore()

    await expect(store.loadExternalTargetCatalog()).resolves.toEqual(catalog)
    expect(store.externalTargetCatalog).toEqual(catalog)
    expect(api.get).toHaveBeenCalledWith('api/ping/external/targets')
  })

  it('refreshes selected external target providers', async () => {
    const catalog = { updated_at: 1710000001, providers: [] }
    vi.mocked(api.post).mockResolvedValueOnce({ data: { success: true, obj: catalog } })

    const { usePingStore } = await import('./ping')
    const store = usePingStore()

    await expect(store.refreshExternalTargetCatalog(['zstatic_cdn'])).resolves.toEqual(catalog)
    expect(store.externalTargetCatalog).toEqual(catalog)
    expect(api.post).toHaveBeenCalledWith(
      'api/ping/external/targets/refresh',
      { provider_ids: ['zstatic_cdn'] },
      { headers: { 'Content-Type': 'application/json' } },
    )
  })
})
