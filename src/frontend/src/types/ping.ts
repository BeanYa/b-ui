export interface MeshPairResult {
  source_member_id: string
  source_name: string
  target_member_id: string
  target_name: string
  method: string | null
  latency_ms: number | null
  success: boolean
  error: string | null
}

export interface MeshResult {
  domain_id: string
  tested_at: number
  results: MeshPairResult[]
}

export interface ExternalSource {
  id: string
  name: string
  type: string
  direction: 'inbound' | 'outbound'
  enabled: boolean
  api_key: string
  worker_url?: string
}

export interface ExternalConfig {
  sources: ExternalSource[]
}

export interface ExternalTestResult {
  source_label: string
  direction: 'inbound' | 'outbound'
  target_member_id: string
  target_name: string
  method: string | null
  latency_ms: number | null
  success: boolean
  error: string | null
}

export interface ExternalResultData {
  tested_at: number
  results: ExternalTestResult[]
}

export interface ExternalRunRequest {
  source_ids: string[]
}

export function latencyColor(ms: number | null, success: boolean): string {
  if (!success) return 'error'
  if (ms === null) return 'unknown'
  if (ms < 50) return 'green'
  if (ms < 150) return 'yellow'
  if (ms < 300) return 'orange'
  return 'red'
}

export function latencyText(r: MeshPairResult | ExternalTestResult): string {
  if (!r.success) return 'ERROR'
  if (r.latency_ms === null) return '-'
  return `${r.latency_ms.toFixed(0)}ms`
}

export function sortedByLatency(results: MeshPairResult[]): MeshPairResult[] {
  return [...results]
    .filter(r => r.success && r.latency_ms !== null)
    .sort((a, b) => (a.latency_ms ?? Infinity) - (b.latency_ms ?? Infinity))
}
