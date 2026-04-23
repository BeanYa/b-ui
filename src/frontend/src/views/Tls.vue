<template>
  <div class="app-page">
    <TlsVue 
      v-model="modal.visible"
      :visible="modal.visible"
      :id="modal.id"
      :data="modal.data"
      @close="closeModal"
      @save="saveModal"
    />
    <v-row class="app-page__toolbar">
      <v-col cols="12">
        <div class="app-page__toolbar-actions">
          <v-btn color="primary" @click="showModal(0)">{{ $t('actions.add') }}</v-btn>
        </div>
      </v-col>
    </v-row>
    <v-row class="app-grid">
      <v-col cols="12">
        <v-card class="app-card-shell" rounded="xl" variant="tonal">
          <v-card-title>{{ $t('tls.builtIn') }}</v-card-title>
          <v-card-text class="d-flex flex-wrap ga-3">
            <v-btn
              v-for="preset in presetItems"
              :key="preset.value"
              variant="outlined"
              color="primary"
              :loading="presetLoading === preset.value"
              @click="showPresetModal(preset.value)"
            >
              {{ preset.title }}
            </v-btn>
          </v-card-text>
        </v-card>
      </v-col>
    </v-row>
    <v-row class="app-grid">
      <v-col cols="12" md="6" lg="4" xl="3" v-for="(item, index) in <any[]>tlsConfigs" :key="item.id">
        <v-card class="app-entity-card" elevation="5" :title="item.name">
          <v-card-subtitle class="app-entity-card__subtitle">
            {{ item.server?.server_name?.length>0 ? item.server.server_name : "-" }}
          </v-card-subtitle>
          <v-card-text class="app-entity-card__text">
          <v-row>
            <v-col>{{ $t('pages.inbounds') }}</v-col>
            <v-col>
              <template v-if="tlsInbounds(item.id).length>0">
                <v-tooltip activator="parent" dir="ltr" location="bottom">
                  <span v-for="i in tlsInbounds(item.id)">{{ i }}<br /></span>
                </v-tooltip>
                {{ tlsInbounds(item.id).length }}
              </template>
              <template v-else>-</template>
            </v-col>
          </v-row>
          <v-row>
            <v-col>ACME</v-col>
            <v-col>
              {{ $t(item.server?.acme == undefined ? 'no' : 'yes') }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>ECH</v-col>
            <v-col>
              {{ $t(item.server?.ech == undefined ? 'no' : 'yes') }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>Reality</v-col>
            <v-col>
              {{ $t(item.server?.reality == undefined ? 'no' : 'yes') }}
            </v-col>
          </v-row>
          </v-card-text>
        <v-divider></v-divider>
          <v-card-actions class="app-card-actions">
          <v-btn icon="mdi-file-edit" @click="showModal(item.id)">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('actions.edit')"></v-tooltip>
          </v-btn>
          <v-btn v-if="tlsInbounds(item.id).length == 0" icon="mdi-file-remove" color="warning" @click="delOverlay[index] = true">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('actions.del')"></v-tooltip>
          </v-btn>
          <v-overlay
            v-model="delOverlay[index]"
            contained
            class="align-center justify-center"
          >
            <v-card :title="$t('actions.del')" rounded="lg">
              <v-divider></v-divider>
              <v-card-text>{{ $t('confirm') }}</v-card-text>
              <v-card-actions>
                <v-btn color="error" variant="outlined" @click="delTls(item.id)">{{ $t('yes') }}</v-btn>
                <v-btn color="success" variant="outlined" @click="delOverlay[index] = false">{{ $t('no') }}</v-btn>
              </v-card-actions>
            </v-card>
          </v-overlay>
          <v-btn icon="mdi-content-duplicate" @click="clone(item)">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('actions.clone')"></v-tooltip>
          </v-btn>
          </v-card-actions>
        </v-card>
      </v-col>
    </v-row>
  </div>
</template>

<script lang="ts" setup>
import TlsVue from '@/layouts/modals/Tls.vue'
import { i18n } from '@/locales'
import { createMaterializedTlsPreset } from '@/plugins/tlsPresetMaterial'
import { ensureUniqueTlsName, getTlsPresetBaseName, type TlsPresetKey } from '@/plugins/tlsTemplates'
import Data from '@/store/modules/data'
import { computed, ref } from 'vue'
import { Inbound } from '@/types/inbounds'
import { tls } from '@/types/tls'
import { push } from 'notivue'

const tlsConfigs = computed((): any[] => {
  return Data().tlsConfigs
})

const inbounds = computed((): Inbound[] => {
  return Data().inbounds
})

const tlsInbounds = (id: number): string[] => {
  return inbounds.value.filter(i => i.tls_id == id).map(i => i.tag)  
}

const modal = ref({
  visible: false,
  id: 0,
  data: "",
})
const presetLoading = ref<TlsPresetKey | ''>('')

const presetItems: { title: string, value: TlsPresetKey }[] = [
  { title: i18n.global.t('tls.presets.standard').toString(), value: 'standard' },
  { title: i18n.global.t('tls.presets.hysteria2').toString(), value: 'hysteria2' },
  { title: i18n.global.t('tls.presets.reality').toString(), value: 'reality' },
]

const delOverlay = ref(new Array<boolean>(tlsConfigs.value.length).fill(false))

const showModal = (id: number, data?: tls) => {
  modal.value.id = id
  modal.value.data = data ? JSON.stringify(data) : (id == 0 ? '{}' : JSON.stringify(tlsConfigs.value.findLast(t => t.id == id)))
  modal.value.visible = true
}
const showPresetModal = async (preset: TlsPresetKey) => {
  const name = ensureUniqueTlsName(
    getTlsPresetBaseName(preset),
    tlsConfigs.value.map(item => item.name),
  )
  presetLoading.value = preset
  try {
    showModal(0, await createMaterializedTlsPreset(preset, name))
  } catch (error: any) {
    push.error({
      title: i18n.global.t('failed').toString(),
      message: error?.message ?? String(error),
    })
  } finally {
    presetLoading.value = ''
  }
}
const clone = (obj: any) => {
  let data = JSON.parse(JSON.stringify(obj))
  data.id = 0
  while (tlsConfigs.value.findIndex(t => t.name == data.name) != -1){
    data.name += "-copy"
  }
  saveModal(data)
}
const closeModal = () => {
  modal.value.visible = false
}
const saveModal = async (data:tls) => {
  const success = await Data().save("tls", data.id > 0 ? "edit" : "new", data)
  if (success) modal.value.visible = false
}

const delTls = async (id: number) => {
  const index = tlsConfigs.value.findIndex(t => t.id == id)
  const success = await Data().save("tls", "del", id)
  if (success) delOverlay.value[index] = false
}

</script>
