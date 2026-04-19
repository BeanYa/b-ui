<template>
  <v-app-bar class="app-bar-shell" height="96">
    <div class="app-bar-shell__inner">
      <div class="app-bar-shell__leading">
        <v-btn
          class="app-bar-shell__nav-btn"
          :icon="isMobile ? 'mdi-menu' : (collapsed ? 'mdi-menu-open' : 'mdi-menu')"
          variant="text"
          @click="$emit('toggleDrawer')"
        />
        <div class="app-bar-shell__brand-mark">
          <v-img src="@/assets/logo.svg" width="24" />
        </div>
        <div class="app-bar-shell__headline">
          <span class="app-bar-shell__eyebrow">Operations Console</span>
          <v-app-bar-title :text="pageTitle" class="app-bar-shell__title" />
          <div class="app-bar-shell__meta">
            <span>{{ hostLabel }}</span>
            <span>{{ activeThemeLabel }}</span>
          </div>
        </div>
      </div>

      <div class="app-bar-shell__actions">
        <div class="app-bar-shell__status">
          <span class="app-bar-shell__status-dot"></span>
          <span>{{ $t('main.stats.title') }}</span>
        </div>
        <v-menu>
          <template v-slot:activator="{ props }">
            <v-btn class="app-bar-shell__icon" icon v-bind="props" variant="text">
              <v-icon>mdi-translate</v-icon>
            </v-btn>
          </template>
          <v-list>
            <v-list-item
              v-for="lang in languages"
              :key="lang.value"
              @click="changeLocale(lang.value)"
              :active="isActiveLocale(lang.value)"
            >
              <v-list-item-title>{{ lang.title }}</v-list-item-title>
            </v-list-item>
          </v-list>
        </v-menu>
        <v-menu>
          <template v-slot:activator="{ props }">
            <v-btn class="app-bar-shell__icon" icon v-bind="props" variant="text">
              <v-icon>mdi-theme-light-dark</v-icon>
            </v-btn>
          </template>
          <v-list>
            <v-list-item
              v-for="th in themes"
              :key="th.value"
              @click="changeTheme(th.value)"
              :prepend-icon="th.icon"
              :active="isActiveTheme(th.value)"
            >
              <v-list-item-title>{{ $t(`theme.${th.value}`) }}</v-list-item-title>
            </v-list-item>
          </v-list>
        </v-menu>
      </div>
    </div>
  </v-app-bar>
</template>

<script lang="ts" setup>
import { computed } from 'vue'
import { useLocale, useTheme } from 'vuetify'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { languages } from '@/locales'
import { applyThemePreference, getThemePreference, type ThemePreference } from '@/plugins/theme'

defineProps(['isMobile', 'collapsed'])

const route = useRoute()
const { locale: i18nLocale, t } = useI18n()
const vuetifyLocale = useLocale()
const theme = useTheme()

const pageTitle = computed(() => t(String(route.name)))
const hostLabel = computed(() => document.location.hostname || 'localhost')
const activeThemeLabel = computed(() => `Theme · ${t(`theme.${getThemePreference()}`)}`)

const changeLocale = (l: string) => {
  i18nLocale.value = l
  vuetifyLocale.current.value = l
  localStorage.setItem('locale', l)
  window.location.reload()
}

const isActiveLocale = (l: string) => i18nLocale.value === l

const themes = [
  { value: 'light', icon: 'mdi-white-balance-sunny' },
  { value: 'dark', icon: 'mdi-moon-waning-crescent' },
  { value: 'system', icon: 'mdi-laptop' },
]

const changeTheme = (th: string) => {
  applyThemePreference(theme, th as ThemePreference)
}

const isActiveTheme = (th: string) => getThemePreference() === th
</script>

<style scoped>
.app-bar-shell {
  background: transparent !important;
  box-shadow: none !important;
  padding: 12px 18px 0;
}

.app-bar-shell__inner {
  align-items: center;
  background: linear-gradient(180deg, color-mix(in srgb, var(--app-surface-2) 94%, transparent), var(--app-surface-1));
  border: 1px solid var(--app-border-1);
  border-radius: 28px;
  box-shadow: var(--app-shadow-ring), var(--app-shadow-panel);
  display: flex;
  gap: 18px;
  justify-content: space-between;
  min-height: 76px;
  overflow: hidden;
  padding: 12px 18px;
  position: relative;
  width: 100%;
}

.app-bar-shell__inner::before {
  background:
    radial-gradient(circle at left, color-mix(in srgb, var(--app-state-info) 10%, transparent), transparent 26%),
    linear-gradient(90deg, color-mix(in srgb, #ffffff 4%, transparent), transparent 34%);
  content: '';
  inset: 0;
  pointer-events: none;
  position: absolute;
}

.app-bar-shell__leading {
  align-items: center;
  display: flex;
  gap: 14px;
  min-width: 0;
  position: relative;
  z-index: 1;
}

.app-bar-shell__nav-btn,
.app-bar-shell__icon {
  backdrop-filter: blur(14px);
  border: 1px solid var(--app-border-1);
  transition:
    background-color var(--app-motion-fast) var(--app-ease-standard),
    border-color var(--app-motion-fast) var(--app-ease-standard),
    transform var(--app-motion-fast) var(--app-ease-standard);
}

.app-bar-shell__nav-btn:hover,
.app-bar-shell__icon:hover {
  background: color-mix(in srgb, var(--app-surface-3) 88%, transparent);
  border-color: var(--app-border-2);
  transform: translateY(-1px);
}

.app-bar-shell__headline {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.app-bar-shell__eyebrow {
  color: var(--app-text-3);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.28em;
  text-transform: uppercase;
}

.app-bar-shell__title {
  font-size: clamp(22px, 2vw, 28px);
  font-weight: 600;
  letter-spacing: 0;
  line-height: 1.05;
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

.app-bar-shell__brand-mark {
  align-items: center;
  background:
    linear-gradient(180deg, color-mix(in srgb, var(--app-state-info) 18%, transparent), color-mix(in srgb, var(--app-state-info) 4%, transparent)),
    color-mix(in srgb, var(--app-surface-3) 80%, transparent);
  border: 1px solid var(--app-border-1);
  border-radius: 16px;
  box-shadow: 0 14px 32px rgba(0, 0, 0, 0.14);
  display: inline-flex;
  flex-shrink: 0;
  height: 42px;
  justify-content: center;
  width: 42px;
}

.app-bar-shell__actions {
  align-items: center;
  display: flex;
  flex-shrink: 0;
  gap: 8px;
  position: relative;
  z-index: 1;
}

.app-bar-shell__status {
  align-items: center;
  background: color-mix(in srgb, var(--app-surface-3) 80%, transparent);
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
  box-shadow: 0 0 0 4px color-mix(in srgb, var(--app-state-success) 18%, transparent);
  height: 8px;
  width: 8px;
}

@media (max-width: 960px) {
  .app-bar-shell {
    padding: 10px 10px 0;
  }

  .app-bar-shell__inner {
    gap: 10px;
    min-height: 68px;
    padding: 10px 12px;
  }

  .app-bar-shell__leading {
    gap: 10px;
  }

  .app-bar-shell__status {
    display: none;
  }

  .app-bar-shell__title {
    font-size: 20px;
  }

  .app-bar-shell__eyebrow {
    font-size: 10px;
    letter-spacing: 0.2em;
  }

  .app-bar-shell__meta {
    display: none;
  }
}

@media (max-width: 720px) {
  .app-bar-shell__inner {
    gap: 12px;
    min-height: 66px;
    padding: 10px 12px;
  }

  .app-bar-shell__leading {
    gap: 10px;
  }

  .app-bar-shell__brand-mark {
    border-radius: 12px;
    height: 38px;
    width: 38px;
  }

  .app-bar-shell__actions {
    gap: 6px;
  }
}
</style>
