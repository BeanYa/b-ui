import type {
  ActionRequest,
  ActionResponse,
  InfoResponse,
} from '@/types/clusterActions'
import api from '@/plugins/api'

export async function fetchNodeInfo(
  nodeId: string
): Promise<InfoResponse> {
  const resp = await api.get('api/cluster/member-info', {
    params: { node_id: nodeId },
  })
  return unwrapMsg<InfoResponse>(resp.data, 'info request failed')
}

export async function sendAction(
  nodeId: string,
  req: ActionRequest
): Promise<ActionResponse> {
  const resp = await api.post('api/cluster/member-action', {
    node_id: nodeId,
    request: req,
  })
  return unwrapMsg<ActionResponse>(resp.data, 'action request failed')
}

export function buildListActionPayload(
  action: string,
  page: number,
  pageSize: number = 10
): ActionRequest {
  return {
    schema_version: 1,
    sourceNodeId: '',
    domain: '',
    sentAt: Math.floor(Date.now() / 1000),
    signature: '',
    action,
    payload: { page, page_size: pageSize },
  }
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
