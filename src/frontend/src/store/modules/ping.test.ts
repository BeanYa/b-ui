import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { MeshPairResult, MeshResult } from '@/types/ping'

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
})
