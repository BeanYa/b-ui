<template>
  <div class="cluster-domain-action-tree">
    <div v-if="visibleRows.length === 0" class="cluster-domain-action-tree__empty">
      {{ emptyText }}
    </div>
    <div v-else class="cluster-domain-action-tree__scroll">
      <button
        v-for="row in visibleRows"
        :key="row.key"
        type="button"
        class="cluster-domain-action-tree__row"
        :class="{
          'cluster-domain-action-tree__row--branch': row.hasChildren,
          'cluster-domain-action-tree__row--expanded': expandedKeys.has(row.key),
        }"
        :disabled="!row.hasChildren"
        :style="{ '--cluster-action-depth': String(row.depth) }"
        @click="toggleRow(row)"
      >
        <span class="cluster-domain-action-tree__chevron">
          {{ row.hasChildren ? (expandedKeys.has(row.key) ? 'v' : '>') : '' }}
        </span>
        <span class="cluster-domain-action-tree__label">{{ row.label }}</span>
        <span
          v-if="row.isAction && row.hasChildren"
          class="cluster-domain-action-tree__marker"
        >
          action
        </span>
      </button>
    </div>
  </div>
</template>

<script lang="ts" setup>
import { computed, ref } from 'vue'

import {
  buildClusterDomainActionTree,
  flattenVisibleClusterDomainActionTree,
  type ClusterDomainActionTreeRow,
} from '@/utils/clusterDomainActionTree'

const props = withDefaults(defineProps<{
  supportedActions?: string[] | null
  emptyText?: string
}>(), {
  supportedActions: () => [],
  emptyText: '-',
})

const expandedKeys = ref<Set<string>>(new Set())
const tree = computed(() => buildClusterDomainActionTree(props.supportedActions))
const visibleRows = computed(() => flattenVisibleClusterDomainActionTree(tree.value, expandedKeys.value))

const toggleRow = (row: ClusterDomainActionTreeRow) => {
  if (!row.hasChildren) return

  const next = new Set(expandedKeys.value)
  if (next.has(row.key)) {
    next.delete(row.key)
  } else {
    next.add(row.key)
  }
  expandedKeys.value = next
}
</script>

<style scoped>
.cluster-domain-action-tree {
  min-height: 0;
}

.cluster-domain-action-tree__scroll {
  display: grid;
  gap: 4px;
  max-height: 240px;
  min-height: 240px;
  overflow: auto;
  padding-right: 4px;
}

.cluster-domain-action-tree__empty {
  align-items: center;
  color: var(--app-text-3);
  display: flex;
  min-height: 240px;
}

.cluster-domain-action-tree__row {
  align-items: center;
  background: transparent;
  border: 0;
  border-radius: 12px;
  color: inherit;
  cursor: default;
  display: flex;
  gap: 8px;
  min-height: 34px;
  padding: 8px 10px;
  padding-inline-start: calc(10px + (var(--cluster-action-depth) * 18px));
  text-align: left;
  width: 100%;
}

.cluster-domain-action-tree__row:disabled {
  opacity: 1;
}

.cluster-domain-action-tree__row--branch {
  cursor: pointer;
}

.cluster-domain-action-tree__row--branch:hover {
  background: color-mix(in srgb, var(--app-state-info) 10%, transparent);
}

.cluster-domain-action-tree__row--expanded {
  background: color-mix(in srgb, var(--app-state-info) 7%, transparent);
}

.cluster-domain-action-tree__chevron {
  color: var(--app-text-3);
  flex: 0 0 14px;
  text-align: center;
}

.cluster-domain-action-tree__label {
  overflow-wrap: anywhere;
}

.cluster-domain-action-tree__marker {
  background: color-mix(in srgb, var(--app-state-info) 12%, transparent);
  border-radius: 999px;
  color: var(--app-text-2);
  font-size: 11px;
  line-height: 1;
  margin-left: auto;
  padding: 3px 7px;
  text-transform: lowercase;
}

@media (max-width: 640px) {
  .cluster-domain-action-tree__scroll,
  .cluster-domain-action-tree__empty {
    max-height: 208px;
    min-height: 208px;
  }
}
</style>
