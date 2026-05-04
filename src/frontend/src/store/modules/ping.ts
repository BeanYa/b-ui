import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import api from '@/plugins/api'
import type {
  MeshResult,
  MeshPairResult,
  ExternalConfig,
  ExternalResultData,
  ExternalRunRequest,
  ExternalSource,
  PingPolicy,
} from '@/types/ping'
import { DEFAULT_PING_POLICY } from '@/types/ping'

type MeshStreamEvent =
  | { type: 'result'; result: MeshPairResult }
  | { type: 'done'; result: MeshResult }
  | { type: 'error'; msg?: string }

export const usePingStore = defineStore('PingStore', () => {
  const meshResult = ref<MeshResult | null>(null)
  const externalConfig = ref<ExternalConfig | null>(null)
  const externalResults = ref<ExternalResultData | null>(null)
  const pingPolicy = ref<PingPolicy>({ ...DEFAULT_PING_POLICY })
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function triggerMeshPing(domainId: string): Promise<MeshResult> {
    loading.value = true
    error.value = null
    try {
      const { data } = await api.post(`api/ping/mesh/${encodeURIComponent(domainId)}`)
      if (data.success) {
        meshResult.value = data.obj
        return data.obj
      }
      throw new Error(data.msg)
    } catch (e: any) {
      error.value = e.message
      throw e
    } finally {
      loading.value = false
    }
  }

  function upsertMeshPairResult(results: MeshPairResult[], result: MeshPairResult): MeshPairResult[] {
    const index = results.findIndex(r =>
      r.source_member_id === result.source_member_id && r.target_member_id === result.target_member_id
    )
    if (index === -1) return [...results, result]
    const next = [...results]
    next[index] = result
    return next
  }

  async function triggerMeshPingStream(
    domainId: string,
    onResult?: (result: MeshPairResult) => void,
  ): Promise<MeshResult> {
    loading.value = true
    error.value = null
    meshResult.value = { domain_id: domainId, tested_at: 0, results: [] }
    try {
      const baseURL = (globalThis as any).window?.BASE_URL ?? '/'
      const response = await fetch(`${baseURL}api/ping/mesh/${encodeURIComponent(domainId)}/stream`, {
        method: 'POST',
      })
      if (!response.ok) {
        throw new Error(`mesh ping stream failed: ${response.status}`)
      }
      if (!response.body) {
        throw new Error('mesh ping stream is not readable')
      }

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''
      let finalResult: MeshResult | null = null

      const handleLine = (line: string) => {
        if (!line.trim()) return
        const event = JSON.parse(line) as MeshStreamEvent
        if (event.type === 'result') {
          meshResult.value = {
            domain_id: domainId,
            tested_at: meshResult.value?.tested_at ?? 0,
            results: upsertMeshPairResult(meshResult.value?.results ?? [], event.result),
          }
          onResult?.(event.result)
          return
        }
        if (event.type === 'done') {
          finalResult = event.result
          meshResult.value = event.result
          return
        }
        throw new Error(event.msg || 'mesh ping stream failed')
      }

      while (true) {
        const { value, done } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop() ?? ''
        for (const line of lines) {
          handleLine(line)
        }
      }
      buffer += decoder.decode()
      handleLine(buffer)

      if (!finalResult) {
        throw new Error('mesh ping stream ended before final result')
      }
      return finalResult
    } catch (e: any) {
      error.value = e.message
      throw e
    } finally {
      loading.value = false
    }
  }

  async function loadMeshResult(domainId: string): Promise<MeshResult | null> {
    error.value = null
    try {
      const { data } = await api.get(`api/ping/mesh/${encodeURIComponent(domainId)}`)
      if (data.success) {
        meshResult.value = data.obj
        return data.obj
      }
      return null
    } catch (e: any) {
      if (e.response?.status === 404) {
        meshResult.value = null
        return null
      }
      error.value = e.message
      return null
    }
  }

  async function triggerExternalPing(request: ExternalRunRequest): Promise<ExternalResultData> {
    loading.value = true
    error.value = null
    try {
      const { data } = await api.post('api/ping/external', request, {
        headers: { 'Content-Type': 'application/json' },
      })
      if (data.success) {
        externalResults.value = data.obj
        return data.obj
      }
      throw new Error(data.msg)
    } catch (e: any) {
      error.value = e.message
      throw e
    } finally {
      loading.value = false
    }
  }

  async function loadExternalResults(): Promise<ExternalResultData | null> {
    error.value = null
    try {
      const { data } = await api.get('api/ping/external/results')
      if (data.success) {
        externalResults.value = data.obj
        return data.obj
      }
      return null
    } catch {
      externalResults.value = null
      return null
    }
  }

  async function loadExternalConfig(): Promise<ExternalConfig> {
    error.value = null
    try {
      const { data } = await api.get('api/ping/external/config')
      if (data.success) {
        externalConfig.value = data.obj
        return data.obj
      }
      throw new Error(data.msg)
    } catch (e: any) {
      error.value = e.message
      throw e
    }
  }

  async function saveExternalConfig(config: ExternalConfig): Promise<void> {
    loading.value = true
    error.value = null
    try {
      const { data } = await api.put('api/ping/external/config', config)
      if (!data.success) throw new Error(data.msg)
      externalConfig.value = config
    } catch (e: any) {
      error.value = e.message
      throw e
    } finally {
      loading.value = false
    }
  }

  const inboundSources = computed(() =>
    externalConfig.value?.sources.filter(s => s.direction === 'inbound') ?? []
  )

  const outboundSources = computed(() =>
    externalConfig.value?.sources.filter(s => s.direction === 'outbound') ?? []
  )

  async function loadPingPolicy(domainId: string): Promise<PingPolicy> {
    error.value = null
    try {
      const { data } = await api.get(`api/ping/policy/${encodeURIComponent(domainId)}`)
      if (data.success) {
        pingPolicy.value = data.obj
        return data.obj
      }
      pingPolicy.value = { ...DEFAULT_PING_POLICY }
      return pingPolicy.value
    } catch (e: any) {
      pingPolicy.value = { ...DEFAULT_PING_POLICY }
      error.value = e.message
      return pingPolicy.value
    }
  }

  async function savePingPolicy(domainId: string, policy: PingPolicy): Promise<void> {
    loading.value = true
    error.value = null
    try {
      const { data } = await api.put(`api/ping/policy/${encodeURIComponent(domainId)}`, policy)
      if (!data.success) throw new Error(data.msg)
      pingPolicy.value = policy
    } catch (e: any) {
      error.value = e.message
      throw e
    } finally {
      loading.value = false
    }
  }

  return {
    meshResult, externalConfig, externalResults, loading, error,
    pingPolicy, loadPingPolicy, savePingPolicy,
    inboundSources, outboundSources,
    triggerMeshPing, triggerMeshPingStream, loadMeshResult,
    triggerExternalPing, loadExternalResults,
    loadExternalConfig, saveExternalConfig,
  }
})
