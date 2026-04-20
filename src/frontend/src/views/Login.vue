<template>
  <v-container class="login-shell fill-height" fluid>
    <v-row align="center" class="fill-height login-shell__row" justify="center">
      <v-col cols="12" lg="10" xl="9">
        <div class="login-panel">
          <div class="login-panel__glow"></div>
          <v-row class="ma-0">
            <v-col class="login-panel__aside" cols="12" md="6">
              <div class="login-panel__badge">
                <v-img src="@/assets/logo.svg" width="18" />
                <span>B-UI</span>
              </div>
              <h1 class="login-panel__title">Control Surface</h1>
              <p class="login-panel__copy">
                Built for B-UI and reshaped into a darker, tighter desktop-style admin shell.
              </p>
              <div class="login-panel__tags">
                <span>{{ $t('pages.inbounds') }}</span>
                <span>{{ $t('pages.clients') }}</span>
                <span>{{ $t('pages.rules') }}</span>
                <span>{{ $t('main.stats.title') }}</span>
              </div>
            </v-col>
            <v-col cols="12" md="6">
              <v-card class="login-card">
                <v-card-title class="login-card__title" v-text="$t('login.title')" />
                <v-card-subtitle class="login-card__subtitle">
                  {{ $t('pages.home') }} · {{ $t('pages.settings') }}
                </v-card-subtitle>
                <v-card-text class="login-card__body">
                  <v-form @submit.prevent="login" ref="form">
                    <v-text-field
                      v-model="username"
                      :label="$t('login.username')"
                      :rules="usernameRules"
                      prepend-inner-icon="mdi-account"
                      required
                    />
                    <v-text-field
                      v-model="password"
                      :label="$t('login.password')"
                      :rules="passwordRules"
                      prepend-inner-icon="mdi-lock"
                      type="password"
                      required
                    />
                    <v-btn
                      :loading="loading"
                      block
                      class="login-card__submit"
                      type="submit"
                    >
                      {{ $t('actions.submit') }}
                    </v-btn>
                  </v-form>
                  <div class="login-card__controls">
                    <v-select
                      density="comfortable"
                      hide-details
                      :items="languages"
                      menu-icon="mdi-chevron-down"
                      v-model="$i18n.locale"
                      @update:modelValue="changeLocale"
                    />
                    <v-menu>
                      <template v-slot:activator="{ props }">
                        <v-btn class="login-card__theme" icon v-bind="props" variant="tonal">
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
                </v-card-text>
              </v-card>
            </v-col>
          </v-row>
        </div>
      </v-col>
    </v-row>
  </v-container>
</template>
  
<script lang="ts" setup>
import { ref } from "vue"
import { useLocale, useTheme } from 'vuetify'
import { i18n, languages } from '@/locales'
import { useRouter } from 'vue-router'
import HttpUtil from '@/plugins/httputil'
import { applyThemePreference, getThemePreference, type ThemePreference } from '@/plugins/theme'

const theme = useTheme()
const locale = useLocale()

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

const login = async () => {
  if (username.value == '' || password.value == '') return
  loading.value=true
  const response = await HttpUtil.post('api/login',{user: username.value, pass: password.value})
  if(response.success){
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
  align-items: center;
  display: flex;
  min-height: 100vh;
  padding: 28px;
  position: relative;
}

.login-shell__row {
  min-height: calc(100vh - 56px);
}

.login-panel {
  background:
    linear-gradient(135deg, rgba(255, 99, 99, 0.08), transparent 32%),
    rgba(16, 17, 17, 0.68);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 32px;
  overflow: hidden;
  position: relative;
}

.login-panel__glow {
  background:
    radial-gradient(circle at top left, rgba(255, 99, 99, 0.24), transparent 28%),
    radial-gradient(circle at bottom right, rgba(85, 179, 255, 0.18), transparent 28%);
  inset: 0;
  pointer-events: none;
  position: absolute;
}

.login-panel__aside {
  display: flex;
  flex-direction: column;
  gap: 18px;
  justify-content: center;
  padding: 48px;
  position: relative;
  z-index: 1;
}

.login-panel__badge {
  align-items: center;
  background: rgba(255, 99, 99, 0.12);
  border: 1px solid rgba(255, 99, 99, 0.18);
  border-radius: 999px;
  color: #ffb5b5;
  display: inline-flex;
  gap: 10px;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.18em;
  padding: 8px 14px;
  text-transform: uppercase;
  width: fit-content;
}

.login-panel__title {
  font-size: clamp(42px, 5vw, 68px);
  font-weight: 700;
  letter-spacing: -0.03em;
  line-height: 0.98;
  margin: 0;
  max-width: 6ch;
}

.login-panel__copy {
  color: #cecece;
  font-size: 16px;
  line-height: 1.7;
  margin: 0;
  max-width: 44ch;
}

.login-panel__tags {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.login-panel__tags span {
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 999px;
  color: #f9f9f9;
  font-size: 13px;
  font-weight: 500;
  padding: 10px 14px;
}

.login-card {
  background: rgba(11, 13, 16, 0.88) !important;
  border-radius: 28px !important;
  margin: 22px;
  min-height: calc(100% - 44px);
  padding: 10px;
  position: relative;
  z-index: 1;
}

.login-card__title {
  font-size: 28px;
  font-weight: 600;
  padding-bottom: 4px;
  text-align: left;
}

.login-card__subtitle {
  border: 0;
  color: #9c9c9d;
  min-height: auto;
  padding-top: 0;
  text-align: left;
}

.login-card__body {
  display: flex;
  flex-direction: column;
  gap: 18px;
  padding-top: 18px;
}

.login-card__submit {
  background: rgba(255, 255, 255, 0.88);
  color: #18191a;
  margin-top: 12px;
  min-height: 48px;
}

.login-card__controls {
  align-items: center;
  display: grid;
  gap: 12px;
  grid-template-columns: minmax(0, 1fr) auto;
}

.login-card__theme {
  min-height: 56px;
}

@media (max-width: 960px) {
  .login-shell {
    padding: 16px;
  }

  .login-panel__aside {
    padding: 28px 28px 10px;
  }

  .login-card {
    margin: 10px;
  }
}
</style>
