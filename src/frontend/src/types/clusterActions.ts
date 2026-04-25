export interface ActionRequest {
  schema_version: number
  sourceNodeId: string
  domain: string
  sentAt: number
  signature: string
  action: string
  payload: Record<string, unknown>
}

export interface ActionResponse {
  status: 'success' | 'unsupported' | 'error'
  action: string
  error_message?: string
  data?: unknown
}

export interface InfoResponse {
  actions: string[]
}

export interface PaginationResponse<T> {
  items: T[]
  total: number
  page: number
  page_size: number
}

export interface ProxyCreatePayload {
  request_id: string
  tls?: Record<string, unknown>
  inbound: Record<string, unknown>
  users?: Record<string, unknown>[]
  expiry?: string | null
}

export interface ProxyCreateResponse {
  inbound_id: number
  uris: string[]
  expiry: string | null
}
