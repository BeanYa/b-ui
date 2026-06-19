import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import api from '@/plugins/api'
import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  createDomainInboundResource,
  createDomainUserResource,
  deleteDomainInboundResource,
  deleteDomainUserResource,
  listDomainResources,
  retryDomainResourceOperation,
  updateDomainInboundResource,
  updateDomainUserResource,
} from './domainResourcesApi'

vi.mock('@/plugins/api', () => ({
  default: {
    delete: vi.fn(),
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
  },
}))

describe('domain resources API', () => {
  const source = readFileSync(fileURLToPath(new URL('./domainResourcesApi.ts', import.meta.url)), 'utf8')

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
      include_protocol: true,
      include_security: true,
      include_flag: true,
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
        include_protocol: true,
        include_security: true,
        include_flag: true,
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

  it('creates domain user resources with domain inbound group bindings instead of raw inbound selectors', async () => {
    vi.mocked(api.post).mockResolvedValue({
      data: { success: true, msg: '', obj: { operationId: 'op-user-1', status: 'applied' } },
    })

    await createDomainUserResource(7, {
      user: {
        uuid: 'user-1',
        name: 'Alice',
        enable: true,
        config: { level: 1 },
        bound_inbound_group_ids: ['group-1'],
        links: [
          { type: 'external', uri: 'vless://external@example.com:443#external' },
          { type: 'sub', uri: 'https://sub.example.com/list' },
        ],
        volume: 5368709120,
        expiry: 1770000000,
        delay_start: true,
        auto_reset: true,
        reset_days: 30,
      },
      bound_inbound_group_ids: ['group-1'],
    })

    expect(api.post).toHaveBeenCalledWith(
      'api/cluster/domains/7/resources/users',
      {
        user: {
          uuid: 'user-1',
          name: 'Alice',
          enable: true,
          config: { level: 1 },
          bound_inbound_group_ids: ['group-1'],
          links: [
            { type: 'external', uri: 'vless://external@example.com:443#external' },
            { type: 'sub', uri: 'https://sub.example.com/list' },
          ],
          volume: 5368709120,
          expiry: 1770000000,
          delay_start: true,
          auto_reset: true,
          reset_days: 30,
        },
        bound_inbound_group_ids: ['group-1'],
      },
      {
        headers: { 'Content-Type': 'application/json' },
      },
    )
  })

  it('types domain user payloads with optional group bindings and optional legacy inbounds', () => {
    expect(source).toContain('bound_inbound_group_ids?: string[]')
    expect(source).toContain('inbounds?: string[]')
    expect(source).toContain('links?: Link[]')
    expect(source).toContain('volume?: number')
    expect(source).toContain('delay_start?: boolean')
  })

  it('lists persisted domain resources for existing inbound groups', async () => {
    vi.mocked(api.get).mockResolvedValue({
      data: {
        success: true,
        msg: '',
        obj: {
          domain_inbounds: [{ group_id: 'group-1', type: 'vless', status: 'active' }],
          domain_users: [{ uuid: 'user-1', name: 'Alice', enable: true, bound_inbound_group_ids: ['group-1'] }],
        },
      },
    })

    const resources = await listDomainResources(7)

    expect(resources.domain_inbounds[0]?.group_id).toBe('group-1')
    expect(api.get).toHaveBeenCalledWith(
      'api/cluster/domains/7/resources',
      { headers: { 'Content-Type': 'application/json' } },
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

    await updateDomainInboundResource(7, 'group-1', { group_id: 'ignored', include_protocol: true, include_security: true, include_flag: true, inbound: { tag: 'updated' } })
    await deleteDomainInboundResource(7, 'group-1')
    await updateDomainUserResource(7, 'user-1', {
      user: {
        name: 'Alice Updated',
        enable: true,
        config: { level: 2 },
        bound_inbound_group_ids: ['group-1', 'group-2'],
      },
      bound_inbound_group_ids: ['group-1', 'group-2'],
    })
    await deleteDomainUserResource(7, 'user-1')

    expect(api.put).toHaveBeenCalledWith(
      'api/cluster/domains/7/resources/inbounds/group-1',
      { group_id: 'ignored', include_protocol: true, include_security: true, include_flag: true, inbound: { tag: 'updated' } },
      { headers: { 'Content-Type': 'application/json' } },
    )
    expect(api.delete).toHaveBeenCalledWith(
      'api/cluster/domains/7/resources/inbounds/group-1',
      { headers: { 'Content-Type': 'application/json' } },
    )
    expect(api.put).toHaveBeenCalledWith(
      'api/cluster/domains/7/resources/users/user-1',
      {
        user: {
          name: 'Alice Updated',
          enable: true,
          config: { level: 2 },
          bound_inbound_group_ids: ['group-1', 'group-2'],
        },
        bound_inbound_group_ids: ['group-1', 'group-2'],
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
