export type ThemePreference = 'light' | 'dark' | 'system'

type ThemeController = {
  change: (name: 'light' | 'dark') => void
  global: {
    name: {
      value: string
    }
  }
}

const THEME_STORAGE_KEY = 'theme'
const THEME_QUERY = '(prefers-color-scheme: dark)'

let activeThemeController: ThemeController | null = null
let activeThemeMediaQuery: MediaQueryList | null = null
let activeThemeListener: ((event: MediaQueryListEvent) => void) | null = null

const getThemeMediaQuery = (): MediaQueryList => globalThis.matchMedia(THEME_QUERY)

export const getThemePreference = (): ThemePreference => {
  const stored = localStorage.getItem(THEME_STORAGE_KEY)

  if (stored === 'light' || stored === 'dark' || stored === 'system') {
    return stored
  }

  return 'system'
}

export const resolveThemeName = (
  preference: ThemePreference = getThemePreference()
): 'light' | 'dark' => {
  if (preference === 'system') {
    return getThemeMediaQuery().matches ? 'dark' : 'light'
  }

  return preference
}

const syncDocumentTheme = (preference: ThemePreference) => {
  document.documentElement.dataset.themePreference = preference
  document.documentElement.dataset.themeName = resolveThemeName(preference)
}

export const applyThemePreference = (
  theme: ThemeController,
  preference: ThemePreference,
  persist = true
) => {
  theme.change(resolveThemeName(preference))
  syncDocumentTheme(preference)

  if (persist) {
    localStorage.setItem(THEME_STORAGE_KEY, preference)
  }
}

export const stopThemeSync = () => {
  if (activeThemeMediaQuery && activeThemeListener) {
    activeThemeMediaQuery.removeEventListener('change', activeThemeListener)
  }

  activeThemeController = null
  activeThemeMediaQuery = null
  activeThemeListener = null
}

export const startThemeSync = (theme: ThemeController) => {
  stopThemeSync()

  activeThemeController = theme
  activeThemeMediaQuery = getThemeMediaQuery()
  activeThemeListener = (event: MediaQueryListEvent) => {
    if (getThemePreference() !== 'system' || !activeThemeController) return

    const nextTheme = event.matches ? 'dark' : 'light'
    activeThemeController.change(nextTheme)
    document.documentElement.dataset.themeName = nextTheme
  }

  activeThemeMediaQuery.addEventListener('change', activeThemeListener)
}
