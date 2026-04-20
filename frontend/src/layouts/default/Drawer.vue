<template>
  <v-navigation-drawer
    v-model="showDrawer"
    class="app-drawer"
    :temporary="isMobile"
    :permanent="!isMobile"
    :rail="!isMobile && collapsed"
    :rail-width="92"
    :width="308"
  >
    <div class="app-drawer__brand">
      <div class="app-drawer__logo">
        <v-img src="@/assets/logo.svg" width="28" />
      </div>
      <div v-if="!collapsed || isMobile" class="app-drawer__titles">
        <div class="app-drawer__name">B-UI</div>
        <div class="app-drawer__meta">Segmented control surface</div>
      </div>
      <v-btn
        v-if="isMobile"
        class="app-drawer__close"
        icon="mdi-close"
        variant="text"
        @click="$emit('toggleDrawer')"
      />
    </div>

    <div
      v-for="group in menuGroups"
      :key="group.label"
      class="app-drawer__group"
    >
      <div class="app-drawer__section" v-if="!collapsed || isMobile">{{ group.label }}</div>
      <v-list class="app-drawer__list" density="comfortable" nav>
        <v-list-item
          v-for="item in group.items"
          :key="item.title"
          class="app-drawer__item"
          link
          :to="item.path"
          :active="router.currentRoute.value.path === item.path"
        >
          <template #prepend>
            <v-icon :icon="item.icon" />
          </template>
          <v-tooltip
            v-if="collapsed && !isMobile"
            activator="parent"
            location="end"
            :text="$t(item.title)"
          />
          <v-list-item-title v-if="!collapsed || isMobile" v-text="$t(item.title)" />
        </v-list-item>
      </v-list>
    </div>

    <template #append>
      <div class="app-drawer__footer">
        <div v-if="!collapsed || isMobile" class="app-drawer__footer-note">
          <span class="app-drawer__footer-label">Runtime route</span>
          <span class="app-drawer__footer-value">{{ router.currentRoute.value.path }}</span>
        </div>
        <v-btn
          class="app-drawer__logout"
          :block="!collapsed || isMobile"
          color="error"
          variant="tonal"
          :icon="collapsed && !isMobile"
          @click="logoutUser"
        >
          <v-tooltip
            v-if="collapsed && !isMobile"
            activator="parent"
            location="end"
            :text="$t('menu.logout')"
          />
          <v-icon v-if="collapsed && !isMobile" icon="mdi-logout" />
          <template v-if="!collapsed || isMobile">
            <v-icon icon="mdi-logout" start />
            {{ $t('menu.logout') }}
          </template>
        </v-btn>
      </div>
    </template>
  </v-navigation-drawer>
</template>

<script lang="ts" setup>
import { computed } from 'vue'
import router from '@/router'
import { logout } from '@/plugins/httputil'

const props = defineProps(['isMobile', 'displayDrawer', 'collapsed'])
const emit = defineEmits(['toggleDrawer'])

const showDrawer = computed({
  get: (): boolean => props.isMobile ? props.displayDrawer : true,
  set: (value: boolean) => {
    if (props.isMobile && value !== props.displayDrawer) {
      emit('toggleDrawer')
    }
  },
})

const menuGroups = [
  {
    label: 'Overview',
    items: [
      { title: 'pages.home', icon: 'mdi-home', path: '/' },
      { title: 'pages.clients', icon: 'mdi-account-multiple', path: '/clients' },
    ],
  },
  {
    label: 'Catalog',
    items: [
      { title: 'pages.inbounds', icon: 'mdi-cloud-download', path: '/inbounds' },
      { title: 'pages.outbounds', icon: 'mdi-cloud-upload', path: '/outbounds' },
      { title: 'pages.endpoints', icon: 'mdi-cloud-tags', path: '/endpoints' },
      { title: 'pages.services', icon: 'mdi-server', path: '/services' },
      { title: 'pages.tls', icon: 'mdi-certificate', path: '/tls' },
      { title: 'pages.rules', icon: 'mdi-routes', path: '/rules' },
      { title: 'pages.admins', icon: 'mdi-account-tie', path: '/admins' },
    ],
  },
  {
    label: 'Configuration',
    items: [
      { title: 'pages.basics', icon: 'mdi-application-cog', path: '/basics' },
      { title: 'pages.dns', icon: 'mdi-dns', path: '/dns' },
      { title: 'pages.settings', icon: 'mdi-cog', path: '/settings' },
    ],
  },
]

const logoutUser = async () => {
  logout()
}
</script>

<style scoped>
.app-drawer {
  border-inline-end: 1px solid var(--app-border-1);
}

.app-drawer__brand {
  align-items: center;
  display: flex;
  flex-shrink: 0;
  gap: 12px;
  min-height: 76px;
  padding: 16px 14px 10px;
}

.app-drawer__logo {
  align-items: center;
  background:
    linear-gradient(180deg, color-mix(in srgb, var(--app-state-danger) 14%, transparent), color-mix(in srgb, var(--app-state-info) 8%, transparent)),
    color-mix(in srgb, var(--app-surface-3) 84%, transparent);
  border: 1px solid var(--app-border-1);
  border-radius: 18px;
  box-shadow: 0 16px 36px rgba(0, 0, 0, 0.18);
  display: inline-flex;
  flex-shrink: 0;
  height: 54px;
  justify-content: center;
  width: 54px;
}

.app-drawer__titles {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.app-drawer__name {
  color: var(--app-text-1);
  font-size: 18px;
  font-weight: 700;
}

.app-drawer__meta {
  color: var(--app-text-3);
  font-size: 12px;
}

.app-drawer__close {
  border: 1px solid var(--app-border-1);
}

.app-drawer__group {
  display: grid;
  gap: 6px;
  padding-top: 4px;
}

.app-drawer__section {
  color: var(--app-text-4);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.22em;
  padding: 8px 14px 4px;
  text-transform: uppercase;
}

.app-drawer__list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-height: 0;
  padding-inline-end: 4px;
}

.app-drawer__item {
  backdrop-filter: blur(12px);
  border: 1px solid transparent;
  min-height: 50px;
}

.app-drawer__item:hover {
  background: color-mix(in srgb, var(--app-surface-3) 86%, transparent);
  border-color: var(--app-border-1);
  transform: translateX(2px);
}

.app-drawer__footer {
  display: grid;
  gap: 12px;
  padding: 12px;
}

.app-drawer__footer-note {
  background: color-mix(in srgb, var(--app-surface-3) 86%, transparent);
  border: 1px solid var(--app-border-1);
  border-radius: 18px;
  display: grid;
  gap: 4px;
  min-height: 72px;
  padding: 12px;
}

.app-drawer__footer-label {
  color: var(--app-text-4);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.app-drawer__footer-value {
  color: var(--app-text-2);
  font-size: 13px;
  overflow-wrap: anywhere;
}

:deep(.v-navigation-drawer__content) {
  background:
    radial-gradient(circle at top left, color-mix(in srgb, var(--app-state-info) 8%, transparent), transparent 24%),
    linear-gradient(180deg, color-mix(in srgb, #ffffff 3%, transparent), transparent 24%);
  display: flex;
  flex-direction: column;
  gap: 4px;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  padding: 12px 10px;
}

:deep(.v-navigation-drawer--rail .v-navigation-drawer__content) {
  padding-inline: 8px;
}

:deep(.v-navigation-drawer--rail .app-drawer__list) {
  align-items: center;
  padding-inline: 0;
}

:deep(.v-navigation-drawer--rail .app-drawer__brand) {
  justify-content: center;
  padding-inline: 0;
}

:deep(.v-navigation-drawer--rail .app-drawer__section) {
  display: none;
}

:deep(.v-navigation-drawer--rail .app-drawer__item) {
  align-items: center;
  display: grid;
  grid-template: 'prepend' / minmax(0, 1fr) !important;
  justify-content: center;
  justify-items: center !important;
  margin-inline: auto;
  min-height: 52px;
  padding-inline: 0 !important;
  place-items: center;
  width: 56px !important;
}

:deep(.v-navigation-drawer--rail .app-drawer__footer-note) {
  display: none;
}

:deep(.v-navigation-drawer--rail .app-drawer__item .v-list-item__overlay) {
  left: 0;
  right: 0;
}

:deep(.v-navigation-drawer--rail .app-drawer__item .v-list-item__content) {
  display: none;
}

:deep(.v-navigation-drawer--rail .app-drawer__item .v-list-item__prepend) {
  align-items: center;
  display: flex;
  grid-area: prepend;
  justify-content: center !important;
  margin-inline: 0 !important;
  padding-inline: 0;
  width: 100% !important;
}

:deep(.v-navigation-drawer--rail .app-drawer__item .v-list-item__prepend > .v-icon) {
  margin-inline: auto;
}

:deep(.v-navigation-drawer--rail .app-drawer__item .v-list-item__spacer) {
  display: none !important;
}

:deep(.v-navigation-drawer--rail .app-drawer__item .v-list-item__prepend > .v-list-item__spacer) {
  display: none;
}

:deep(.v-navigation-drawer--rail .app-drawer__footer) {
  justify-items: center;
  padding-inline: 8px;
}

:deep(.v-navigation-drawer--rail .app-drawer__logout) {
  min-height: 52px;
  min-width: 52px;
  padding-inline: 0;
  width: 52px;
}

@media (max-width: 960px) {
  :deep(.v-navigation-drawer__content) {
    padding-inline: 12px;
  }
}
</style>
