import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

describe('ClusterScatterTaskResult view source', () => {
  const readSource = () =>
    readFileSync(fileURLToPath(new URL('./ClusterScatterTaskResult.vue', import.meta.url)), 'utf8')

  it('renders latency matrix headers from node display names with node ids as fallback context', () => {
    const source = readSource()

    expect(source).toContain('displayNameByNodeId')
    expect(source).toContain('resolveNodeDisplayName')
    expect(source).toContain('node.displayName')
    expect(source).toContain('node.nodeId')
    expect(source).toContain('node.nodeIdShort')
    expect(source).toContain('avgMs: reachable > 0 ? value.avg_ms ?? value.avgMs ?? null : null')
  })

  it('keeps the latency matrix compact with fixed columns and alternating cell fills', () => {
    const source = readSource()

    expect(source).toContain('table-layout: fixed;')
    expect(source).toContain('width: max-content;')
    expect(source).toContain('min-width: 108px;')
    expect(source).toContain('.scatter-result__matrix-table tbody tr:nth-child(odd) td:nth-child(even)')
    expect(source).toContain('.scatter-result__matrix-table tbody tr:nth-child(even) td:nth-child(odd)')
  })
})
