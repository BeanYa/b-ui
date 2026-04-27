export interface ClusterDomainActionTreeNode {
  key: string
  label: string
  children: ClusterDomainActionTreeNode[]
}

export interface ClusterDomainActionTreeRow {
  key: string
  label: string
  depth: number
  hasChildren: boolean
}

const splitActionSegments = (value: string) => {
  const segments = value.split('.').map(segment => segment.trim())

  return segments.some(segment => segment.length === 0) ? [] : segments
}

export const buildClusterDomainActionTree = (
  supportedActions: string[] = [],
): ClusterDomainActionTreeNode[] => {
  const roots: ClusterDomainActionTreeNode[] = []

  for (const action of supportedActions) {
    if (typeof action !== 'string') continue

    const segments = splitActionSegments(action)
    if (segments.length === 0) continue

    let branch = roots
    let currentKey = ''

    for (const segment of segments) {
      currentKey = currentKey ? `${currentKey}.${segment}` : segment

      let node = branch.find(entry => entry.key === currentKey)
      if (!node) {
        node = {
          key: currentKey,
          label: segment,
          children: [],
        }
        branch.push(node)
      }

      branch = node.children
    }
  }

  return roots
}

export const flattenVisibleClusterDomainActionTree = (
  nodes: ClusterDomainActionTreeNode[],
  expandedKeys: ReadonlySet<string>,
): ClusterDomainActionTreeRow[] => {
  const rows: ClusterDomainActionTreeRow[] = []

  const visit = (branch: ClusterDomainActionTreeNode[], depth: number) => {
    for (const node of branch) {
      const hasChildren = node.children.length > 0

      rows.push({
        key: node.key,
        label: node.label,
        depth,
        hasChildren,
      })

      if (hasChildren && expandedKeys.has(node.key)) {
        visit(node.children, depth + 1)
      }
    }
  }

  visit(nodes, 0)
  return rows
}
