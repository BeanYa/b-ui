<template>
  <v-app-bar class="app-bar-shell app-bar-shell--docked" height="84">
    <div class="app-bar-shell__inner">
      <div class="app-bar-shell__leading">
        <v-btn
          class="app-bar-shell__nav-btn"
          :icon="isMobile ? 'mdi-menu' : (collapsed ? 'mdi-menu' : 'mdi-menu-open')"
          variant="text"
          @click="$emit('toggleDrawer')"
        />
        <div class="app-bar-shell__brand-mark">
          <v-img src="@/assets/logo.svg" width="26" />
        </div>
        <div class="app-bar-shell__headline">
          <span class="app-bar-shell__eyebrow">{{ pageSection }}</span>
          <v-app-bar-title :text="pageTitle" class="app-bar-shell__title" />
        </div>
      </div>

      <div class="app-bar-shell__actions">
        <span class="app-bar-shell__check">Last check: {{ lastCheckLabel }}</span>
        <div class="app-bar-shell__status">
          <span class="app-bar-shell__status-dot"></span>
          <span>{{ hostLabel }}</span>
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
            <v-btn class="app-bar-shell__theme-toggle" v-bind="props" variant="text">
              <v-icon>{{ activeThemeIcon }}</v-icon>
              <span>{{ activeThemeLabel }}</span>
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
const localeLabel = computed(() => i18nLocale.value.toUpperCase())
const runtimeLabel = computed(() => t(`theme.${getThemePreference()}`))
const lastCheckLabel = computed(() => new Intl.DateTimeFormat(i18nLocale.value, {
  hour: '2-digit',
  minute: '2-digit',
}).format(new Date()))
const activeThemeIcon = computed(() => theme.global.name.value === 'dark' ? 'mdi-weather-night' : 'mdi-white-balance-sunny')
const activeThemeLabel = computed(() => t(`theme.${getThemePreference()}`))
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
  border-radius: 28px;
  box-shadow: none !important;
  isolation: isolate;
  overflow: hidden !important;
  padding: 12px 18px 0;
}

.app-bar-shell--docked {
  filter: drop-shadow(0 18px 38px color-mix(in srgb, var(--app-primary) 10%, transparent));
}

.app-bar-shell :deep(.v-toolbar__content),
.app-bar-shell :deep(.v-toolbar__extension) {
  background: transparent !important;
  border-radius: inherit;
  overflow: hidden;
  padding: 0 !important;
}

.app-bar-shell :deep(.v-toolbar__overlay) {
  display: none;
}

.app-bar-shell__inner {
  align-items: center;
  background:
    radial-gradient(circle at 68% -30%, color-mix(in srgb, var(--app-brand-ochre) 20%, transparent), transparent 26%),
    linear-gradient(180deg, color-mix(in srgb, var(--app-panel-bg) 94%, transparent), color-mix(in srgb, var(--app-surface-1) 92%, transparent));
  border: 1px solid var(--app-border-1);
  border-radius: 26px;
  border-bottom-left-radius: 18px;
  border-bottom-right-radius: 18px;
  box-shadow: var(--app-shadow-device);
  display: flex;
  gap: 16px;
  justify-content: space-between;
  min-height: 66px;
  overflow: hidden;
  padding: 10px 14px;
  position: relative;
  width: 100%;
}

.app-bar-shell__inner::before {
  background:
    radial-gradient(circle at 100% 0%, color-mix(in srgb, var(--app-state-warning) 10%, transparent), transparent 24%),
    radial-gradient(circle at 42% 140%, color-mix(in srgb, var(--app-brand-lavender) 12%, transparent), transparent 30%),
    linear-gradient(120deg, color-mix(in srgb, #ffffff 8%, transparent), transparent 34%);
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
.app-bar-shell__icon,
.app-bar-shell__theme-toggle {
  backdrop-filter: blur(14px);
  border: 1px solid var(--app-border-1);
  color: var(--app-text-2) !important;
}

.app-bar-shell__nav-btn:hover,
.app-bar-shell__icon:hover,
.app-bar-shell__theme-toggle:hover {
  color: var(--app-text-1) !important;
}

.app-bar-shell__nav-btn :deep(.v-icon),
.app-bar-shell__icon :deep(.v-icon),
.app-bar-shell__theme-toggle :deep(.v-icon) {
  color: currentColor !important;
}

.app-bar-shell__theme-toggle {
  background: color-mix(in srgb, var(--app-brand-ochre) 22%, var(--app-control-chip));
  border-radius: var(--app-radius-pill);
  gap: 8px;
  min-height: 38px;
  padding-inline: 12px;
}

.app-bar-shell__theme-toggle span {
  font-size: 12px;
  font-weight: 700;
}

.app-bar-shell__brand-mark {
  align-items: center;
  background: var(--app-control-chip);
  border: 1px solid var(--app-border-1);
  border-radius: 15px;
  box-shadow: var(--app-shadow-soft);
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
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.app-bar-shell__title {
  font-size: clamp(20px, 1.8vw, 26px);
  font-weight: 560;
  line-height: 1.02;
  letter-spacing: 0;
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
.app-bar-shell__check,
.app-bar-shell__context-chip {
  align-items: center;
  background: var(--app-control-chip);
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

.app-bar-shell__check {
  background: transparent;
  border-color: transparent;
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
    min-height: 64px;
    padding: 12px;
  }

  .app-bar-shell__check,
  .app-bar-shell__status {
    display: none;
  }

  .app-bar-shell__title {
    font-size: 20px;
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
