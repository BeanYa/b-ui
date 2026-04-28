import { afterEach, describe, expect, it, vi } from 'vitest'

describe('api client base URL', () => {
  afterEach(() => {
    vi.resetModules()
    vi.unstubAllGlobals()
  })

  it('uses the deployed app base URL so nested routes call the real API root', async () => {
    vi.stubGlobal('window', { BASE_URL: '/beanui/' } as any)

    const { default: api } = await import('./api')

    expect(api.defaults.baseURL).toBe('/beanui/')
  })

  it('falls back to the dev app base URL when the template placeholder is still present', async () => {
    vi.stubGlobal('window', { BASE_URL: '{{ .BASE_URL }}' } as any)

    const { default: api } = await import('./api')

    expect(api.defaults.baseURL).toBe('/app/')
  })
})
