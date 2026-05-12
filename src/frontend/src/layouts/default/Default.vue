<template>
  <v-app class="shell-app">
    <div class="shell-app__bg"></div>
    <div class="shell-app__clay-scene" aria-hidden="true">
      <span></span>
      <span></span>
      <span></span>
    </div>
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
      <div
        class="shell-app__workspace"
        :class="{ 'shell-app__workspace--expanded-nav': !isMobile && !collapsed }"
      >
        <section class="shell-frame">
          <div class="shell-frame__header">
            <DefaultBar
              :isMobile="isMobile"
              :collapsed="collapsed"
              @toggleDrawer="toggleDrawer"
            />
          </div>
          <div class="shell-frame__body">
            <DefaultView />
          </div>
        </section>
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
    radial-gradient(ellipse at 18% 8%, var(--app-bg-glow-warm), transparent 38%),
    radial-gradient(ellipse at 78% 6%, var(--app-bg-glow-primary), transparent 30%),
    radial-gradient(ellipse at 94% 70%, var(--app-bg-glow-danger), transparent 32%),
    linear-gradient(160deg, var(--app-bg-elevated), var(--app-bg-base));
  background-position: center;
  background-size: auto;
  min-height: 100vh;
  overflow: hidden;
  position: relative;
}

.shell-app__bg,
.shell-app__clay-scene,
.shell-app__aurora,
.shell-app__scan,
.shell-app__vignette {
  inset: 0;
  pointer-events: none;
  position: fixed;
}

.shell-app__bg {
  background:
    linear-gradient(110deg, color-mix(in srgb, #ffffff 8%, transparent), transparent 28%),
    radial-gradient(ellipse at 76% 14%, color-mix(in srgb, var(--app-brand-lavender) 18%, transparent), transparent 34%);
  animation: shell-ambient-drift 18s var(--app-ease-standard) infinite alternate;
  filter: saturate(1.18);
  z-index: 0;
}

.shell-app__clay-scene {
  z-index: 0;
}

.shell-app__clay-scene span {
  border: 1px solid color-mix(in srgb, var(--app-border-1) 60%, transparent);
  border-radius: 44% 56% 54% 46% / 58% 42% 52% 48%;
  box-shadow: var(--app-shadow-soft);
  display: block;
  position: absolute;
  transform: translate3d(0, 0, 0);
}

.shell-app__clay-scene span:nth-child(1) {
  animation: shell-clay-float 12s var(--app-ease-standard) infinite alternate;
  background: color-mix(in srgb, var(--app-brand-pink) 82%, var(--app-surface-1));
  height: 172px;
  left: 5vw;
  top: 11vh;
  width: 172px;
}

.shell-app__clay-scene span:nth-child(2) {
  animation: shell-clay-float 15s var(--app-ease-standard) infinite alternate-reverse;
  background: color-mix(in srgb, var(--app-brand-peach) 86%, var(--app-surface-1));
  bottom: 7vh;
  height: 220px;
  right: 8vw;
  width: 220px;
}

.shell-app__clay-scene span:nth-child(3) {
  animation: shell-clay-float 14s var(--app-ease-standard) infinite alternate;
  background: color-mix(in srgb, var(--app-brand-ochre) 82%, var(--app-surface-1));
  height: 118px;
  right: 28vw;
  top: 4vh;
  width: 118px;
}

.shell-app__aurora {
  background:
    radial-gradient(ellipse at 14% 86%, color-mix(in srgb, var(--app-brand-mint) 18%, transparent), transparent 28%),
    linear-gradient(128deg, transparent 12%, color-mix(in srgb, #ffffff 7%, transparent) 30%, transparent 52%);
  filter: blur(24px);
  opacity: 0.5;
  transform: translate3d(0, 0, 0);
  animation: shell-aurora-sweep 22s var(--app-ease-standard) infinite alternate;
  z-index: 0;
}

.shell-app__scan {
  background: linear-gradient(120deg, transparent 0 28%, color-mix(in srgb, #ffffff 8%, transparent) 46%, transparent 66% 100%);
  opacity: 0.18;
  animation: shell-scan-pass 14s var(--app-ease-standard) infinite;
  z-index: 0;
}

.shell-app__vignette {
  background:
    radial-gradient(circle at center, transparent 48%, var(--app-bg-vignette-edge) 100%),
    linear-gradient(110deg, color-mix(in srgb, var(--app-bg-base) 24%, transparent), transparent 24% 78%, color-mix(in srgb, var(--app-bg-base) 20%, transparent));
  z-index: 0;
}

.shell-app__layout {
  min-height: 100vh;
  position: relative;
  z-index: 1;
}

.shell-app__workspace {
  --shell-drawer-offset: 104px;
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  margin-left: var(--shell-drawer-offset);
  min-width: 0;
  padding: 18px 18px 30px;
  transition: margin var(--app-motion-base) var(--app-ease-standard), padding var(--app-motion-base) var(--app-ease-standard);
}

.shell-app__workspace--expanded-nav {
  --shell-drawer-offset: 320px;
}

.shell-frame {
  background:
    radial-gradient(circle at 72% 0%, color-mix(in srgb, var(--app-brand-ochre) 13%, transparent), transparent 26%),
    radial-gradient(circle at 10% 100%, color-mix(in srgb, var(--app-state-success) 7%, transparent), transparent 26%),
    linear-gradient(135deg, color-mix(in srgb, #ffffff 10%, transparent), transparent 38%),
    var(--app-surface-1);
  border: 1px solid var(--app-border-1);
  border-radius: 34px;
  box-shadow: var(--app-shadow-device);
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-height: calc(100vh - 48px);
  min-width: 0;
  overflow: hidden;
  position: relative;
}

.shell-frame::before {
  background:
    radial-gradient(circle at 94% 42%, color-mix(in srgb, var(--app-state-warning) 8%, transparent), transparent 24%),
    linear-gradient(90deg, color-mix(in srgb, var(--app-bg-base) 3%, transparent), transparent 18% 82%, color-mix(in srgb, var(--app-bg-base) 4%, transparent));
  content: '';
  inset: 0;
  pointer-events: none;
  position: absolute;
}

.shell-frame__header,
.shell-frame__body {
  position: relative;
  z-index: 1;
}

.shell-frame__header {
  border-bottom: 1px solid color-mix(in srgb, var(--app-border-1) 72%, transparent);
}

.shell-frame__body {
  display: flex;
  flex: 1 1 auto;
  min-height: 0;
}

@media (max-width: 960px) {
  .shell-app__aurora,
  .shell-app__scan {
    opacity: 0.14;
  }

  .shell-app__workspace {
    --shell-drawer-offset: 0px;
    margin-left: 0;
    padding: 10px 10px 18px;
  }

  .shell-frame {
    border-radius: 26px;
    min-height: calc(100vh - 28px);
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
    transform: translateX(-42%) skewX(-8deg);
  }

  55%,
  100% {
    transform: translateX(42%) skewX(-8deg);
  }
}

@keyframes shell-clay-float {
  from {
    filter: saturate(0.96);
    transform: translate3d(-10px, -8px, 0) rotate(-4deg);
  }

  to {
    filter: saturate(1.08);
    transform: translate3d(10px, 8px, 0) rotate(4deg);
  }
}
</style>
