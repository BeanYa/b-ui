<template>
  <v-container :class="['login-shell', themeModel.rootClass, 'fill-height']" fluid>
    <section class="login-brand-bg" aria-label="B-UI command surface">
      <div class="login-brand__content">
        <div class="login-brand__kicker">
          <span class="login-brand__signal"></span>
          B-UI Command Surface
        </div>
        <h1 class="login-brand__title">Route everything. Observe every pulse.</h1>
        <p class="login-brand__copy">
          A control surface for Sing-box operators who need traffic policy, identity access,
          runtime signal, and cluster coordination without leaving the browser.
        </p>
        <div class="login-brand__proof">
          <span>Policy plane</span>
          <span>Runtime telemetry</span>
          <span>Cluster sync</span>
        </div>
      </div>

      <div class="login-brand__visual" aria-label="Live control plane preview">
        <div class="login-console login-console--ambient">
          <div class="login-console__top">
            <span>Live control plane preview</span>
            <strong>127.0.0.1</strong>
          </div>
          <div class="login-console__matrix">
            <div
              v-for="capability in brandCapabilities"
              :key="capability.title"
              class="login-console__card"
            >
              <v-icon :icon="capability.icon" size="18" />
              <strong>{{ capability.title }}</strong>
              <span>{{ capability.copy }}</span>
            </div>
          </div>
          <div class="login-console__rail">
            <span></span>
            <span></span>
            <span></span>
          </div>
        </div>
      </div>
    </section>

    <v-row align="center" class="fill-height login-shell__row" justify="center" no-gutters>
      <v-col cols="12" class="login-shell__col">
          <section :class="['login-window', themeModel.surfaceClass]">
            <div class="login-window__module">
              <header class="login-window__header">
                <div class="login-window__brand">
                  <v-img class="login-window__logo" src="@/assets/logo.svg" width="18" />
                  <span>B-UI</span>
                </div>
                <div class="login-window__toolbar">
                  <v-select
                    v-model="$i18n.locale"
                    class="login-window__locale"
                    density="comfortable"
                    hide-details
                    :items="languages"
                    menu-icon="mdi-chevron-down"
                    variant="outlined"
                    @update:modelValue="changeLocale"
                  />
                  <v-menu>
                    <template #activator="{ props }">
                      <v-btn class="login-window__theme" icon variant="outlined" v-bind="props">
                        <v-icon>mdi-theme-light-dark</v-icon>
                      </v-btn>
                    </template>
                    <v-list>
                      <v-list-item
                        v-for="th in themes"
                        :key="th.value"
                        :active="isActiveTheme(th.value)"
                        :prepend-icon="th.icon"
                        @click="changeTheme(th.value)"
                      >
                        <v-list-item-title>{{ $t(`theme.${th.value}`) }}</v-list-item-title>
                      </v-list-item>
                    </v-list>
                  </v-menu>
                </div>
              </header>

              <div class="login-window__body">
                <div class="login-window__intro">
                  <p class="login-window__eyebrow">Access node</p>
                  <h1 class="login-window__title">{{ $t('login.title') }}</h1>
                  <p class="login-window__subtitle">{{ loginSubtitle }}</p>
                </div>

                <v-form class="login-window__form" @submit.prevent="login" ref="form">
                  <v-text-field
                    v-model="username"
                    :label="$t('login.username')"
                    :rules="usernameRules"
                    class="login-window__field"
                    prepend-inner-icon="mdi-account"
                    required
                    variant="outlined"
                  />
                  <v-text-field
                    v-model="password"
                    :label="$t('login.password')"
                    :rules="passwordRules"
                    class="login-window__field"
                    prepend-inner-icon="mdi-lock"
                    required
                    type="password"
                    variant="outlined"
                  />
                  <v-btn
                    :loading="loading"
                    block
                    class="login-window__submit"
                    type="submit"
                  >
                    {{ $t('actions.submit') }}
                  </v-btn>
                </v-form>

                <div class="login-window__status" aria-label="Access surface status">
                  <span>Encrypted session</span>
                  <strong>Standby</strong>
                </div>
              </div>
            </div>
          </section>
      </v-col>
    </v-row>
  </v-container>
</template>
  
<script lang="ts" setup>
import { computed, ref } from 'vue'
import { useLocale, useTheme } from 'vuetify'
import { i18n, languages } from '@/locales'
import { useRouter } from 'vue-router'
import HttpUtil from '@/plugins/httputil'
import useAuthStore from '@/store/modules/auth'
import { applyThemePreference, getThemePreference, type ThemePreference } from '@/plugins/theme'
import { getLoginWindowThemeModel, type LoginWindowThemeName } from '@/views/loginWindowTheme'

const theme = useTheme()
const locale = useLocale()

const themeModel = computed(() => getLoginWindowThemeModel(theme.global.name.value as LoginWindowThemeName))
const loginSubtitle = computed(() => {
  locale.current.value
  return i18n.global.t('login.subtitle')
})

const themes = [
  { value: 'light', icon: 'mdi-white-balance-sunny' },
  { value: 'dark', icon: 'mdi-moon-waning-crescent' },
  { value: 'system', icon: 'mdi-laptop' },
]

const brandCapabilities = [
  {
    icon: 'mdi-routes',
    title: 'Traffic policy',
    copy: 'Shape ingress and egress routes from one surface.',
  },
  {
    icon: 'mdi-account-network-outline',
    title: 'Identity access',
    copy: 'Keep clients, groups, and online state close.',
  },
  {
    icon: 'mdi-chart-timeline-variant-shimmer',
    title: 'Runtime signal',
    copy: 'Pair system telemetry with service health.',
  },
  {
    icon: 'mdi-vector-link',
    title: 'Cluster sync',
    copy: 'Coordinate Hub domains and mirrored members.',
  },
] as const

const username = ref('')
const usernameRules = [
  (value: string) => {
    if (value?.length > 0) return true
    return i18n.global.t('login.unRules')
  },
]

const password = ref('')
const passwordRules = [
  (value: string) => {
    if (value?.length > 0) return true
    return i18n.global.t('login.pwRules')
  },
]

const loading = ref(false)
const router = useRouter()
const auth = useAuthStore()

const login = async () => {
  if (username.value == '' || password.value == '') return
  loading.value=true
  const response = await HttpUtil.post('api/login',{user: username.value, pass: password.value})
  if(response.success){
    await auth.loadAuthState()
    setTimeout(() => {
      loading.value=false
      router.push('/')
    }, 500)
  } else {
    loading.value=false
  }
}
const changeLocale = (l: any) => {
  locale.current.value = l ?? 'en'
  localStorage.setItem('locale', locale.current.value)
}
const changeTheme = (th: string) => {
  applyThemePreference(theme, th as ThemePreference)
}
const isActiveTheme = (th: string) => {
  return getThemePreference() === th
}
</script>

<style scoped>
.login-shell {
  --login-page-bg: #fffaf0;
  --login-page-noise: radial-gradient(circle at 18% 16%, rgba(20, 33, 50, 0.04) 0 1px, transparent 1.4px),
    radial-gradient(circle at 72% 62%, rgba(26, 58, 58, 0.045) 0 1px, transparent 1.5px);
  --login-surface: rgba(255, 250, 240, 0.9);
  --login-surface-strong: #fffaf0;
  --login-border: rgba(10, 10, 10, 0.1);
  --login-border-strong: rgba(10, 10, 10, 0.18);
  --login-shadow:
    0 24px 60px rgba(80, 60, 35, 0.13),
    0 1px 0 rgba(255, 255, 255, 0.7) inset,
    0 0 0 1px rgba(255, 255, 255, 0.6) inset;
  --login-text: #0a0a0a;
  --login-text-muted: #6a6a6a;
  --login-toolbar: rgba(250, 245, 232, 0.92);
  --login-field-bg: rgba(255, 250, 240, 0.96);
  --login-field-border: rgba(10, 10, 10, 0.1);
  --login-accent: #ff4d8b;
  --login-button-bg: #0a0a0a;
  --login-button-text: #ffffff;
  align-items: center;
  background:
    radial-gradient(circle at 16% 12%, rgba(255, 176, 132, 0.3), transparent 28%),
    radial-gradient(circle at 88% 4%, rgba(184, 164, 237, 0.28), transparent 24%),
    var(--login-page-noise),
    linear-gradient(180deg, rgba(255, 255, 255, 0.75), rgba(238, 243, 248, 0.92)),
    var(--login-page-bg);
  background-size: auto, auto, 180px 180px, auto, auto;
  display: flex;
  min-height: 100vh;
  overflow: hidden;
  padding: 28px;
  position: relative;
}

.login-shell,
.login-shell * {
  box-sizing: border-box;
}

.login-shell--dark {
  --login-page-bg: #0a1a1a;
  --login-page-noise: radial-gradient(circle at 18% 16%, rgba(255, 255, 255, 0.045) 0 1px, transparent 1.4px),
    radial-gradient(circle at 72% 62%, rgba(85, 179, 255, 0.055) 0 1px, transparent 1.5px);
  --login-surface: rgba(16, 31, 30, 0.92);
  --login-surface-strong: #1a2a2a;
  --login-border: rgba(255, 250, 240, 0.1);
  --login-border-strong: rgba(255, 250, 240, 0.16);
  --login-shadow:
    0 28px 70px rgba(0, 0, 0, 0.45),
    0 1px 0 rgba(255, 255, 255, 0.05) inset,
    0 0 0 1px rgba(7, 8, 10, 0.9) inset;
  --login-text: #fffaf0;
  --login-text-muted: #a9a093;
  --login-toolbar: rgba(26, 42, 42, 0.9);
  --login-field-bg: rgba(10, 26, 26, 0.92);
  --login-field-border: rgba(255, 250, 240, 0.1);
  --login-accent: #b8a4ed;
  --login-button-bg: #fffaf0;
  --login-button-text: #0a1a1a;
  background:
    radial-gradient(circle at top left, rgba(255, 77, 139, 0.1), transparent 26%),
    radial-gradient(circle at bottom right, rgba(184, 164, 237, 0.16), transparent 28%),
    radial-gradient(circle at 48% 0, rgba(164, 212, 197, 0.08), transparent 34%),
    var(--login-page-noise),
    var(--login-page-bg);
  background-size: auto, auto, auto, 180px 180px, auto;
}

.login-shell::before,
.login-shell::after {
  content: '';
  inset: 0;
  pointer-events: none;
  position: absolute;
}

.login-shell::before {
  background:
    conic-gradient(from 218deg at 12% 14%, transparent 0 16%, color-mix(in srgb, var(--login-accent) 16%, transparent) 28%, transparent 44% 100%),
    conic-gradient(from 42deg at 88% 2%, transparent 0 12%, rgba(255, 99, 99, 0.12) 20%, transparent 34% 100%),
    linear-gradient(115deg, transparent 8%, rgba(255, 255, 255, 0.05) 24%, transparent 42%);
  filter: blur(18px);
  opacity: 0.5;
  transform: translate3d(0, 0, 0);
  animation: login-aurora-sweep 20s cubic-bezier(0.22, 1, 0.36, 1) infinite alternate;
}

.login-shell::after {
  background:
    linear-gradient(180deg, transparent 0%, color-mix(in srgb, var(--login-accent) 8%, transparent) 52%, transparent 100%),
    var(--login-page-noise);
  background-size: 100% 42%, 180px 180px;
  mix-blend-mode: screen;
  opacity: 0.2;
  animation: login-scan-pass 9s cubic-bezier(0.22, 1, 0.36, 1) infinite;
}

.login-shell__row {
  margin: 0;
  min-height: calc(100vh - 56px);
  position: relative;
  width: 100%;
  z-index: 2;
}

.login-shell__col {
  align-items: center;
  display: flex;
  gap: 18px;
  justify-content: flex-end;
  max-width: min(1220px, calc(100vw - 72px));
  max-inline-size: 100%;
  min-width: 0;
  padding: 0 !important;
  width: 100%;
}

.login-brand-bg {
  align-content: center;
  color: var(--login-text);
  display: grid;
  gap: clamp(22px, 2.4vw, 34px);
  grid-template-columns: minmax(320px, 560px) minmax(260px, 420px);
  inset: 0;
  padding: clamp(44px, 7vw, 108px) clamp(460px, 34vw, 620px) clamp(42px, 6vw, 86px) clamp(42px, 7vw, 112px);
  pointer-events: none;
  position: absolute;
  z-index: 1;
}

.login-brand-bg::before {
  background:
    radial-gradient(ellipse at 20% 22%, color-mix(in srgb, var(--login-surface) 62%, transparent), transparent 44%),
    radial-gradient(ellipse at 62% 52%, color-mix(in srgb, var(--login-accent) 10%, transparent), transparent 38%),
    linear-gradient(90deg, color-mix(in srgb, var(--login-surface) 38%, transparent), transparent 72%);
  content: '';
  inset: 0;
  opacity: 0.9;
  pointer-events: none;
  position: absolute;
}

.login-brand-bg::after {
  background:
    linear-gradient(90deg, transparent, color-mix(in srgb, var(--login-accent) 24%, transparent), transparent);
  content: '';
  height: 1px;
  inset: auto clamp(460px, 34vw, 620px) 16vh clamp(42px, 7vw, 112px);
  pointer-events: none;
  position: absolute;
  width: auto;
}

.login-window {
  color: var(--login-text);
  max-width: 100%;
  min-width: 0;
  position: relative;
  width: 100%;
  z-index: 1;
}

.login-brand__content {
  align-content: center;
  display: grid;
  min-width: 0;
  position: relative;
  z-index: 1;
}

.login-window {
  align-items: center;
  display: flex;
  justify-content: flex-end;
  padding: 0;
}

.login-brand__kicker {
  align-items: center;
  color: var(--login-text-muted);
  display: inline-flex;
  font-size: 11px;
  font-weight: 800;
  gap: 10px;
  letter-spacing: 0.18em;
  text-transform: uppercase;
}

.login-brand__signal {
  animation: login-brand-pulse 1.8s ease-in-out infinite;
  background: #22c55e;
  border-radius: 999px;
  box-shadow: 0 0 0 6px rgba(95, 201, 146, 0.14);
  height: 8px;
  width: 8px;
}

.login-brand__title {
  color: var(--login-text);
  font-size: clamp(44px, 4.3vw, 68px);
  font-weight: 800;
  letter-spacing: -0.055em;
  line-height: 0.94;
  margin: 20px 0 0;
  max-width: 9.8ch;
}

.login-brand__copy {
  color: var(--login-text-muted);
  font-size: clamp(15px, 1.2vw, 17px);
  line-height: 1.72;
  margin: 22px 0 0;
  max-width: 48ch;
}

.login-brand__proof {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 24px;
}

.login-brand__proof span {
  background: color-mix(in srgb, var(--login-field-bg) 82%, transparent);
  border: 1px solid var(--login-field-border);
  border-radius: 999px;
  color: var(--login-text);
  font-size: 12px;
  font-weight: 700;
  padding: 8px 11px;
}

.login-brand__visual {
  align-self: end;
  max-width: 420px;
  min-width: 0;
  opacity: 0.82;
  position: relative;
  transform: translate3d(0, 8px, 0);
  width: 100%;
  z-index: 1;
}

.login-console {
  background: transparent;
  border: 0;
  border-radius: 0;
  backdrop-filter: none;
  display: grid;
  gap: 14px;
  max-width: 100%;
  min-width: 0;
  padding: 0;
  position: relative;
  width: 100%;
}

.login-console--ambient::before {
  background:
    radial-gradient(circle at 16% 16%, color-mix(in srgb, var(--login-accent) 16%, transparent), transparent 28%),
    radial-gradient(circle at 92% 78%, color-mix(in srgb, var(--login-accent) 11%, transparent), transparent 32%);
  content: '';
  filter: blur(18px);
  inset: -18px;
  opacity: 0.42;
  pointer-events: none;
  position: absolute;
}

.login-console__top {
  align-items: center;
  display: flex;
  gap: 12px;
  justify-content: space-between;
  position: relative;
  z-index: 1;
}

.login-console__top span {
  color: var(--login-text-muted);
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.16em;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  text-transform: uppercase;
  white-space: nowrap;
}

.login-console__top strong {
  color: var(--login-text);
  flex: none;
  font-family: var(--app-font-mono);
  font-size: 13px;
}

.login-console__matrix {
  display: grid;
  gap: 10px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  position: relative;
  z-index: 1;
}

.login-console__card {
  background: transparent;
  border: 0;
  border-radius: 0;
  min-height: 104px;
  padding: 10px 8px;
  position: relative;
}

.login-console__card::before {
  background: color-mix(in srgb, var(--login-accent) 54%, transparent);
  border-radius: 999px;
  content: '';
  height: 6px;
  left: 8px;
  position: absolute;
  top: 7px;
  width: 6px;
}

.login-console__card .v-icon {
  color: var(--login-accent);
}

.login-console__card strong {
  color: var(--login-text);
  display: block;
  font-size: 13px;
  line-height: 1.25;
  margin-top: 18px;
}

.login-console__card span {
  color: var(--login-text-muted);
  display: block;
  font-size: 12px;
  line-height: 1.45;
  margin-top: 6px;
}

.login-console__rail {
  display: grid;
  gap: 8px;
  position: relative;
  z-index: 1;
}

.login-console__rail span {
  background: color-mix(in srgb, var(--login-accent) 18%, var(--login-field-bg));
  border-radius: 999px;
  display: block;
  height: 8px;
  overflow: hidden;
  position: relative;
}

.login-console__rail span::after {
  animation: login-rail-flow 3.6s ease-in-out infinite;
  background: linear-gradient(90deg, transparent, color-mix(in srgb, var(--login-accent) 56%, #ffffff), transparent);
  content: '';
  inset: 0;
  position: absolute;
  transform: translateX(-70%);
}

.login-console__rail span:nth-child(2)::after {
  animation-delay: 400ms;
}

.login-console__rail span:nth-child(3)::after {
  animation-delay: 800ms;
}

.login-window__header,
.login-window__body {
  position: relative;
  z-index: 1;
}

.login-window__module {
  background:
    radial-gradient(circle at 12% 0, color-mix(in srgb, var(--login-accent) 11%, transparent), transparent 34%),
    linear-gradient(180deg, color-mix(in srgb, var(--login-surface-strong) 82%, transparent), color-mix(in srgb, var(--login-field-bg) 76%, transparent)),
    color-mix(in srgb, var(--login-field-bg) 82%, transparent);
  border: 1px solid var(--login-field-border);
  border-radius: 24px;
  box-shadow: var(--login-shadow);
  backdrop-filter: blur(18px);
  display: grid;
  max-width: 470px;
  min-width: 0;
  overflow: hidden;
  position: relative;
  width: 100%;
}

.login-window__module::before {
  background:
    radial-gradient(circle at 18% 0, color-mix(in srgb, var(--login-accent) 15%, transparent), transparent 30%),
    linear-gradient(120deg, transparent 0 30%, rgba(255, 255, 255, 0.06) 48%, transparent 66% 100%);
  content: '';
  inset: 0;
  opacity: 0.48;
  pointer-events: none;
  position: absolute;
}

.login-window__module::after {
  background: color-mix(in srgb, var(--login-accent) 72%, #ffffff);
  border-radius: 999px;
  box-shadow:
    0 0 18px color-mix(in srgb, var(--login-accent) 54%, transparent),
    0 0 0 6px color-mix(in srgb, var(--login-accent) 10%, transparent);
  content: '';
  height: 6px;
  inset: 22px 22px auto auto;
  opacity: 0.72;
  pointer-events: none;
  position: absolute;
  width: 6px;
}

.login-window--light {
  background-image: linear-gradient(180deg, rgba(255, 255, 255, 0.5), rgba(255, 255, 255, 0));
}

.login-window--dark {
  background-image: linear-gradient(180deg, rgba(255, 255, 255, 0.035), rgba(255, 255, 255, 0));
}

.login-window__header {
  align-items: center;
  border-bottom: 1px solid var(--login-field-border);
  display: flex;
  gap: 16px;
  justify-content: space-between;
  padding: 16px 18px;
}

.login-window__brand {
  align-items: center;
  color: var(--login-text);
  display: inline-flex;
  gap: 10px;
  font-size: 13px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.login-window__logo {
  flex: none;
}

.login-window__toolbar {
  align-items: center;
  display: inline-grid;
  gap: 10px;
  grid-template-columns: minmax(0, 156px) auto;
}

.login-window__body {
  display: grid;
  gap: 24px;
  padding: 30px 28px 24px;
}

.login-window__intro {
  display: grid;
  gap: 8px;
}

.login-window__eyebrow {
  color: var(--login-accent);
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.12em;
  margin: 0;
  text-transform: uppercase;
}

.login-window__title {
  color: var(--login-text);
  font-size: clamp(30px, 4vw, 40px);
  font-weight: 700;
  letter-spacing: -0.03em;
  line-height: 1.02;
  margin: 0;
}

.login-window__subtitle {
  color: var(--login-text-muted);
  font-size: 15px;
  line-height: 1.6;
  margin: 0;
  max-width: 34ch;
}

.login-window__form {
  display: grid;
  gap: 14px;
}

.login-window__submit {
  background: var(--login-button-bg);
  color: var(--login-button-text);
  margin-top: 6px;
  min-height: 48px;
}

.login-window__status {
  align-items: center;
  border-top: 1px solid var(--login-field-border);
  color: var(--login-text-muted);
  display: flex;
  font-family: var(--app-font-mono);
  font-size: 11px;
  gap: 12px;
  justify-content: space-between;
  letter-spacing: 0.08em;
  padding-top: 16px;
  text-transform: uppercase;
}

.login-window__status strong {
  color: #22c55e;
  font-size: 11px;
}

:deep(.login-window__field .v-field),
:deep(.login-window__locale .v-field),
.login-window__theme {
  background: var(--login-field-bg);
  border: 1px solid var(--login-field-border);
  box-shadow: 0 1px 0 rgba(255, 255, 255, 0.04) inset;
}

:deep(.login-window__field .v-field),
:deep(.login-window__locale .v-field) {
  border-radius: 14px;
}

:deep(.login-window__field .v-field__input),
:deep(.login-window__field .v-label),
:deep(.login-window__locale .v-field__input),
:deep(.login-window__locale .v-select__selection-text) {
  color: var(--login-text);
}

:deep(.login-window__field .v-field__prepend-inner),
:deep(.login-window__locale .v-field__append-inner),
:deep(.login-window__locale .v-field__clearable) {
  color: var(--login-text-muted);
}

:deep(.login-window__field .v-field--focused),
:deep(.login-window__locale .v-field--focused),
.login-window__theme:hover,
.login-window__theme:focus-visible {
  border-color: color-mix(in srgb, var(--login-accent) 48%, var(--login-field-border));
  box-shadow:
    0 0 0 3px color-mix(in srgb, var(--login-accent) 18%, transparent),
    0 1px 0 rgba(255, 255, 255, 0.04) inset;
}

.login-window__theme {
  border-radius: 14px;
  min-height: 44px;
  min-width: 44px;
}

@media (prefers-reduced-motion: reduce) {
  .login-shell::before,
  .login-shell::after,
  .login-brand__signal,
  .login-window__module::after,
  .login-console__rail span::after {
    animation: none !important;
  }
}

@keyframes login-brand-pulse {
  0%,
  100% {
    opacity: 0.45;
    transform: scale(0.78);
  }

  50% {
    opacity: 1;
    transform: scale(1);
  }
}

@keyframes login-rail-flow {
  0% {
    transform: translateX(-70%);
  }

  56%,
  100% {
    transform: translateX(70%);
  }
}

@keyframes login-aurora-sweep {
  from {
    opacity: 0.38;
    transform: translate3d(-2%, -1%, 0) rotate(-1deg);
  }

  to {
    opacity: 0.58;
    transform: translate3d(2%, 1%, 0) rotate(1deg);
  }
}

@keyframes login-scan-pass {
  0% {
    background-position: 0 -42vh, 0 0;
  }

  58%,
  100% {
    background-position: 0 110vh, 18px 12px;
  }
}

@media (min-width: 1600px) {
  .login-shell__col {
    max-width: 1220px;
  }

  .login-brand__title {
    font-size: 68px;
  }

  .login-window__module {
    max-width: 460px;
  }
}

@media (max-width: 960px) {
  .login-shell {
    padding: 16px;
  }

  .login-shell__col {
    justify-content: center;
    max-width: 720px;
  }

  .login-brand-bg {
    align-content: start;
    grid-template-columns: minmax(0, 1fr);
    opacity: 0.62;
    padding: 28px 24px 0;
  }

  .login-brand__title {
    max-width: 12ch;
  }

  .login-brand__visual {
    display: none;
  }

  .login-window__body {
    padding: 24px 22px 22px;
  }

  .login-window {
    padding: 0;
  }

  .login-window__module {
    margin-top: 170px;
    max-width: min(100%, 520px);
  }
}

@media (max-width: 600px) {
  .login-shell {
    max-width: 100vw;
    padding: 8px !important;
    width: 100vw;
  }

  .login-shell__row {
    max-width: 100%;
  }

  .login-shell__col {
    flex: 0 1 calc(100vw - 16px) !important;
    max-width: calc(100vw - 16px);
    width: calc(100vw - 16px);
  }

  .login-brand-bg {
    opacity: 0.5;
    padding: 20px 18px 0;
  }

  .login-brand__kicker {
    font-size: 10px;
    letter-spacing: 0.12em;
  }

  .login-brand__title {
    font-size: clamp(32px, 8.8vw, 38px);
    letter-spacing: 0;
    line-height: 1;
    max-width: 12ch;
  }

  .login-brand__copy {
    font-size: 14px;
    max-width: 34ch;
  }

  .login-brand__proof {
    gap: 8px;
  }

  .login-brand__proof span {
    padding: 7px 9px;
  }

  .login-console {
    border-radius: 18px;
    padding: 14px;
  }

  .login-console__top {
    gap: 8px;
  }

  .login-console__top span {
    letter-spacing: 0.1em;
  }

  .login-console__top strong {
    display: none;
  }

  .login-console__matrix {
    grid-template-columns: minmax(0, 1fr);
  }

  .login-window__header {
    align-items: stretch;
    flex-direction: column;
    padding: 16px;
  }

  .login-window__toolbar {
    grid-template-columns: minmax(0, 1fr) auto;
  }

  .login-window__body {
    gap: 20px;
    padding: 18px;
  }

  .login-window {
    padding: 0;
  }

  .login-window__module {
    border-radius: 18px;
    margin-top: 148px;
  }

  .login-window__status {
    align-items: flex-start;
    flex-direction: column;
    gap: 6px;
  }
}
</style>
