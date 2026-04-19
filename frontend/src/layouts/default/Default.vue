<template>
  <v-app class="shell-app">
    <div class="shell-app__bg"></div>
    <div class="shell-app__mesh"></div>
    <div class="shell-app__vignette"></div>
    <v-layout class="shell-app__layout">
      <Drawer
        :isMobile="isMobile"
        :displayDrawer="displayDrawer"
        :collapsed="collapsed"
        @toggleDrawer="toggleDrawer"
      />
      <DefaultBar
        :isMobile="isMobile"
        :collapsed="collapsed"
        @toggleDrawer="toggleDrawer"
      />
      <DefaultView />
    </v-layout>
  </v-app>
</template>

<script lang="ts" setup>
import { computed, ref, watch } from 'vue'
import { useDisplay } from 'vuetify'
import DefaultBar from './AppBar.vue'
import Drawer from './Drawer.vue'
import DefaultView from './View.vue'

const { mdAndDown } = useDisplay()

const isMobile = computed((): boolean => mdAndDown.value)
const displayDrawer = ref(false)
const collapsed = ref(localStorage.getItem('shell.drawer.collapsed') === '1')

const toggleDrawer = () => {
  if (isMobile.value) {
    displayDrawer.value = !displayDrawer.value
    return
  }
  collapsed.value = !collapsed.value
}

watch(collapsed, value => {
  localStorage.setItem('shell.drawer.collapsed', value ? '1' : '0')
})

watch(isMobile, value => {
  if (!value) {
    displayDrawer.value = false
  }
})
</script>

<style>
.v-card-subtitle {
  text-align: center;
  border-bottom: 1px solid gray;
  min-height: 20px;
}

.v-switch.v-input {
  padding-inline-start: .6rem;
}

.shell-app {
  min-height: 100vh;
  overflow: hidden;
  position: relative;
}

.shell-app__bg {
  background:
    radial-gradient(circle at top left, color-mix(in srgb, var(--app-state-info) 12%, transparent), transparent 26%),
    radial-gradient(circle at top right, color-mix(in srgb, var(--app-state-danger) 10%, transparent), transparent 20%),
    linear-gradient(180deg, color-mix(in srgb, var(--app-surface-0) 42%, transparent), transparent 30%);
  inset: 0;
  pointer-events: none;
  position: fixed;
  z-index: 0;
}

.shell-app__mesh,
.shell-app__vignette {
  inset: 0;
  pointer-events: none;
  position: fixed;
}

.shell-app__mesh {
  background:
    linear-gradient(90deg, transparent 0, transparent calc(100% - 1px), color-mix(in srgb, var(--app-border-1) 80%, transparent) calc(100% - 1px)),
    linear-gradient(180deg, transparent 0, transparent calc(100% - 1px), color-mix(in srgb, var(--app-border-1) 80%, transparent) calc(100% - 1px));
  background-size: min(24vw, 320px) 100%, 100% min(22vh, 220px);
  opacity: 0.2;
  z-index: 0;
}

.shell-app__vignette {
  background:
    radial-gradient(circle at center, transparent 48%, rgba(5, 9, 15, 0.22) 100%);
  z-index: 0;
}

.shell-app__layout {
  min-height: 100vh;
  position: relative;
  z-index: 1;
}

@media (max-width: 960px) {
  .shell-app__mesh {
    opacity: 0.12;
  }
}
</style>
