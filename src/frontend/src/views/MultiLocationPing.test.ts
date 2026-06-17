import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

describe('MultiLocationPing view source', () => {
  const readSource = () =>
    readFileSync(fileURLToPath(new URL('./MultiLocationPing.vue', import.meta.url)), 'utf8')

  it('sends inbound target host and port in the run request', () => {
    const source = readSource()

    expect(source).toContain('inboundTargetHost')
    expect(source).toContain('inboundTargetPort')
    expect(source).toContain("direction: 'inbound'")
    expect(source).toContain('target: {')
  })

  it('sends outbound current-node requests without target node ids', () => {
    const source = readSource()

    expect(source).toContain("direction: 'outbound'")
    expect(source).not.toContain('target_node_ids')
    expect(source).not.toContain('targetNodeIds')
  })

  it('renders endpoint metadata columns for external results', () => {
    const source = readSource()

    expect(source).toContain('endpointLabel')
    expect(source).toContain('endpointLocation')
    expect(source).toContain('endpointAddressText')
  })

  it('normalizes inbound target host and accepts only integer ports in range', () => {
    const source = readSource()

    expect(source).toContain('normalizeInboundTargetHost')
    expect(source).toContain('normalizedInboundTargetPort')
    expect(source).toContain('Number.isInteger')
    expect(source).toContain('65535')
  })

  it('combines category tabs with the active data-source panel', () => {
    const source = readSource()
    const workspaceStart = source.indexOf('class="app-card-shell multi-location-ping__workspace"')
    const tabsStart = source.indexOf('class="multi-location-ping__tabs"')
    const inboundSourceStart = source.indexOf('Inbound Data Sources (External → Cluster)')

    expect(source).not.toContain('grow class="mb-4"')
    expect(workspaceStart).toBeGreaterThan(-1)
    expect(tabsStart).toBeGreaterThan(workspaceStart)
    expect(inboundSourceStart).toBeGreaterThan(tabsStart)
    expect(source).toContain('.multi-location-ping__tabs {')
    expect(source).toContain('height: 48px;')
    expect(source).toContain('.multi-location-ping__source-pane {')
  })
})
