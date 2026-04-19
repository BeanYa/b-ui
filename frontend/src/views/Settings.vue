<template>
  <div class="app-page">
    <section class="app-page__hero">
      <div class="app-page__hero-head">
        <div class="app-page__hero-kicker">{{ $t('pages.settings') }}</div>
        <h1 class="app-page__hero-title">{{ $t('pages.settings') }}</h1>
        <p class="app-page__hero-copy">
          Tune interface exposure, subscription endpoints, and exported formats from one control surface with explicit save and restart boundaries.
        </p>
        <div class="app-page__hero-meta">
          <span class="app-page__hero-meta-item">{{ settings.webListen || '0.0.0.0' }}</span>
          <span class="app-page__hero-meta-item">Port {{ webPort }}</span>
          <span class="app-page__hero-meta-item">{{ stateChange ? 'Unsaved changes' : 'Configuration synced' }}</span>
        </div>
      </div>
      <div class="app-page__hero-side">
        <div class="app-page__hero-stats">
          <div class="app-page__hero-stat">
            <span class="app-page__hero-stat-label">{{ $t('setting.sessionAge') }}</span>
            <strong class="app-page__hero-stat-value">{{ sessionMaxAge }}</strong>
            <span class="app-page__hero-stat-note">Minutes before interface sessions expire</span>
          </div>
          <div class="app-page__hero-stat">
            <span class="app-page__hero-stat-label">{{ $t('setting.update') }}</span>
            <strong class="app-page__hero-stat-value">{{ subUpdates }}</strong>
            <span class="app-page__hero-stat-note">Hours between subscription refresh windows</span>
          </div>
        </div>
      </div>
    </section>
    <v-card class="app-card-shell" :loading="loading">
      <v-tabs
      v-model="tab"
      color="primary"
      align-tabs="center"
      show-arrows
    >
      <v-tab value="t1">{{ $t('setting.interface') }}</v-tab>
      <v-tab value="t2">{{ $t('setting.sub') }}</v-tab>
      <v-tab value="t3">{{ $t('setting.jsonSub') }}</v-tab>
      <v-tab value="t4">{{ $t('setting.clashSub') }}</v-tab>
    </v-tabs>
    <v-card-text>
      <v-row class="app-page__toolbar">
        <v-col cols="12">
          <div class="app-page__toolbar-actions app-toolbar-cluster">
            <v-btn color="primary" @click="save" :loading="loading" :disabled="!stateChange">
              {{ $t('actions.save') }}
            </v-btn>
            <v-btn variant="outlined" color="warning" @click="restartApp" :loading="loading" :disabled="stateChange">
              {{ $t('actions.restartApp') }}
            </v-btn>
          </div>
        </v-col>
      </v-row>
      <v-window v-model="tab">
      <v-window-item value="t1">
        <div class="settings-stack">
          <section class="settings-section app-panel">
            <div class="settings-section__head">
              <div class="settings-section__label">Interface binding</div>
              <div class="settings-section__caption">Address, public path, and TLS assets for the web console.</div>
            </div>
            <v-row>
              <v-col cols="12" sm="6" md="4">
                <v-text-field v-model="settings.webListen" :label="$t('setting.addr')" hide-details></v-text-field>
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-text-field v-model.number="webPort" min="1" type="number" :label="$t('setting.port')" hide-details></v-text-field>
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-text-field v-model="settings.webPath" :label="$t('setting.webPath')" hide-details></v-text-field>
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-text-field v-model="settings.webDomain" :label="$t('setting.domain')" hide-details></v-text-field>
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-text-field v-model="settings.webKeyFile" :label="$t('setting.sslKey')" hide-details></v-text-field>
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-text-field v-model="settings.webCertFile" :label="$t('setting.sslCert')" hide-details></v-text-field>
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-text-field v-model="settings.webURI" :label="$t('setting.webUri')" hide-details></v-text-field>
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-text-field v-model="settings.timeLocation" :label="$t('setting.timeLoc')" hide-details></v-text-field>
              </v-col>
            </v-row>
          </section>

          <section class="settings-section app-panel">
            <div class="settings-section__head">
              <div class="settings-section__label">Retention and session windows</div>
              <div class="settings-section__caption">Control how long sessions and traffic history remain available.</div>
            </div>
            <v-row>
              <v-col cols="12" sm="6" md="4">
                <v-text-field
                  type="number"
                  v-model.number="sessionMaxAge"
                  min="0"
                  :label="$t('setting.sessionAge')"
                  :suffix="$t('date.m')"
                  hide-details
                ></v-text-field>
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-text-field
                  type="number"
                  v-model.number="trafficAge"
                  min="0"
                  :label="$t('setting.trafficAge')"
                  :suffix="$t('date.d')"
                  hide-details
                ></v-text-field>
              </v-col>
            </v-row>
          </section>

          <section class="settings-section app-panel">
            <div class="settings-section__head">
              <div class="settings-section__label">TLS hinting</div>
              <div class="settings-section__caption">Hints used to improve certificate and domain presentation in generated assets.</div>
            </div>
            <v-row>
              <v-col cols="12">
                <v-textarea
                  v-model="settings.tlsDomainHints"
                  :label="$t('setting.tlsDomainHints')"
                  :hint="$t('setting.tlsDomainHintsHint')"
                  persistent-hint
                  rows="5"
                />
              </v-col>
            </v-row>
          </section>
        </div>
      </v-window-item>

      <v-window-item value="t2">
        <div class="settings-stack">
          <section class="settings-section app-panel">
            <div class="settings-section__head">
              <div class="settings-section__label">Subscription behavior</div>
              <div class="settings-section__caption">Encoding and metadata rules for generated subscription links.</div>
            </div>
            <v-row>
              <v-col cols="12" sm="6" md="4">
                <v-switch color="primary" v-model="subEncode" :label="$t('setting.subEncode')" hide-details />
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-switch color="primary" v-model="subShowInfo" :label="$t('setting.subInfo')" hide-details />
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-text-field
                  type="number"
                  v-model.number="subUpdates"
                  min="0"
                  :label="$t('setting.update')"
                  hide-details
                ></v-text-field>
              </v-col>
            </v-row>
          </section>

          <section class="settings-section app-panel">
            <div class="settings-section__head">
              <div class="settings-section__label">Endpoint exposure</div>
              <div class="settings-section__caption">Address, path, domain, and URI controls for the subscription surface.</div>
            </div>
            <v-row>
              <v-col cols="12" sm="6" md="4">
                <v-text-field v-model="settings.subListen" :label="$t('setting.addr')" hide-details></v-text-field>
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-text-field
                  type="number"
                  v-model.number="subPort"
                  min="1"
                  :label="$t('setting.port')"
                  hide-details></v-text-field>
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-text-field v-model="settings.subDomain" :label="$t('setting.domain')" hide-details></v-text-field>
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-text-field v-model="settings.subPath" :label="$t('setting.path')" hide-details></v-text-field>
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-text-field v-model="settings.subURI" :label="$t('setting.subUri')" hide-details></v-text-field>
              </v-col>
            </v-row>
          </section>

          <section class="settings-section app-panel">
            <div class="settings-section__head">
              <div class="settings-section__label">TLS material</div>
              <div class="settings-section__caption">Optional certificate files for a secured subscription endpoint.</div>
            </div>
            <v-row>
              <v-col cols="12" sm="6" md="4">
                <v-text-field v-model="settings.subKeyFile" :label="$t('setting.sslKey')" hide-details></v-text-field>
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-text-field v-model="settings.subCertFile" :label="$t('setting.sslCert')" hide-details></v-text-field>
              </v-col>
            </v-row>
          </section>
        </div>
      </v-window-item>

      <v-window-item value="t3">
        <SubJsonExtVue :settings="settings" />
      </v-window-item>

      <v-window-item value="t4">
        <SubClashExtVue :settings="settings" />
      </v-window-item>
      </v-window>
    </v-card-text>
    </v-card>
  </div>
</template>

<script lang="ts" setup>
import { i18n } from '@/locales'
import { Ref, computed, inject, onMounted, ref } from 'vue'
import HttpUtils from '@/plugins/httputil'
import { FindDiff } from '@/plugins/utils'
import SubJsonExtVue from '@/components/SubJsonExt.vue'
import SubClashExtVue from '@/components/SubClashExt.vue'
import { push } from 'notivue'
import { defaultSettings, normalizeSettings, toNumberSetting } from '@/features/settings/normalize'
const tab = ref("t1")
const loading:Ref = inject('loading')?? ref(false)
const oldSettings = ref({})
const settings = ref({ ...defaultSettings })

onMounted(async () => {
  loading.value = true
  await loadData()
  loading.value = false
})

const loadData = async () => {
  loading.value = true
  const msg = await HttpUtils.get('api/settings')
  loading.value = false
  if (msg.success) {
    setData(msg.obj)
  }
}

const setData = (data: any) => {
  const normalized = normalizeSettings(data)
  settings.value = normalized
  oldSettings.value = JSON.parse(JSON.stringify(normalized))
}

const save = async () => {
  loading.value = true
  const msg = await HttpUtils.post('api/save', { object: 'settings', action: 'set', data: JSON.stringify(settings.value) })
  if (msg.success) {
    push.success({
      title: i18n.global.t('success'),
      duration: 5000,
      message: i18n.global.t('actions.set') + " " + i18n.global.t('pages.settings')
    })
    setData(msg.obj.settings)
  }
  loading.value = false
}

const sleep = (ms: number) => new Promise(resolve => setTimeout(resolve, ms))

const restartApp = async () => {
  loading.value = true
  const msg = await HttpUtils.post('api/restartApp',{})
  if (msg.success) {
    let url = settings.value.webURI
    if (url !== "") {
      const isTLS = settings.value.webCertFile !== "" || settings.value.webKeyFile !== ""
      url = buildURL(settings.value.webDomain,settings.value.webPort.toString(),isTLS, settings.value.webPath)
    }
    await sleep(3000)
    window.location.replace(url)
  }
  loading.value = false
}

const buildURL = (host: string, port: string, isTLS: boolean, path: string) => {
  if (!host || host.length == 0) host = window.location.hostname
  if (!port || port.length == 0) port = window.location.port

  const protocol = isTLS ? "https:" : "http:"

  if (port === "" || (isTLS && port === "443") || (!isTLS && port === "80")) {
      port = ""
  } else {
      port = `:${port}`
  }

  return `${protocol}//${host}${port}${path}settings`
}

const subEncode = computed({
  get: () => { return settings.value.subEncode == "true" },
  set: (v:boolean) => { settings.value.subEncode = v ? "true" : "false" }
})

const subShowInfo = computed({
  get: () => { return settings.value.subShowInfo == "true" },
  set: (v:boolean) => { settings.value.subShowInfo = v ? "true" : "false" }
})

const webPort = computed({
  get: () => { return toNumberSetting(settings.value.webPort, 2095) },
  set: (v:number) => { settings.value.webPort = v>0 ? v.toString() : "2095" }
})

const sessionMaxAge = computed({
  get: () => { return toNumberSetting(settings.value.sessionMaxAge, 0) },
  set: (v:number) => { settings.value.sessionMaxAge = v>0 ? v.toString() : "0" }
})

const trafficAge = computed({
  get: () => { return toNumberSetting(settings.value.trafficAge, 0) },
  set: (v:number) => { settings.value.trafficAge = v>0 ? v.toString() : "0" }
})

const subPort = computed({
  get: () => { return toNumberSetting(settings.value.subPort, 2096) },
  set: (v:number) => { settings.value.subPort = v>0 ? v.toString() : "2096" }
})

const subUpdates = computed({
  get: () => { return toNumberSetting(settings.value.subUpdates, 12) },
  set: (v:number) => { settings.value.subUpdates = v>0 ? v.toString() : "12" }
})

const stateChange = computed(() => {
  return !FindDiff.deepCompare(settings.value,oldSettings.value)
})
</script>

<style scoped>
.settings-stack {
  display: grid;
  gap: 14px;
}

.settings-section {
  padding: 16px;
}

.settings-section__head {
  align-items: end;
  display: grid;
  gap: 8px;
  grid-template-columns: minmax(0, 1fr) minmax(220px, 0.8fr);
  margin-bottom: 14px;
}

.settings-section__label {
  color: var(--app-text-2);
  font-size: 15px;
  font-weight: 600;
  line-height: 1.2;
}

.settings-section__caption {
  color: var(--app-text-3);
  font-size: 12px;
  line-height: 1.5;
  text-align: right;
}

@media (max-width: 720px) {
  .settings-section__head {
    grid-template-columns: 1fr;
  }

  .settings-section__caption {
    text-align: left;
  }
}
</style>
