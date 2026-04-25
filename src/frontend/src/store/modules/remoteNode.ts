import { defineStore } from 'pinia'
import { ref, reactive } from 'vue'
import { fetchNodeInfo, sendAction, buildListActionPayload } from '@/features/clusterPeerApi'
import type { InfoResponse, PaginationResponse } from '@/types/clusterActions'

interface TabData<T> {
  items: T[]
  total: number
  page: number
  loaded: boolean
  loading: boolean
}

export const useRemoteNodeStore = defineStore('RemoteNode', () => {
  const baseURL = ref('')
  const token = ref('')
  const info = ref<InfoResponse | null>(null)

  const pageLoading = ref(true)
  const pageError = ref<string | null>(null)

  const tlsConfigs = reactive<TabData<any>>({ items: [], total: 0, page: 1, loaded: false, loading: false })
  const clients = reactive<TabData<any>>({ items: [], total: 0, page: 1, loaded: false, loading: false })
  const inbounds = reactive<TabData<any>>({ items: [], total: 0, page: 1, loaded: false, loading: false })
  const services = reactive<TabData<any>>({ items: [], total: 0, page: 1, loaded: false, loading: false })
  const routes = reactive<TabData<any>>({ items: [], total: 0, page: 1, loaded: false, loading: false })
  const outbounds = reactive<TabData<any>>({ items: [], total: 0, page: 1, loaded: false, loading: false })

  async function init(url: string, t: string) {
    baseURL.value = url
    token.value = t
    pageLoading.value = true
    pageError.value = null

    try {
      info.value = await fetchNodeInfo(url, t)
      await Promise.all([
        fetchTab(tlsConfigs, 'tls.list'),
        fetchTab(clients, 'client.list'),
      ])
    } catch (e: any) {
      pageError.value = e.message
    } finally {
      pageLoading.value = false
    }
  }

  async function fetchTab<T>(tab: TabData<T>, action: string, page?: number) {
    tab.loading = true
    try {
      const p = page ?? tab.page ?? 1
      const req = buildListActionPayload(action, p)
      const resp = await sendAction(baseURL.value, token.value, req)
      if (resp.status === 'success' && resp.data) {
        const data = resp.data as PaginationResponse<T>
        tab.items = data.items
        tab.total = data.total
        tab.page = data.page
        tab.loaded = true
      }
    } finally {
      tab.loading = false
    }
  }

  function reset() {
    info.value = null
    tlsConfigs.loaded = false
    clients.loaded = false
    inbounds.loaded = false
    services.loaded = false
    routes.loaded = false
    outbounds.loaded = false
  }

  return {
    baseURL, token, info, pageLoading, pageError,
    tlsConfigs, clients, inbounds, services, routes, outbounds,
    init, fetchTab, reset,
  }
})
