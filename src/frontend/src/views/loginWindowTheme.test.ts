import { describe, expect, it } from 'vitest'

import { getLoginWindowThemeModel } from '@/views/loginWindowTheme'

describe('getLoginWindowThemeModel', () => {
  it('returns the light root and surface modifiers for light theme', () => {
    expect(getLoginWindowThemeModel('light')).toEqual({
      rootClass: 'login-shell--light',
      surfaceClass: 'login-window--light',
    })
  })

  it('returns the dark root and surface modifiers for dark theme', () => {
    expect(getLoginWindowThemeModel('dark')).toEqual({
      rootClass: 'login-shell--dark',
      surfaceClass: 'login-window--dark',
    })
  })

  it('keeps distinct root and surface classes for each supported theme', () => {
    const lightModel = getLoginWindowThemeModel('light')
    const darkModel = getLoginWindowThemeModel('dark')

    expect(lightModel.rootClass).not.toBe(darkModel.rootClass)
    expect(lightModel.surfaceClass).not.toBe(darkModel.surfaceClass)
    expect(lightModel).not.toHaveProperty('subtitle')
    expect(darkModel).not.toHaveProperty('subtitle')
  })

  it('exposes the exact window surface modifiers used by Login.vue styles', () => {
    expect(getLoginWindowThemeModel('light').surfaceClass).toBe('login-window--light')
    expect(getLoginWindowThemeModel('dark').surfaceClass).toBe('login-window--dark')
  })
})
