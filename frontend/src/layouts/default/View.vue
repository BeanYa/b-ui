<template>
  <v-main class="shell-main">
    <div class="shell-main__inner">
      <div class="shell-main__content">
        <router-view v-slot="{ Component, route }">
          <transition mode="out-in" name="page-shell">
            <keep-alive v-if="route.meta.keepAlive">
              <component :is="Component" :key="String(route.name)" />
            </keep-alive>
            <component v-else :is="Component" :key="route.fullPath" />
          </transition>
        </router-view>
      </div>
    </div>
  </v-main>
</template>

<script lang="ts" setup>
</script>

<style>
.shell-main {
  min-height: 100vh;
  overflow: auto;
}

.shell-main__inner {
  margin: 0 auto;
  max-width: 1820px;
  min-height: 100%;
  padding: 8px 18px 30px;
  width: 100%;
}

.shell-main__content {
  background: linear-gradient(180deg, color-mix(in srgb, var(--app-surface-0) 54%, transparent), transparent 18%);
  border-radius: 34px;
  min-height: calc(100vh - 112px);
  padding: 10px 10px 4px;
}

.page-shell-enter-active,
.page-shell-leave-active {
  transition:
    opacity var(--app-motion-base) var(--app-ease-standard),
    transform var(--app-motion-base) var(--app-ease-standard),
    filter var(--app-motion-base) var(--app-ease-standard);
}

.page-shell-enter-from,
.page-shell-leave-to {
  filter: blur(6px);
  opacity: 0;
  transform: translateY(12px);
}

@media (max-width: 960px) {
  .shell-main__inner {
    padding: 4px 10px 18px;
  }

  .shell-main__content {
    border-radius: 24px;
    min-height: calc(100vh - 92px);
    padding: 8px 6px 2px;
  }
}
</style>
