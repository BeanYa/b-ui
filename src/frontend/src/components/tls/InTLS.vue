<template>
  <v-card :subtitle="$t('objects.tls')">
    <v-row>
      <v-col cols="12" sm="6" md="4">
        <v-select
          hide-details
          :label="$t('template')"
          :items="tlsItems"
          v-model="inbound.tls_id">
        </v-select>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-menu location="bottom">
          <template v-slot:activator="{ props }">
            <v-btn
              v-bind="props"
              block
              variant="tonal"
              color="primary"
              :loading="creating"
            >
              {{ $t('tls.quickCreate') }}
            </v-btn>
          </template>
          <v-list>
            <v-list-item
              v-for="preset in presetItems"
              :key="preset.value"
              :title="preset.title"
              @click="createPreset(preset.value)"
            />
          </v-list>
        </v-menu>
      </v-col>
    </v-row>
  </v-card>
</template>

<script lang="ts">
import { createMaterializedTlsPreset } from '@/plugins/tlsPresetMaterial'
import { ensureUniqueTlsName, getTlsPresetBaseName, type TlsPresetKey } from '@/plugins/tlsTemplates'
import { i18n } from '@/locales'
import Data from '@/store/modules/data'
import { push } from 'notivue'
export default {
  props: ['inbound', 'tlsConfigs'],
  data() {
    return {
      creating: false,
      presetItems: [
        { title: i18n.global.t('tls.presets.standard').toString(), value: 'standard' },
        { title: i18n.global.t('tls.presets.hysteria2').toString(), value: 'hysteria2' },
        { title: i18n.global.t('tls.presets.reality').toString(), value: 'reality' },
      ] as { title: string, value: TlsPresetKey }[],
    }
  },
  methods: {
    async createPreset(preset: TlsPresetKey) {
      this.creating = true
      try {
        const name = ensureUniqueTlsName(
          getTlsPresetBaseName(preset),
          this.$props.tlsConfigs?.map((item: any) => item.name) ?? [],
        )
        const payload = await createMaterializedTlsPreset(preset, name)
        const success = await Data().save('tls', 'new', payload)
        if (success) {
          const created = Data().tlsConfigs.findLast((item: any) => item.name === name)
          if (created) {
            this.$props.inbound.tls_id = created.id
          }
        }
      } catch (error: any) {
        push.error({
          title: i18n.global.t('failed').toString(),
          message: error?.message ?? String(error),
        })
      } finally {
        this.creating = false
      }
    },
  },
  computed: {
    tlsItems(): any[] {
      return [ { title: i18n.global.t('none'), value: 0 }, ...this.$props.tlsConfigs?.map((t:any) => { return { title: t.name, value: t.id } } )]
    }
  }
}
</script>
