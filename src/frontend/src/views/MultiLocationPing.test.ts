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

  it('sends selected outbound target node ids', () => {
    const source = readSource()

    expect(source).toContain('selectedOutboundTargetIds')
    expect(source).toContain('target_node_ids: selectedOutboundTargetIds.value')
  })

  it('loads and refreshes the outbound target catalog', () => {
    const source = readSource()

    expect(source).toContain('loadExternalTargetCatalog')
    expect(source).toContain('refreshExternalTargetCatalog')
    expect(source).toContain('refreshOutboundTargets')
  })

  it('groups outbound targets by provider and target group', () => {
    const source = readSource()

    expect(source).toContain('outboundProviderGroups')
    expect(source).toContain('targetGroups')
    expect(source).toContain('toggleProviderTargets')
    expect(source).toContain('toggleTargetGroup')
  })

  it('does not run disabled outbound providers', () => {
    const source = readSource()

    expect(source).toContain('isOutboundProviderEnabled')
    expect(source).toContain('.filter(provider => provider.enabled')
    expect(source).toContain(':disabled="!provider.enabled"')
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
