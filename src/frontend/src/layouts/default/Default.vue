<template>
  <v-app class="shell-app">
    <div class="shell-app__bg"></div>
    <div class="shell-app__aurora"></div>
    <div class="shell-app__scan"></div>
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
    radial-gradient(circle at 12% 14%, var(--app-bg-glow-primary), transparent 28%),
    radial-gradient(circle at 92% 4%, var(--app-bg-glow-danger), transparent 24%),
    radial-gradient(circle at 52% -8%, var(--app-bg-glow-warm), transparent 34%),
    linear-gradient(145deg, color-mix(in srgb, var(--app-bg-ambient) 100%, transparent), transparent 34%),
    linear-gradient(180deg, var(--app-bg-elevated), var(--app-bg-base));
  background-position: center;
  background-size: auto;
  min-height: 100vh;
  overflow: hidden;
  position: relative;
}

.shell-app__bg,
.shell-app__aurora,
.shell-app__scan,
.shell-app__vignette {
  inset: 0;
  pointer-events: none;
  position: fixed;
}

.shell-app__bg {
  background:
    radial-gradient(ellipse at 16% 20%, color-mix(in srgb, var(--app-state-info) 15%, transparent), transparent 34%),
    radial-gradient(ellipse at 82% 12%, color-mix(in srgb, var(--app-state-danger) 13%, transparent), transparent 30%),
    radial-gradient(ellipse at 56% 0%, color-mix(in srgb, var(--app-state-success) 8%, transparent), transparent 36%);
  animation: shell-ambient-drift 18s var(--app-ease-standard) infinite alternate;
  filter: saturate(1.18);
  z-index: 0;
}

.shell-app__aurora {
  background:
    conic-gradient(from 210deg at 20% 0%, transparent 0 18%, color-mix(in srgb, var(--app-state-info) 16%, transparent) 24%, transparent 34% 100%),
    conic-gradient(from 42deg at 86% 4%, transparent 0 12%, color-mix(in srgb, var(--app-state-danger) 12%, transparent) 20%, transparent 30% 100%),
    linear-gradient(115deg, transparent 8%, color-mix(in srgb, #ffffff 4%, transparent) 22%, transparent 38%);
  filter: blur(18px);
  opacity: 0.56;
  transform: translate3d(0, 0, 0);
  animation: shell-aurora-sweep 22s var(--app-ease-standard) infinite alternate;
  z-index: 0;
}

.shell-app__scan {
  background:
    linear-gradient(180deg, transparent 0%, color-mix(in srgb, var(--app-state-info) 7%, transparent) 50%, transparent 100%),
    var(--app-bg-noise);
  background-size: 100% 42%, 180px 180px;
  mix-blend-mode: screen;
  opacity: 0.24;
  animation: shell-scan-pass 10s var(--app-ease-standard) infinite;
  z-index: 0;
}

.shell-app__vignette {
  background:
    radial-gradient(circle at center, transparent 44%, var(--app-bg-vignette-edge) 100%),
    linear-gradient(90deg, color-mix(in srgb, var(--app-bg-base) 34%, transparent), transparent 20% 80%, color-mix(in srgb, var(--app-bg-base) 34%, transparent));
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
  .shell-app__aurora,
  .shell-app__scan {
    opacity: 0.18;
  }
}

@media (prefers-reduced-motion: reduce) {
  .shell-app__bg,
  .shell-app__aurora,
  .shell-app__scan {
    animation: none !important;
  }
}

@keyframes shell-ambient-drift {
  from {
    transform: scale(1) translate3d(-1.5%, -1%, 0);
  }

  to {
    transform: scale(1.05) translate3d(1.5%, 1%, 0);
  }
}

@keyframes shell-aurora-sweep {
  from {
    opacity: 0.42;
    transform: translate3d(-2%, -1%, 0) rotate(-1deg);
  }

  to {
    opacity: 0.62;
    transform: translate3d(2%, 1%, 0) rotate(1deg);
  }
}

@keyframes shell-scan-pass {
  0% {
    background-position: 0 -42vh, 0 0;
  }

  55%,
  100% {
    background-position: 0 110vh, 18px 12px;
  }
}
</style>
