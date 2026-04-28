import type {
  ActionRequest,
  ActionResponse,
  InfoResponse,
} from '@/types/clusterActions'

export async function fetchNodeInfo(
  baseURL: string,
  token: string
): Promise<InfoResponse> {
  const resp = await fetch(clusterPeerURL(baseURL, '/_cluster/v1/info'), {
    headers: { 'X-Cluster-Token': token },
  })
  if (!resp.ok) throw new Error(`info request failed: ${resp.status}`)
  return resp.json()
}

export async function sendAction(
  baseURL: string,
  token: string,
  req: ActionRequest
): Promise<ActionResponse> {
  const resp = await fetch(clusterPeerURL(baseURL, '/_cluster/v1/action'), {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-Cluster-Token': token,
    },
    body: JSON.stringify(req),
  })
  if (!resp.ok) throw new Error(`action request failed: ${resp.status}`)
  return resp.json()
}

function clusterPeerURL(baseURL: string, path: string): string {
  return `${baseURL.replace(/\/+$/, '')}/${path.replace(/^\/+/, '')}`
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
