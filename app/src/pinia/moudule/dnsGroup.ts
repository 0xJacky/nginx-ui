import { v4 as uuidv4 } from 'uuid'
import { useSettingsStore } from '@/pinia/moudule/settings'
import { useUserStore } from '@/pinia/moudule/user'

export interface DNSDomainGroup {
  id: string
  name: string
  description: string
  domainIds: number[]
  createdAt: string
  updatedAt: string
}

export interface DNSDomainGroupInput {
  name: string
  description?: string
  domainIds: number[]
}

export const useDnsGroupStore = defineStore('dnsGroup', () => {
  const settingsStore = useSettingsStore()
  const userStore = useUserStore()
  const groupsByScope = ref<Record<string, DNSDomainGroup[]>>({})

  const scopeKey = computed(() => {
    const userId = userStore.info.id || 0
    const nodeId = settingsStore.node.id || 0
    return `${userId}:${nodeId}`
  })

  const groups = computed(() => groupsByScope.value[scopeKey.value] ?? [])

  function replaceScopedGroups(nextGroups: DNSDomainGroup[]) {
    groupsByScope.value = {
      ...groupsByScope.value,
      [scopeKey.value]: nextGroups,
    }
  }

  function createGroup(input: DNSDomainGroupInput) {
    const now = new Date().toISOString()
    const group: DNSDomainGroup = {
      id: uuidv4(),
      name: input.name.trim(),
      description: input.description?.trim() ?? '',
      domainIds: [...new Set(input.domainIds)],
      createdAt: now,
      updatedAt: now,
    }
    replaceScopedGroups([...groups.value, group])
    return group
  }

  function updateGroup(id: string, input: DNSDomainGroupInput) {
    const nextGroups = groups.value.map(group => group.id === id
      ? {
          ...group,
          name: input.name.trim(),
          description: input.description?.trim() ?? '',
          domainIds: [...new Set(input.domainIds)],
          updatedAt: new Date().toISOString(),
        }
      : group)
    replaceScopedGroups(nextGroups)
  }

  function deleteGroup(id: string) {
    replaceScopedGroups(groups.value.filter(group => group.id !== id))
  }

  function getGroup(id: string) {
    return groups.value.find(group => group.id === id)
  }

  return {
    groupsByScope,
    scopeKey,
    groups,
    createGroup,
    updateGroup,
    deleteGroup,
    getGroup,
  }
}, {
  persist: {
    key: 'nginx-ui-dns-groups-v1',
    storage: localStorage,
    pick: ['groupsByScope'],
  },
})
