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
          primary: '#177ddc',
          secondary: '#101828',
          accent: '#ff6363',
          background: '#edf1f7',
          surface: '#ffffff',
          'surface-bright': '#f8faff',
          'surface-light': '#eef3f8',
          error: '#ff6363',
          info: '#55b3ff',
          success: '#5fc992',
          warning: '#ffbc33',
        },
      },
      dark: {
        colors: {
          primary: '#55b3ff',
          secondary: '#101111',
          accent: '#ff6363',
          background: '#07080a',
          surface: '#101111',
          'surface-bright': '#15171a',
          'surface-light': '#1b1c1e',
          error: '#ff6363',
          info: '#55b3ff',
          success: '#5fc992',
          warning: '#ffbc33',
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
