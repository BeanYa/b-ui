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

<style>
.shell-main {
  min-height: 100vh;
  overflow: auto;
}

.shell-main__inner {
  min-height: 100%;
  padding: 8px 18px 30px;
  width: 100%;
}

.shell-main__content {
  min-height: calc(100vh - 124px);
  padding: 8px 0 4px;
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
  filter: blur(8px);
  opacity: 0;
  transform: translateY(12px);
}

@media (max-width: 960px) {
  .shell-main__inner {
    padding: 4px 10px 18px;
  }

  .shell-main__content {
    min-height: calc(100vh - 98px);
    padding: 8px 0 2px;
  }
}
</style>
