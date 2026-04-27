export interface ClusterDomainActionTreeNode {
  key: string
  label: string
  isAction: boolean
  children: ClusterDomainActionTreeNode[]
}

export interface ClusterDomainActionTreeRow {
  key: string
  label: string
  depth: number
  hasChildren: boolean
  isAction: boolean
}

const splitActionSegments = (value: string) => {
  const segments = value.split('.').map(segment => segment.trim())

  return segments.some(segment => segment.length === 0) ? [] : segments
}

export const buildClusterDomainActionTree = (
  supportedActions: unknown = [],
): ClusterDomainActionTreeNode[] => {
  const actions = Array.isArray(supportedActions) ? supportedActions : []
  const roots: ClusterDomainActionTreeNode[] = []

  for (const action of actions) {
    if (typeof action !== 'string') continue

    const segments = splitActionSegments(action)
    if (segments.length === 0) continue

    let branch = roots
    let currentKey = ''

    for (const [index, segment] of segments.entries()) {
      currentKey = currentKey ? `${currentKey}.${segment}` : segment

      let node = branch.find(entry => entry.key === currentKey)
      if (!node) {
        node = {
          key: currentKey,
          label: segment,
          isAction: false,
          children: [],
        }
        branch.push(node)
      }

      if (index === segments.length - 1) {
        node.isAction = true
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
        isAction: node.isAction,
      })

      if (hasChildren && expandedKeys.has(node.key)) {
        visit(node.children, depth + 1)
      }
    }
  }

  visit(nodes, 0)
  return rows
}
