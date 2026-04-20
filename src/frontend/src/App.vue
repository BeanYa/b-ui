<template>
  <v-overlay
    :model-value="loading"
    persistent
    content-class="text-center"
    class="align-center justify-center"
  >
    <v-progress-circular
      indeterminate
      size="64"
    ></v-progress-circular>
    <br />
    {{ $t('loading') }}
  </v-overlay>
  <Message />
  <router-view />
</template>

<script lang="ts" setup>
import Message from '@/components/message.vue'
import { inject, onBeforeUnmount, onMounted, ref, Ref } from 'vue'
import { useTheme } from 'vuetify'
import { applyThemePreference, getThemePreference, startThemeSync, stopThemeSync } from '@/plugins/theme'

const loading:Ref = inject('loading')?? ref(false)
const theme = useTheme()

const syncDocumentTitle = () => {
  document.title = `B-UI ${document.location.hostname}`
}

onMounted(() => {
  syncDocumentTitle()
  applyThemePreference(theme, getThemePreference(), false)
  startThemeSync(theme)
})

onBeforeUnmount(() => {
  stopThemeSync()
})

</script>

<style>
.v-overlay .v-list-item,
.v-field__input {
  direction: ltr;
}

.v-progress-circular__overlay {
  stroke: #55b3ff;
}

.v-overlay .v-progress-circular {
  filter: drop-shadow(0 0 18px rgba(85, 179, 255, 0.35));
}
</style>
