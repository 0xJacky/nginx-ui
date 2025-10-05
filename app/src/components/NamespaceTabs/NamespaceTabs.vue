<script setup lang="ts">
import type { Namespace } from '@/api/namespace'
import namespaceApi from '@/api/namespace'
import nodeApi from '@/api/node'
import { useNodeAvailabilityStore } from '@/pinia/moudule/nodeAvailability'

defineProps<{
  hideNodeInfo?: boolean
}>()

const modelValue = defineModel<string | number>('activeKey')
const nodeStore = useNodeAvailabilityStore()
const namespaces = ref<Namespace[]>([])
const { message } = useGlobalApp()

// Load all namespaces on mount (handle pagination)
async function loadAllNamespaces() {
  const allNamespaces: Namespace[] = []
  let currentPage = 1
  let hasMore = true

  while (hasMore) {
    try {
      const response = await namespaceApi.getList({ page: currentPage })
      allNamespaces.push(...response.data)

      if (response.pagination && response.pagination.current_page < response.pagination.total_pages) {
        currentPage++
      }
      else {
        hasMore = false
      }
    }
    catch (error) {
      console.error('Failed to load namespaces:', error)
      hasMore = false
    }
  }

  namespaces.value = allNamespaces
}

onMounted(() => {
  loadAllNamespaces()
})

const loading = ref({
  reload: false,
  restart: false,
})

// Get the current Node Group data
const currentNamespace = computed(() => {
  if (!modelValue.value || modelValue.value === 0)
    return null
  return namespaces.value.find(g => g.id === Number(modelValue.value))
})

// Get the list of nodes in the current group
const syncNodes = computed(() => {
  if (!currentNamespace.value)
    return []

  if (!currentNamespace.value.sync_node_ids)
    return []

  return currentNamespace.value.sync_node_ids
    .map(id => nodeStore.getNodeStatus(id))
    .filter((node): node is NonNullable<typeof node> => Boolean(node))
})

// Handle reload Nginx on all sync nodes
async function handleReloadNginx() {
  if (!currentNamespace.value || !syncNodes.value.length)
    return

  const nodeIds = syncNodes.value.map(node => node.id)

  loading.value.reload = true
  try {
    await nodeApi.reloadNginx(nodeIds)
  }
  catch (error) {
    console.error(error)
    message.error($gettext('Reload request failed, please check your network connection'))
  }
  finally {
    loading.value.reload = false
  }
}

// Handle restart Nginx on all sync nodes
async function handleRestartNginx() {
  if (!currentNamespace.value || !syncNodes.value.length)
    return

  const nodeIds = syncNodes.value.map(node => node.id)

  loading.value.restart = true
  try {
    await nodeApi.restartNginx(nodeIds)
  }
  catch (error) {
    console.error(error)
    message.error($gettext('Restart request failed, please check your network connection'))
  }
  finally {
    loading.value.restart = false
  }
}
</script>

<template>
  <div>
    <ATabs v-model:active-key="modelValue">
      <ATabPane :key="0" :tab="$gettext('Local')" />
      <ATabPane
        v-for="ns in namespaces"
        :key="ns.id"
        :tab="ns.name"
      />
    </ATabs>

    <!-- Display node information -->
    <ACard
      v-if="!hideNodeInfo && modelValue && modelValue !== 0 && syncNodes.length > 0"
      :title="$gettext('Sync Nodes')"
      size="small"
      class="mb-4"
    >
      <template #extra>
        <ASpace>
          <APopconfirm
            :title="$gettext('Are you sure you want to reload Nginx on the following sync nodes?')"
            :ok-text="$gettext('Yes')"
            :cancel-text="$gettext('No')"
            placement="bottom"
            @confirm="handleReloadNginx"
          >
            <AButton type="link" size="small" :loading="loading.reload">
              {{ $gettext('Reload Nginx') }}
            </AButton>
          </APopconfirm>

          <APopconfirm
            :title="$gettext('Are you sure you want to restart Nginx on the following sync nodes?')"
            :ok-text="$gettext('Yes')"
            :cancel-text="$gettext('No')"
            placement="bottomRight"
            @confirm="handleRestartNginx"
          >
            <AButton type="link" danger size="small" :loading="loading.restart">
              {{ $gettext('Restart Nginx') }}
            </AButton>
          </APopconfirm>
        </ASpace>
      </template>

      <ARow :gutter="[16, 16]">
        <ACol v-for="node in syncNodes" :key="node.id" :xs="24" :sm="12" :md="8" :lg="6" :xl="4">
          <div class="node-item">
            <span class="node-name">{{ node.name }}</span>
            <ATag :color="node.status ? 'green' : 'error'">
              {{ node.status ? $gettext('Online') : $gettext('Offline') }}
            </ATag>
          </div>
        </ACol>
      </ARow>
    </ACard>
  </div>
</template>

<style scoped>
.node-name {
  margin-right: 8px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
</style>
