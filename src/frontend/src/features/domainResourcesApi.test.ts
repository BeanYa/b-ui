import api from '@/plugins/api'
import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  createDomainInboundResource,
  createDomainUserResource,
  deleteDomainInboundResource,
  deleteDomainUserResource,
  retryDomainResourceOperation,
  updateDomainInboundResource,
  updateDomainUserResource,
} from './domainResourcesApi'

vi.mock('@/plugins/api', () => ({
  default: {
    delete: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
  },
}))

describe('domain resources API', () => {
  afterEach(() => {
    vi.clearAllMocks()
  })

  it('creates domain inbound resources through the local cluster endpoint with JSON payload shape', async () => {
    vi.mocked(api.post).mockResolvedValue({
      data: {
        success: true,
        msg: '',
        obj: {
          operationId: 'op-1',
          status: 'failed',
          instances: [
            {
              nodeId: 'local-node',
              displayName: 'local-node',
              status: 'failed',
              error: 'inbound type is required',
            },
          ],
        },
      },
    })

    const operation = await createDomainInboundResource(7, {
      group_id: 'group-1',
      tag_seed: 'edge-main',
      prefix: 'edge',
      suffix: 'prod',
      inbound: {
        tag: 'main',
        type: 'vless',
        listen_port: { LocalProvided: 'DomainInboundListenPort' },
      },
      tls_template: 'standard',
      tls: {
        name: 'edge-main-tls',
        server: {
          enabled: true,
          server_name: { LocalProvided: 'DomainName' },
          certificate: { LocalProvided: 'GeneratedTLSCertificate' },
          key: { LocalProvided: 'GeneratedTLSKey' },
        },
        client: { insecure: true },
      },
    })

    expect(operation.instances?.[0]?.error).toBe('inbound type is required')
    expect(api.post).toHaveBeenCalledWith(
      'api/cluster/domains/7/resources/inbounds',
      {
        group_id: 'group-1',
        tag_seed: 'edge-main',
        prefix: 'edge',
        suffix: 'prod',
        inbound: {
          tag: 'main',
          type: 'vless',
          listen_port: { LocalProvided: 'DomainInboundListenPort' },
        },
        tls_template: 'standard',
        tls: {
          name: 'edge-main-tls',
          server: {
            enabled: true,
            server_name: { LocalProvided: 'DomainName' },
            certificate: { LocalProvided: 'GeneratedTLSCertificate' },
            key: { LocalProvided: 'GeneratedTLSKey' },
          },
          client: { insecure: true },
        },
      },
      {
        headers: { 'Content-Type': 'application/json' },
      },
    )
  })

  it('creates domain user resources through the local cluster endpoint with JSON payload shape', async () => {
    vi.mocked(api.post).mockResolvedValue({
      data: { success: true, msg: '', obj: { operationId: 'op-user-1', status: 'applied' } },
    })

    await createDomainUserResource(7, {
      user: {
        uuid: 'user-1',
        name: 'Alice',
        enable: true,
        config: { level: 1 },
      },
      inbounds: ['group-1'],
    })

    expect(api.post).toHaveBeenCalledWith(
      'api/cluster/domains/7/resources/users',
      {
        user: {
          uuid: 'user-1',
          name: 'Alice',
          enable: true,
          config: { level: 1 },
        },
        inbounds: ['group-1'],
      },
      {
        headers: { 'Content-Type': 'application/json' },
      },
    )
  })

  it('updates and deletes domain resources through the local cluster endpoints', async () => {
    vi.mocked(api.post).mockResolvedValue({
      data: { success: true, msg: '', obj: { operationId: 'op-1', status: 'applied' } },
    })
    vi.mocked(api.put).mockResolvedValue({
      data: { success: true, msg: '', obj: { operationId: 'op-2', status: 'applied' } },
    })
    vi.mocked(api.delete).mockResolvedValue({
      data: { success: true, msg: '', obj: { operationId: 'op-3', status: 'applied' } },
    })

    await updateDomainInboundResource(7, 'group-1', { group_id: 'ignored', inbound: { tag: 'updated' } })
    await deleteDomainInboundResource(7, 'group-1')
    await updateDomainUserResource(7, 'user-1', {
      user: { name: 'Alice Updated', enable: true, config: { level: 2 } },
      inbounds: ['group-1'],
    })
    await deleteDomainUserResource(7, 'user-1')

    expect(api.put).toHaveBeenCalledWith(
      'api/cluster/domains/7/resources/inbounds/group-1',
      { group_id: 'ignored', inbound: { tag: 'updated' } },
      { headers: { 'Content-Type': 'application/json' } },
    )
    expect(api.delete).toHaveBeenCalledWith(
      'api/cluster/domains/7/resources/inbounds/group-1',
      { headers: { 'Content-Type': 'application/json' } },
    )
    expect(api.put).toHaveBeenCalledWith(
      'api/cluster/domains/7/resources/users/user-1',
      {
        user: { name: 'Alice Updated', enable: true, config: { level: 2 } },
        inbounds: ['group-1'],
      },
      { headers: { 'Content-Type': 'application/json' } },
    )
    expect(api.delete).toHaveBeenCalledWith(
      'api/cluster/domains/7/resources/users/user-1',
      { headers: { 'Content-Type': 'application/json' } },
    )
  })

  it('retries failed domain resource operations through the local cluster endpoint', async () => {
    vi.mocked(api.post).mockResolvedValue({
      data: { success: true, msg: '', obj: { operationId: 'op-1', status: 'applied' } },
    })

    await retryDomainResourceOperation('op-1')

    expect(api.post).toHaveBeenCalledWith(
      'api/cluster/domain-operations/op-1/retry',
      {},
      {
        headers: { 'Content-Type': 'application/json' },
      },
    )
  })
})
