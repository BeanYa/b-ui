import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const { mockGet } = vi.hoisted(() => ({
  mockGet: vi.fn(),
}))

vi.mock('@/plugins/httputil', () => ({
  default: {
    get: mockGet,
  },
}))

describe('auth store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockGet.mockReset()
  })

  it('loads the current auth state from api/authState', async () => {
    mockGet.mockResolvedValue({
      success: true,
      msg: '',
      obj: {
        username: 'admin',
        isAdmin: true,
      },
    })

    const { default: useAuthStore } = await import('./auth')
    const auth = useAuthStore()

    await auth.loadAuthState()

    expect(mockGet).toHaveBeenCalledWith('api/authState')
    expect(auth.username).toBe('admin')
    expect(auth.isAdmin).toBe(true)
    expect(auth.loaded).toBe(true)
  })

  it('keeps auth state unloaded when api/authState fails', async () => {
    mockGet.mockResolvedValue({
      success: false,
      msg: 'Invalid login',
      obj: null,
    })

    const { default: useAuthStore } = await import('./auth')
    const auth = useAuthStore()

    await auth.loadAuthState()

    expect(auth.username).toBe('')
    expect(auth.isAdmin).toBe(false)
    expect(auth.loaded).toBe(false)
  })
})

describe('Default layout auth bootstrap', () => {
  it('loads auth state on mount only when it has not been loaded yet', () => {
    const source = readFileSync(
      fileURLToPath(new URL('../../layouts/default/Default.vue', import.meta.url)),
      'utf8',
    )

    expect(source).toContain("@/store/modules/auth")
    expect(source).toMatch(/onMounted\(\(\) => \{[\s\S]*if \(!auth\.loaded\) \{[\s\S]*auth\.loadAuthState\(\)[\s\S]*\}[\s\S]*\}\)/)
  })
})
