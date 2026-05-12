<template>
  <main class="shell-main">
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
  </main>
</template>

<style>
.shell-main {
  display: flex;
  flex: 1 1 auto;
  min-height: 0;
  overflow: auto;
  width: 100%;
}

.shell-main__inner {
  display: flex;
  flex: 1 1 auto;
  min-height: 0;
  padding: 0;
  width: 100%;
}

.shell-main__content {
  display: flex;
  flex: 1 1 auto;
  min-height: 0;
  padding: 0;
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
    padding: 0;
  }

  .shell-main__content {
    padding: 0;
  }
}
</style>
