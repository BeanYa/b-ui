<template>
  <v-container :class="['login-shell', themeModel.rootClass, 'fill-height']" fluid>
    <v-row align="center" class="fill-height login-shell__row" justify="center">
      <v-col cols="12" sm="10" md="8" lg="6" xl="5">
        <section :class="['login-window', themeModel.surfaceClass]">
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
              <p class="login-window__eyebrow">B-UI</p>
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
  --login-page-bg: #eef3f8;
  --login-page-grid: rgba(110, 138, 170, 0.08);
  --login-surface: rgba(255, 255, 255, 0.88);
  --login-surface-strong: #ffffff;
  --login-border: rgba(145, 164, 187, 0.28);
  --login-border-strong: rgba(112, 134, 162, 0.4);
  --login-shadow:
    0 24px 60px rgba(44, 65, 91, 0.12),
    0 1px 0 rgba(255, 255, 255, 0.7) inset,
    0 0 0 1px rgba(255, 255, 255, 0.6) inset;
  --login-text: #142132;
  --login-text-muted: #5f7086;
  --login-toolbar: rgba(246, 249, 252, 0.92);
  --login-field-bg: rgba(247, 250, 252, 0.96);
  --login-field-border: rgba(145, 164, 187, 0.28);
  --login-accent: #55b3ff;
  --login-button-bg: #142132;
  --login-button-text: #f8fbff;
  align-items: center;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.75), rgba(238, 243, 248, 0.92)),
    linear-gradient(var(--login-page-grid) 1px, transparent 1px),
    linear-gradient(90deg, var(--login-page-grid) 1px, transparent 1px),
    var(--login-page-bg);
  background-size: auto, 28px 28px, 28px 28px, auto;
  display: flex;
  min-height: 100vh;
  padding: 28px;
  position: relative;
}

.login-shell--dark {
  --login-page-bg: #07080a;
  --login-page-grid: rgba(255, 255, 255, 0.04);
  --login-surface: rgba(16, 17, 17, 0.92);
  --login-surface-strong: #101111;
  --login-border: rgba(255, 255, 255, 0.08);
  --login-border-strong: rgba(255, 255, 255, 0.14);
  --login-shadow:
    0 28px 70px rgba(0, 0, 0, 0.45),
    0 1px 0 rgba(255, 255, 255, 0.05) inset,
    0 0 0 1px rgba(7, 8, 10, 0.9) inset;
  --login-text: #f9f9f9;
  --login-text-muted: #9c9c9d;
  --login-toolbar: rgba(18, 18, 18, 0.9);
  --login-field-bg: rgba(7, 8, 10, 0.92);
  --login-field-border: rgba(255, 255, 255, 0.1);
  --login-accent: #55b3ff;
  --login-button-bg: rgba(255, 255, 255, 0.92);
  --login-button-text: #18191a;
  background:
    radial-gradient(circle at top left, rgba(255, 99, 99, 0.08), transparent 24%),
    radial-gradient(circle at bottom right, rgba(85, 179, 255, 0.08), transparent 24%),
    linear-gradient(var(--login-page-grid) 1px, transparent 1px),
    linear-gradient(90deg, var(--login-page-grid) 1px, transparent 1px),
    var(--login-page-bg);
  background-size: auto, auto, 28px 28px, 28px 28px, auto;
}

.login-shell__row {
  min-height: calc(100vh - 56px);
}

.login-window {
  background: var(--login-surface);
  border: 1px solid var(--login-border);
  border-radius: 24px;
  box-shadow: var(--login-shadow);
  color: var(--login-text);
  overflow: hidden;
  position: relative;
  backdrop-filter: blur(18px);
}

.login-window--light {
  background-image: linear-gradient(180deg, rgba(255, 255, 255, 0.62), rgba(255, 255, 255, 0));
}

.login-window--dark {
  background-image: linear-gradient(180deg, rgba(255, 255, 255, 0.03), rgba(255, 255, 255, 0));
}

.login-window__header {
  align-items: center;
  background: var(--login-toolbar);
  border-bottom: 1px solid var(--login-border);
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
  padding: 28px;
}

.login-window__intro {
  display: grid;
  gap: 8px;
}

.login-window__eyebrow {
  color: var(--login-text-muted);
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

@media (max-width: 960px) {
  .login-shell {
    padding: 16px;
  }

  .login-window__body {
    padding: 22px;
  }
}

@media (max-width: 600px) {
  .login-window {
    border-radius: 20px;
  }

  .login-window__header {
    align-items: stretch;
    flex-direction: column;
  }

  .login-window__toolbar {
    grid-template-columns: minmax(0, 1fr) auto;
  }

  .login-window__body {
    gap: 20px;
    padding: 18px;
  }
}
</style>
