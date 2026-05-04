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

export interface ExternalEndpoint {
  id: string
  label: string
  provider: string
  region?: string
  country?: string
  city?: string
  network?: string
  host?: string
  port?: number
  methods?: string[]
}

export interface ExternalTargetRequest {
  host: string
  port?: number
  label?: string
}

export interface ExternalTestResult {
  source_member_id: string
  source_label: string
  direction: 'inbound' | 'outbound'
  target_member_id: string
  target_name: string
  source: ExternalEndpoint
  target: ExternalEndpoint
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
  direction?: 'inbound' | 'outbound'
  source_ids: string[]
  target_node_ids?: string[]
  target?: ExternalTargetRequest
  methods?: string[]
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

export function sortedExternalByLatency(results: ExternalTestResult[]): ExternalTestResult[] {
  return [...results]
    .filter(r => r.success && r.latency_ms !== null)
    .sort((a, b) => (a.latency_ms ?? Infinity) - (b.latency_ms ?? Infinity))
}

export function latencyCSSColor(ms: number | null, success: boolean): string {
  const name = latencyColor(ms, success)
  switch (name) {
    case 'green': return '#4caf50'
    case 'yellow': return '#f9a825'
    case 'orange': return '#ff9800'
    case 'red': return '#f44336'
    default: return '#9e9e9e'
  }
}

export interface PingPolicy {
  enabled: boolean
  interval: number       // seconds
  timeout: number        // seconds
  alert_threshold: number // ms, 0 = disabled
  probe_methods: string[]
  max_concurrent: number
}

export const DEFAULT_PING_POLICY: PingPolicy = {
  enabled: false,
  interval: 60,
  timeout: 2,
  alert_threshold: 300,
  probe_methods: ['icmp', 'tcp', 'http'],
  max_concurrent: 5,
}

export const PING_INTERVAL_OPTIONS = [
  { label: '30s', value: 30 },
  { label: '1min', value: 60 },
  { label: '2min', value: 120 },
  { label: '5min', value: 300 },
  { label: '10min', value: 600 },
]
