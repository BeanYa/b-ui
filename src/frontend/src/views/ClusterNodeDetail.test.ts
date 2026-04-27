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

  it('loads connection details by node_id instead of trusting URL token query', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterNodeDetail.vue', import.meta.url)), 'utf8')

    expect(source).toContain("route.query.node_id || route.params.nodeId")
    expect(source).toContain('api/cluster/member-connection?node_id=')
    expect(source).toContain('await remoteNode.init(nodeConnection.value.baseUrl, nodeConnection.value.token)')
    expect(source).not.toContain('route.query.baseUrl')
    expect(source).not.toContain('route.query.token')
  })
})
