import { defineStore } from 'pinia'
import { ref } from 'vue'
import HttpUtils from '@/plugins/httputil'
import type { ScatterTaskSummary, ScatterTaskResultDetail } from '@/types/clusters'

export const useClusterStore = defineStore('ClusterStore', () => {
  const tasks = ref<ScatterTaskSummary[]>([])
  const tasksLoading = ref(false)
  const pollTimer = ref<ReturnType<typeof setInterval> | null>(null)

  async function fetchScatterTasks(domainId: string): Promise<ScatterTaskSummary[]> {
    tasksLoading.value = true
    try {
      const msg = await HttpUtils.get(`_cluster/v1/domains/${encodeURIComponent(domainId)}/tasks`)
      if (msg.success && Array.isArray(msg.obj)) {
        tasks.value = msg.obj
        return msg.obj
      }
      return []
    } finally {
      tasksLoading.value = false
    }
  }

  async function createScatterTask(
    domainId: string,
    taskType: string,
    scope: string,
    params: Record<string, any>,
  ): Promise<ScatterTaskSummary | null> {
    const msg = await HttpUtils.post(`_cluster/v1/domains/${encodeURIComponent(domainId)}/tasks`, {
      taskType,
      scope,
      params,
    })
    if (msg.success && msg.obj) {
      return msg.obj as ScatterTaskSummary
    }
    return null
  }

  async function fetchScatterTaskResult(
    domainId: string,
    taskId: string,
  ): Promise<ScatterTaskResultDetail | null> {
    const msg = await HttpUtils.get(
      `_cluster/v1/domains/${encodeURIComponent(domainId)}/tasks/${encodeURIComponent(taskId)}/result`,
    )
    if (msg.success && msg.obj) {
      return msg.obj as ScatterTaskResultDetail
    }
    return null
  }

  function hasActiveTasks(): boolean {
    return tasks.value.some(t => t.status !== 'completed' && t.status !== 'failed' && t.status !== 'timeout')
  }

  function startPolling(domainId: string, intervalMs: number = 5000) {
    stopPolling()
    pollTimer.value = setInterval(() => {
      fetchScatterTasks(domainId)
    }, intervalMs)
  }

  function stopPolling() {
    if (pollTimer.value) {
      clearInterval(pollTimer.value)
      pollTimer.value = null
    }
  }

  return {
    tasks,
    tasksLoading,
    fetchScatterTasks,
    createScatterTask,
    fetchScatterTaskResult,
    hasActiveTasks,
    startPolling,
    stopPolling,
  }
})
