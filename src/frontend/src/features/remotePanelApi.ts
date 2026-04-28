import { sendAction } from '@/features/clusterPeerApi'
import type { ActionRequest, ActionResponse } from '@/types/clusterActions'

export interface RemotePanelLoadPayload {
  lu?: number | string
  hostname?: string
}

export interface RemotePanelPartialPayload {
  object: string
  id?: string | number
  hostname?: string
}

export interface RemotePanelSavePayload {
  object: string
  action: string
  data: unknown
  initUsers?: number[] | string
  hostname?: string
}

export interface RemotePanelKeypairPayload {
  k: string
  o?: string
}

export async function remotePanelLoad(nodeId: string, payload: RemotePanelLoadPayload): Promise<any> {
  return sendPanelAction(nodeId, 'panel.load', compactPayload(payload))
}

export async function remotePanelPartial(nodeId: string, payload: RemotePanelPartialPayload): Promise<any> {
  return sendPanelAction(nodeId, 'panel.partial', compactPayload(payload))
}

export async function remotePanelSave(nodeId: string, payload: RemotePanelSavePayload): Promise<any> {
  return sendPanelAction(nodeId, 'panel.save', compactPayload(payload))
}

export async function remotePanelKeypairs(nodeId: string, payload: RemotePanelKeypairPayload): Promise<string[]> {
  return sendPanelAction(nodeId, 'panel.keypairs', compactPayload(payload))
}

export async function remotePanelLinkConvert(nodeId: string, payload: { link: string }): Promise<any> {
  return sendPanelAction(nodeId, 'panel.linkConvert', compactPayload(payload))
}

export async function remotePanelCheckOutbound(nodeId: string, payload: { tag: string; link?: string }): Promise<any> {
  return sendPanelAction(nodeId, 'panel.checkOutbound', compactPayload(payload))
}

export async function remotePanelStats(nodeId: string, payload: { resource: string; tag: string; limit: number }): Promise<any> {
  return sendPanelAction(nodeId, 'panel.stats', compactPayload(payload))
}

async function sendPanelAction(nodeId: string, action: string, payload: Record<string, unknown>): Promise<any> {
  const response = await sendAction(nodeId, buildPanelActionRequest(action, payload))
  return unwrapPanelAction(response)
}

function buildPanelActionRequest(action: string, payload: Record<string, unknown>): ActionRequest {
  return {
    schema_version: 1,
    sourceNodeId: '',
    domain: '',
    sentAt: Math.floor(Date.now() / 1000),
    signature: '',
    action,
    payload,
  }
}

function unwrapPanelAction(response: ActionResponse): any {
  if (response.status === 'success') {
    return response.data
  }
  if (response.status === 'unsupported') {
    throw new Error(`${response.action || 'panel action'} is unsupported by the remote node`)
  }
  throw new Error(response.error_message || `${response.action || 'panel action'} failed`)
}

function compactPayload<T extends object>(payload: T): Record<string, unknown> {
  return Object.fromEntries(
    Object.entries(payload).filter(([, value]) => value !== undefined),
  )
}
