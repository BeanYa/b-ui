import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

describe('ClusterDomainActionTree source', () => {
  it('uses the shared tree utility and keeps expanded keys local to the component', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterDomainActionTree.vue', import.meta.url)), 'utf8')

    expect(source).toContain('buildClusterDomainActionTree')
    expect(source).toContain('flattenVisibleClusterDomainActionTree')
    expect(source).toContain('const expandedKeys = ref<Set<string>>(new Set())')
    expect(source).toContain('const visibleRows = computed(() => flattenVisibleClusterDomainActionTree(tree.value, expandedKeys.value))')
  })

  it('renders a scrollable row list, an inline empty state, and branch-only toggling', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterDomainActionTree.vue', import.meta.url)), 'utf8')

    expect(source).toContain('v-for="row in visibleRows"')
    expect(source).toContain('class="cluster-domain-action-tree__empty"')
    expect(source).toContain('class="cluster-domain-action-tree__scroll"')
    expect(source).toContain('row.isAction && row.hasChildren')
    expect(source).toContain(':disabled="!row.hasChildren"')
    expect(source).toContain('if (!row.hasChildren) return')
    expect(source).toContain("cluster-domain-action-tree__row--expanded")
  })
})
