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
      <div class="shell-app__workspace">
        <DefaultBar
          :isMobile="isMobile"
          :collapsed="collapsed"
          @toggleDrawer="toggleDrawer"
        />
        <DefaultView />
      </div>
    </v-layout>
  </v-app>
</template>

<script lang="ts" setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useDisplay } from 'vuetify'
import useAuthStore from '@/store/modules/auth'
import DefaultBar from './AppBar.vue'
import Drawer from './Drawer.vue'
import DefaultView from './View.vue'

const { mdAndDown, width } = useDisplay()
const DRAWER_WIDE_BREAKPOINT = 1680
const auth = useAuthStore()

const isMobile = computed((): boolean => mdAndDown.value)
const isWideScreen = computed((): boolean => width.value >= DRAWER_WIDE_BREAKPOINT)
const displayDrawer = ref(false)
const collapsed = ref(isWideScreen.value ? localStorage.getItem('shell.drawer.collapsed') === '1' : true)

const toggleDrawer = () => {
  if (isMobile.value) {
    displayDrawer.value = !displayDrawer.value
    return
  }

  collapsed.value = !collapsed.value

  if (isWideScreen.value) {
    localStorage.setItem('shell.drawer.collapsed', collapsed.value ? '1' : '0')
  }
}

watch(isMobile, value => {
  if (!value) {
    displayDrawer.value = false
  }
})

watch([isMobile, isWideScreen], ([mobile, wide]) => {
  if (mobile) {
    displayDrawer.value = false
    return
  }

  collapsed.value = wide ? localStorage.getItem('shell.drawer.collapsed') === '1' : true
})

onMounted(() => {
  if (!auth.loaded) {
    void auth.loadAuthState()
  }
})
</script>

<style>
.v-switch.v-input {
  padding-inline-start: 0.6rem;
}

.shell-app {
  background:
    linear-gradient(var(--app-bg-grid) 1px, transparent 1px),
    linear-gradient(90deg, var(--app-bg-grid) 1px, transparent 1px),
    radial-gradient(circle at 10% 10%, var(--app-bg-glow-primary), transparent 24%),
    radial-gradient(circle at 90% 0%, var(--app-bg-glow-danger), transparent 22%),
    radial-gradient(circle at 50% 0%, var(--app-bg-glow-warm), transparent 30%),
    linear-gradient(180deg, color-mix(in srgb, var(--app-bg-ambient) 100%, transparent), transparent 24%),
    linear-gradient(180deg, var(--app-bg-elevated), var(--app-bg-base));
  background-position: center;
  background-size: 52px 52px, 52px 52px, auto, auto, auto, auto, auto;
  min-height: 100vh;
  overflow: hidden;
  position: relative;
}

.shell-app__bg,
.shell-app__mesh,
.shell-app__vignette {
  inset: 0;
  pointer-events: none;
  position: fixed;
}

.shell-app__bg {
  background:
    radial-gradient(circle at top left, color-mix(in srgb, var(--app-state-danger) 12%, transparent), transparent 28%),
    radial-gradient(circle at top right, color-mix(in srgb, var(--app-state-info) 10%, transparent), transparent 22%),
    radial-gradient(circle at center top, color-mix(in srgb, var(--app-bg-glow-warm) 100%, transparent), transparent 34%);
  z-index: 0;
}

.shell-app__mesh {
  background:
    linear-gradient(90deg, transparent 0, transparent calc(100% - 1px), color-mix(in srgb, var(--app-bg-grid-strong) 100%, transparent) calc(100% - 1px)),
    linear-gradient(180deg, transparent 0, transparent calc(100% - 1px), color-mix(in srgb, var(--app-bg-grid-strong) 100%, transparent) calc(100% - 1px));
  background-size: min(28vw, 360px) 100%, 100% min(24vh, 240px);
  opacity: 0.22;
  z-index: 0;
}

.shell-app__vignette {
  background: radial-gradient(circle at center, transparent 50%, var(--app-bg-vignette-edge) 100%);
  z-index: 0;
}

.shell-app__layout {
  min-height: 100vh;
  position: relative;
  z-index: 1;
}

.shell-app__workspace {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-width: 0;
}

@media (max-width: 960px) {
  .shell-app__mesh {
    opacity: 0.12;
  }
}
</style>
