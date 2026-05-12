/**
 * plugins/vuetify.ts
 *
 * Framework documentation: https://vuetifyjs.com`
 */

// Styles
import '@mdi/font/css/materialdesignicons.css'
import 'vuetify/styles/main.css'

import { fa, en, vi, zhHans, zhHant, ru } from 'vuetify/locale'

// Composables
import { createVuetify } from 'vuetify'
import { getThemePreference, resolveThemeName } from './theme'

// https://vuetifyjs.com/en/introduction/why-vuetify/#feature-guides
export default createVuetify({
  defaults: {
    VRow: { density: 'compact' },
    VAppBar: {
      flat: true,
      height: 76,
    },
    VCard: {
      rounded: 'xl',
    },
    VBtn: {
      rounded: 'pill',
      style: 'letter-spacing: 0.3px; text-transform: none; font-weight: 600;',
    },
    VChip: {
      rounded: 'lg',
    },
    VTextField: {
      variant: 'solo-filled',
      rounded: 'xl',
    },
    VSelect: {
      variant: 'solo-filled',
      rounded: 'xl',
    },
    VCombobox: {
      variant: 'solo-filled',
      rounded: 'xl',
    },
    VTextarea: {
      variant: 'solo-filled',
      rounded: 'xl',
    },
  },
  theme: {
    defaultTheme: resolveThemeName(getThemePreference()),
    themes: {
      light: {
        colors: {
          primary: '#0a0a0a',
          secondary: '#1a3a3a',
          accent: '#ff4d8b',
          background: '#fffaf0',
          surface: '#faf5e8',
          'surface-bright': '#fffaf0',
          'surface-light': '#f5f0e0',
          error: '#ef4444',
          info: '#b8a4ed',
          success: '#22c55e',
          warning: '#e8b94a',
        },
      },
      dark: {
        colors: {
          primary: '#fffaf0',
          secondary: '#1a3a3a',
          accent: '#ff4d8b',
          background: '#0a1a1a',
          surface: '#1a2a2a',
          'surface-bright': '#233534',
          'surface-light': '#132322',
          error: '#ff6b5a',
          info: '#b8a4ed',
          success: '#a4d4c5',
          warning: '#e8b94a',
        },
      },
    },
  },
  locale: {
    locale: localStorage.getItem("locale") ?? 'en',
    fallback: 'en',
    messages: { en, fa, vi, zhHans, zhHant, ru },
  },
})
