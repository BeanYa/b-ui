import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const { mockPost } = vi.hoisted(() => ({
  mockPost: vi.fn(),
}))

vi.mock('@/plugins/httputil', () => ({
  default: {
    get: vi.fn(),
    post: mockPost,
  },
}))

describe('cluster store scatter tasks', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockPost.mockReset()
  })

  it('creates scatter tasks with a JSON request body', async () => {
    mockPost.mockResolvedValue({
      success: true,
      msg: '',
      obj: {
        taskId: 'task-1',
        taskType: 'mesh.latency',
        status: 'queued',
        scope: 'domain',
        progress: '0/1',
        createdAt: '2026-05-03T00:00:00Z',
      },
    })

    const { useClusterStore } = await import('./cluster')
    const store = useClusterStore()

    await store.createScatterTask('domain.example', 'mesh.latency', 'domain', {})

    expect(mockPost).toHaveBeenCalledWith(
      'api/cluster/domains/domain.example/tasks',
      {
        taskType: 'mesh.latency',
        scope: 'domain',
        params: {},
      },
      {
        headers: { 'Content-Type': 'application/json' },
      },
    )
  })
})
