<template>
  <v-app-bar class="app-bar-shell" height="104">
    <div class="app-bar-shell__inner">
      <div class="app-bar-shell__leading">
        <v-btn
          class="app-bar-shell__nav-btn"
          :icon="isMobile ? 'mdi-menu' : (collapsed ? 'mdi-menu-open' : 'mdi-menu')"
          variant="text"
          @click="$emit('toggleDrawer')"
        />
        <div class="app-bar-shell__brand-mark">
          <v-img src="@/assets/logo.svg" width="26" />
        </div>
        <div class="app-bar-shell__headline">
          <span class="app-bar-shell__eyebrow">{{ pageSection }}</span>
          <v-app-bar-title :text="pageTitle" class="app-bar-shell__title" />
          <div class="app-bar-shell__meta">
            <span>{{ hostLabel }}</span>
            <span>{{ route.path }}</span>
            <span>{{ activeThemeLabel }}</span>
          </div>
        </div>
      </div>

      <div class="app-bar-shell__actions">
        <div class="app-bar-shell__status">
          <span class="app-bar-shell__status-dot"></span>
          <span>{{ consoleStatus }}</span>
        </div>
        <div class="app-bar-shell__context">
          <span class="app-bar-shell__context-chip">{{ localeLabel }}</span>
          <span class="app-bar-shell__context-chip">{{ runtimeLabel }}</span>
        </div>
        <v-menu>
          <template #activator="{ props }">
            <v-btn class="app-bar-shell__icon" icon v-bind="props" variant="text">
              <v-icon>mdi-translate</v-icon>
            </v-btn>
          </template>
          <v-list>
            <v-list-item
              v-for="lang in languages"
              :key="lang.value"
              :active="isActiveLocale(lang.value)"
              @click="changeLocale(lang.value)"
            >
              <v-list-item-title>{{ lang.title }}</v-list-item-title>
            </v-list-item>
          </v-list>
        </v-menu>
        <v-menu>
          <template #activator="{ props }">
            <v-btn class="app-bar-shell__icon" icon v-bind="props" variant="text">
              <v-icon>mdi-theme-light-dark</v-icon>
            </v-btn>
          </template>
          <v-list>
            <v-list-item
              v-for="themeOption in themes"
              :key="themeOption.value"
              :active="isActiveTheme(themeOption.value)"
              :prepend-icon="themeOption.icon"
              @click="changeTheme(themeOption.value)"
            >
              <v-list-item-title>{{ $t(`theme.${themeOption.value}`) }}</v-list-item-title>
            </v-list-item>
          </v-list>
        </v-menu>
      </div>
    </div>
  </v-app-bar>
</template>

<script lang="ts" setup>
import { computed, onBeforeUnmount, onMounted } from 'vue'
import { useLocale, useTheme } from 'vuetify'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { languages } from '@/locales'
import { applyThemePreference, getThemePreference, startThemeSync, stopThemeSync, type ThemePreference } from '@/plugins/theme'

defineProps(['isMobile', 'collapsed'])

const route = useRoute()
const { locale: i18nLocale, t } = useI18n()
const vuetifyLocale = useLocale()
const theme = useTheme()

const pageTitle = computed(() => t(String(route.name)))
const hostLabel = computed(() => document.location.hostname || 'localhost')
const activeThemeLabel = computed(() => `Theme · ${t(`theme.${getThemePreference()}`)}`)
const localeLabel = computed(() => `Locale · ${i18nLocale.value.toUpperCase()}`)
const runtimeLabel = computed(() => `Segmented workspace`)
const consoleStatus = computed(() => t('main.stats.title'))
const pageSection = computed(() => {
  if (route.path === '/') return 'Overview Workspace'
  if (route.path === '/clients') return 'Inventory Workspace'
  if (route.path === '/settings' || route.path === '/basics' || route.path === '/dns') return 'Configuration Workspace'
  return 'Catalog Workspace'
})

const themes = [
  { value: 'light', icon: 'mdi-white-balance-sunny' },
  { value: 'dark', icon: 'mdi-moon-waning-crescent' },
  { value: 'system', icon: 'mdi-laptop' },
]

const changeLocale = (localeValue: string) => {
  i18nLocale.value = localeValue
  vuetifyLocale.current.value = localeValue
  localStorage.setItem('locale', localeValue)
  window.location.reload()
}

const isActiveLocale = (localeValue: string) => i18nLocale.value === localeValue

const changeTheme = (themeValue: string) => {
  applyThemePreference(theme, themeValue as ThemePreference)
}

const isActiveTheme = (themeValue: string) => getThemePreference() === themeValue

onMounted(() => {
  applyThemePreference(theme, getThemePreference(), false)
  startThemeSync(theme)
})

onBeforeUnmount(() => {
  stopThemeSync()
})
</script>

<style scoped>
.app-bar-shell {
  backdrop-filter: none !important;
  background: transparent !important;
  border-color: transparent !important;
  box-shadow: none !important;
  padding: 14px 18px 0;
}

.app-bar-shell :deep(.v-toolbar__content),
.app-bar-shell :deep(.v-toolbar__extension) {
  background: transparent !important;
  padding: 0 !important;
}

.app-bar-shell :deep(.v-toolbar__overlay) {
  display: none;
}

.app-bar-shell__inner {
  align-items: center;
  border: 1px solid var(--app-border-1);
  border-radius: 30px;
  box-shadow: var(--app-shadow-ring), var(--app-shadow-panel);
  display: flex;
  gap: 18px;
  justify-content: space-between;
  min-height: 82px;
  overflow: hidden;
  padding: 14px 18px;
  position: relative;
  width: 100%;
}

.app-bar-shell__inner::before {
  background:
    radial-gradient(circle at 0% 0%, color-mix(in srgb, var(--app-state-danger) 10%, transparent), transparent 22%),
    linear-gradient(120deg, color-mix(in srgb, #ffffff 5%, transparent), transparent 30%);
  content: '';
  inset: 0;
  pointer-events: none;
  position: absolute;
}

.app-bar-shell__leading,
.app-bar-shell__actions {
  position: relative;
  z-index: 1;
}

.app-bar-shell__leading {
  align-items: center;
  display: flex;
  gap: 14px;
  min-width: 0;
}

.app-bar-shell__nav-btn,
.app-bar-shell__icon {
  backdrop-filter: blur(14px);
  border: 1px solid var(--app-border-1);
}

.app-bar-shell__brand-mark {
  align-items: center;
  background:
    linear-gradient(180deg, color-mix(in srgb, var(--app-state-danger) 16%, transparent), color-mix(in srgb, var(--app-state-info) 8%, transparent)),
    color-mix(in srgb, var(--app-surface-3) 84%, transparent);
  border: 1px solid var(--app-border-1);
  border-radius: 18px;
  box-shadow: 0 16px 36px rgba(0, 0, 0, 0.18);
  display: inline-flex;
  flex-shrink: 0;
  height: 44px;
  justify-content: center;
  width: 44px;
}

.app-bar-shell__headline {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.app-bar-shell__eyebrow {
  color: var(--app-text-3);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.24em;
  text-transform: uppercase;
}

.app-bar-shell__title {
  font-size: clamp(24px, 2.2vw, 30px);
  font-weight: 600;
  line-height: 1.02;
  letter-spacing: -0.01em;
}

.app-bar-shell__meta {
  color: var(--app-text-3);
  display: flex;
  flex-wrap: wrap;
  font-size: 12px;
  gap: 10px;
  line-height: 1.4;
}

.app-bar-shell__meta span + span::before {
  color: var(--app-text-4);
  content: '•';
  margin-inline-end: 10px;
}

.app-bar-shell__actions {
  align-items: center;
  display: flex;
  flex-shrink: 0;
  gap: 8px;
}

.app-bar-shell__status,
.app-bar-shell__context-chip {
  align-items: center;
  background: color-mix(in srgb, var(--app-surface-3) 84%, transparent);
  border: 1px solid var(--app-border-1);
  border-radius: 999px;
  color: var(--app-text-2);
  display: inline-flex;
  font-size: 12px;
  font-weight: 600;
  gap: 8px;
  min-height: 36px;
  padding: 0 12px;
}

.app-bar-shell__status-dot {
  background: var(--app-state-success);
  border-radius: 999px;
  box-shadow: 0 0 0 4px color-mix(in srgb, var(--app-state-success) 16%, transparent);
  height: 8px;
  width: 8px;
}

.app-bar-shell__context {
  display: flex;
  gap: 8px;
}

@media (max-width: 1100px) {
  .app-bar-shell__context {
    display: none;
  }
}

@media (max-width: 960px) {
  .app-bar-shell {
    padding: 10px 10px 0;
  }

  .app-bar-shell__inner {
    gap: 10px;
    min-height: 74px;
    padding: 12px;
  }

  .app-bar-shell__status {
    display: none;
  }

  .app-bar-shell__title {
    font-size: 22px;
  }

  .app-bar-shell__meta {
    display: none;
  }
}

@media (max-width: 720px) {
  .app-bar-shell__brand-mark {
    border-radius: 14px;
    height: 40px;
    width: 40px;
  }

  .app-bar-shell__eyebrow {
    font-size: 10px;
    letter-spacing: 0.18em;
  }
}
</style>
