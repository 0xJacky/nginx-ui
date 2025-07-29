import type { EnvGroup } from '@/api/env_group'
import { defineStore } from 'pinia'
import env_group from '@/api/env_group'

export const useNodeGroupStore = defineStore('nodeGroup', () => {
  const envGroups = ref<EnvGroup[]>([])
  const envGroupMap = ref<Record<number, EnvGroup>>({})
  const isLoading = ref(false)
  const isInitialized = ref(false)
  const lastUpdateTime = ref<string>('')

  // Initialize the store with data
  async function initialize() {
    if (isInitialized.value) {
      return
    }

    await loadAll()
    isInitialized.value = true
  }

  // Load all environment groups by cycling through pages
  async function loadAll() {
    if (isLoading.value) {
      return
    }

    isLoading.value = true

    try {
      const allGroups: EnvGroup[] = []
      let currentPage = 1
      let hasMorePages = true

      while (hasMorePages) {
        const response = await env_group.getList({
          page: currentPage,
          page_size: 100, // Use a reasonable page size
        })

        const pageData = response.data || []
        allGroups.push(...pageData)

        // Check if there are more pages
        if (response.pagination) {
          hasMorePages = currentPage < response.pagination.total_pages
        }
        else {
          // Fallback: if no pagination info, check if we got a full page
          hasMorePages = pageData.length === 100
        }

        currentPage++
      }

      envGroups.value = allGroups
      lastUpdateTime.value = new Date().toISOString()
      envGroupMap.value = allGroups.reduce((acc, group) => {
        acc[group.id] = group
        return acc
      }, {} as Record<number, EnvGroup>)
    }
    catch (error) {
      console.error('Failed to load environment groups:', error)
    }
    finally {
      isLoading.value = false
    }
  }

  // Get environment group by ID
  function getGroupById(id: number): EnvGroup | undefined {
    return envGroupMap.value[id]
  }

  // Refresh all data
  async function refresh() {
    await loadAll()
  }

  return {
    envGroups: readonly(envGroups),
    isLoading: readonly(isLoading),
    isInitialized: readonly(isInitialized),
    lastUpdateTime: readonly(lastUpdateTime),
    initialize,
    loadAll,
    getGroupById,
    refresh,
  }
})
