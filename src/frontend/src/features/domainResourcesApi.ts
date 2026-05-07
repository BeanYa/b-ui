import api from '@/plugins/api'

export interface DomainResourceOperationSummary {
  queued: number
  applied: number
  failed: number
  timeout: number
  skipped: number
  total: number
}

export interface DomainResourceOperationView {
  operationId: string
  domainId?: number
  domain?: string
  resourceKind?: string
  resourceId?: string
  action?: string
  revision?: number
  status: string
  summary?: DomainResourceOperationSummary
  instances?: DomainResourceOperationInstanceView[]
}

export interface DomainResourceOperationInstanceView {
  memberId?: string
  nodeId: string
  displayName?: string
  targetTag?: string
  status: string
  attemptCount?: number
  localResourceId?: number
  error?: string
  updatedAt?: number
}

export interface DomainResourceInboundView {
  group_id: string
  type?: string
  status?: string
}

export interface DomainResourceUserView {
  uuid: string
  name: string
  enable: boolean
  sub_token?: string
  bound_inbound_group_ids?: string[]
  inbounds?: Array<string | number>
}

export interface DomainResourcesView {
  domain_inbounds: DomainResourceInboundView[]
  domain_users: DomainResourceUserView[]
}

export interface CreateDomainInboundResourcePayload {
  group_id: string
  tag_seed?: string
  target_members?: {
    member_id?: string
    node_id?: string
    display_name?: string
    target_tag?: string
    remote_inbound_id?: number
    domain_inbound_request_id?: string
  }[]
  prefix?: string
  suffix?: string
  inbound: Record<string, unknown>
  tls_template?: string
  tls?: {
    name: string
    server: Record<string, unknown>
    client: Record<string, unknown>
  }
}

export interface CreateDomainUserResourcePayload {
  user: {
    uuid?: string
    name: string
    enable: boolean
    desc?: string
    group?: string
    config: Record<string, unknown>
    volume?: number
    expiry?: number
    down?: number
    up?: number
    delay_start?: boolean
    auto_reset?: boolean
    reset_days?: number
    next_reset?: number
    total_up?: number
    total_down?: number
    bound_inbound_group_ids?: string[]
  }
  bound_inbound_group_ids?: string[]
  inbounds?: string[]
}

export async function createDomainInboundResource(
  domainId: number,
  payload: CreateDomainInboundResourcePayload,
): Promise<DomainResourceOperationView> {
  const resp = await api.post(
    `api/cluster/domains/${encodeURIComponent(String(domainId))}/resources/inbounds`,
    payload,
    { headers: { 'Content-Type': 'application/json' } },
  )
  return unwrapMsg<DomainResourceOperationView>(resp.data, 'domain inbound resource create failed')
}

export async function listDomainResources(domainId: number): Promise<DomainResourcesView> {
  const resp = await api.get(
    `api/cluster/domains/${encodeURIComponent(String(domainId))}/resources`,
    { headers: { 'Content-Type': 'application/json' } },
  )
  return unwrapMsg<DomainResourcesView>(resp.data, 'domain resources list failed')
}

export async function updateDomainInboundResource(
  domainId: number,
  groupId: string,
  payload: CreateDomainInboundResourcePayload,
): Promise<DomainResourceOperationView> {
  const resp = await api.put(
    `api/cluster/domains/${encodeURIComponent(String(domainId))}/resources/inbounds/${encodeURIComponent(groupId)}`,
    payload,
    { headers: { 'Content-Type': 'application/json' } },
  )
  return unwrapMsg<DomainResourceOperationView>(resp.data, 'domain inbound resource update failed')
}

export async function deleteDomainInboundResource(
  domainId: number,
  groupId: string,
): Promise<DomainResourceOperationView> {
  const resp = await api.delete(
    `api/cluster/domains/${encodeURIComponent(String(domainId))}/resources/inbounds/${encodeURIComponent(groupId)}`,
    { headers: { 'Content-Type': 'application/json' } },
  )
  return unwrapMsg<DomainResourceOperationView>(resp.data, 'domain inbound resource delete failed')
}

export async function createDomainUserResource(
  domainId: number,
  payload: CreateDomainUserResourcePayload,
): Promise<DomainResourceOperationView> {
  const resp = await api.post(
    `api/cluster/domains/${encodeURIComponent(String(domainId))}/resources/users`,
    payload,
    { headers: { 'Content-Type': 'application/json' } },
  )
  return unwrapMsg<DomainResourceOperationView>(resp.data, 'domain user resource create failed')
}

export async function updateDomainUserResource(
  domainId: number,
  userUUID: string,
  payload: CreateDomainUserResourcePayload,
): Promise<DomainResourceOperationView> {
  const resp = await api.put(
    `api/cluster/domains/${encodeURIComponent(String(domainId))}/resources/users/${encodeURIComponent(userUUID)}`,
    payload,
    { headers: { 'Content-Type': 'application/json' } },
  )
  return unwrapMsg<DomainResourceOperationView>(resp.data, 'domain user resource update failed')
}

export async function deleteDomainUserResource(
  domainId: number,
  userUUID: string,
): Promise<DomainResourceOperationView> {
  const resp = await api.delete(
    `api/cluster/domains/${encodeURIComponent(String(domainId))}/resources/users/${encodeURIComponent(userUUID)}`,
    { headers: { 'Content-Type': 'application/json' } },
  )
  return unwrapMsg<DomainResourceOperationView>(resp.data, 'domain user resource delete failed')
}

export async function retryDomainResourceOperation(
  operationId: string,
): Promise<DomainResourceOperationView> {
  const resp = await api.post(
    `api/cluster/domain-operations/${encodeURIComponent(operationId)}/retry`,
    {},
    { headers: { 'Content-Type': 'application/json' } },
  )
  return unwrapMsg<DomainResourceOperationView>(resp.data, 'domain resource operation retry failed')
}

function unwrapMsg<T>(data: unknown, fallback: string): T {
  if (!data || typeof data !== 'object') {
    throw new Error(fallback)
  }
  const msg = data as { success?: boolean; msg?: string; obj?: T | null }
  if (!msg.success) {
    throw new Error(msg.msg || fallback)
  }
  if (msg.obj == null) {
    throw new Error(fallback)
  }
  return msg.obj
}
