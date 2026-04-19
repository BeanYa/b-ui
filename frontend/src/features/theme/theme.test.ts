import { beforeEach, describe, expect, it, vi } from 'vitest'

import { applyThemePreference, startThemeSync, stopThemeSync } from '@/plugins/theme'

describe('theme preference helpers', () => {
  beforeEach(() => {
    stopThemeSync()
    vi.unstubAllGlobals()

    const storage = new Map<string, string>()
    const dataset = {} as Record<string, string>

    vi.stubGlobal('localStorage', {
      clear: () => storage.clear(),
      getItem: (key: string) => storage.get(key) ?? null,
      removeItem: (key: string) => {
        storage.delete(key)
      },
      setItem: (key: string, value: string) => {
        storage.set(key, value)
      },
    })
    vi.stubGlobal('document', {
      documentElement: {
        dataset,
        removeAttribute: (name: string) => {
          const key = name.replace(/^data-/, '').replace(/-([a-z])/g, (_, char: string) => char.toUpperCase())
          delete dataset[key]
        },
      },
    })

    localStorage.clear()
    document.documentElement.removeAttribute('data-theme-preference')
    document.documentElement.removeAttribute('data-theme-name')
  })

  it('stores both preference and resolved theme markers on the document', () => {
    const controller = {
      change: vi.fn(),
      global: { name: { value: 'dark' } },
    }

    applyThemePreference(controller, 'dark')

    expect(controller.change).toHaveBeenCalledWith('dark')
    expect(document.documentElement.dataset.themePreference).toBe('dark')
    expect(document.documentElement.dataset.themeName).toBe('dark')
  })

  it('reacts to system theme changes when preference is system', () => {
    const controller = {
      change: vi.fn(),
      global: { name: { value: 'light' } },
    }
    const listeners: Array<(event: MediaQueryListEvent) => void> = []
    const media = {
      matches: false,
      addEventListener: vi.fn((_: string, cb: (event: MediaQueryListEvent) => void) => listeners.push(cb)),
      removeEventListener: vi.fn(),
    }

    vi.stubGlobal('matchMedia', vi.fn(() => media))

    applyThemePreference(controller, 'system')
    startThemeSync(controller)
    listeners[0]({ matches: true } as MediaQueryListEvent)

    expect(controller.change).toHaveBeenLastCalledWith('dark')
    expect(document.documentElement.dataset.themeName).toBe('dark')
  })
})
