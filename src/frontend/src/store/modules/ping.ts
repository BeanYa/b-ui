import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import axios from 'axios'
import type {
  MeshResult,
  ExternalConfig,
  ExternalResultData,
  ExternalRunRequest,
  ExternalSource,
} from '@/types/ping'

export const usePingStore = defineStore('PingStore', () => {
  const meshResult = ref<MeshResult | null>(null)
  const externalConfig = ref<ExternalConfig | null>(null)
  const externalResults = ref<ExternalResultData | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function triggerMeshPing(domainId: string): Promise<MeshResult> {
    loading.value = true
    error.value = null
    try {
      const { data } = await axios.post(`/api/ping/mesh/${encodeURIComponent(domainId)}`)
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

  async function loadMeshResult(domainId: string): Promise<MeshResult | null> {
    error.value = null
    try {
      const { data } = await axios.get(`/api/ping/mesh/${encodeURIComponent(domainId)}`)
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

  async function triggerExternalPing(sourceIds: string[]): Promise<ExternalResultData> {
    loading.value = true
    error.value = null
    try {
      const req: ExternalRunRequest = { source_ids: sourceIds }
      const { data } = await axios.post('/api/ping/external', req)
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
      const { data } = await axios.get('/api/ping/external/results')
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
      const { data } = await axios.get('/api/ping/external/config')
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
      const { data } = await axios.put('/api/ping/external/config', config)
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

  return {
    meshResult, externalConfig, externalResults, loading, error,
    inboundSources, outboundSources,
    triggerMeshPing, loadMeshResult,
    triggerExternalPing, loadExternalResults,
    loadExternalConfig, saveExternalConfig,
  }
})
