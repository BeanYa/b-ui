import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

describe('ClusterNodeDetail view source', () => {
  it('can be imported without errors', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterNodeDetail.vue', import.meta.url)), 'utf8')
    expect(source).toBeTruthy()
    expect(source).toContain('<template>')
    expect(source).toContain('<script lang="ts" setup>')
  })

  it('loads connection details by id query and initializes node management through the server proxy', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterNodeDetail.vue', import.meta.url)), 'utf8')

    expect(source).toContain("route.query.id")
    expect(source).not.toContain("route.query.node_id || route.params.nodeId")
    expect(source).not.toContain("route.params.nodeId")
    expect(source).toContain('api/cluster/member-connection?node_id=')
    expect(source).toContain('await remoteNode.init(nodeConnection.value.nodeId, nodeConnection.value.baseUrl)')
    expect(source).not.toContain('nodeConnection.value.token')
    expect(source).not.toContain('route.query.baseUrl')
    expect(source).not.toContain('route.query.token')
  })

  it('enters remote Data mode and renders the full local panel when panel actions are supported', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterNodeDetail.vue', import.meta.url)), 'utf8')

    expect(source).toContain("nodeActions.value.includes('panel.load')")
    expect(source).toContain("nodeActions.value.includes('panel.save')")
    expect(source).toContain('panelReady.value')
    expect(source).toContain('Data().enterRemoteNode(nodeConnection.value.nodeId, nodeConnection.value.baseUrl)')
    expect(source).toContain('await Data().loadData()')
    expect(source).toContain('Data().exitRemoteNode()')

    expect(source).toContain("import InboundsView from '@/views/Inbounds.vue'")
    expect(source).toContain("import ClientsView from '@/views/Clients.vue'")
    expect(source).toContain("import TlsView from '@/views/Tls.vue'")
    expect(source).toContain("import ServicesView from '@/views/Services.vue'")
    expect(source).toContain("import RulesView from '@/views/Rules.vue'")
    expect(source).toContain("import OutboundsView from '@/views/Outbounds.vue'")
    expect(source).toContain("import EndpointsView from '@/views/Endpoints.vue'")

    expect(source).toContain('<InboundsView />')
    expect(source).toContain('<ClientsView />')
    expect(source).toContain('<TlsView />')
    expect(source).toContain('<ServicesView />')
    expect(source).toContain('<RulesView />')
    expect(source).toContain('<OutboundsView />')
    expect(source).toContain('<EndpointsView />')
  })

  it('probes panel.load when member-info returns no advertised actions', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterNodeDetail.vue', import.meta.url)), 'utf8')

    expect(source).toContain('async function tryEnterRemotePanel()')
    expect(source).toContain('nodeActions.value.length === 0')
    expect(source).toContain('await tryEnterRemotePanel()')
    expect(source).toContain('panelReady.value = true')
  })
})
