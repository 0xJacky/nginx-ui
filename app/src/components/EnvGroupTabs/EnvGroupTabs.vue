<script setup lang="ts">
import type ReconnectingWebSocket from 'reconnecting-websocket'
import type { EnvGroup } from '@/api/env_group'
import type { Environment } from '@/api/environment'
import { message } from 'ant-design-vue'
import nodeApi from '@/api/node'
import ws from '@/lib/websocket'

const props = defineProps<{
  envGroups: EnvGroup[]
}>()

const modelValue = defineModel<string | number>('activeKey')

const environments = ref<Environment[]>([])
const environmentsMap = ref<Record<number, Environment>>({})
const loading = ref({
  reload: false,
  restart: false,
})

// WebSocket connection for environment monitoring
let socket: ReconnectingWebSocket | WebSocket | null = null

// Get node data when tab is not 'All'
watch(modelValue, newVal => {
  if (newVal && newVal !== 0) {
    connectWebSocket()
  }
  else {
    disconnectWebSocket()
  }
}, { immediate: true })

function connectWebSocket() {
  if (socket) {
    socket.close()
  }

  socket = ws('/api/cluster/environments/enabled/ws', true)

  socket.onmessage = event => {
    try {
      const message = JSON.parse(event.data)

      if (message.event === 'message') {
        const data: Environment[] = message.data
        environments.value = data
        environmentsMap.value = environments.value.reduce((acc, node) => {
          acc[node.id] = node
          return acc
        }, {} as Record<number, Environment>)
      }
    }
    catch (error) {
      console.error('Error parsing WebSocket message:', error)
    }
  }

  socket.onerror = error => {
    console.warn('Failed to connect to environments WebSocket endpoint', error)
  }
}

function disconnectWebSocket() {
  if (socket) {
    socket.close()
    socket = null
  }
}

// Cleanup on unmount
onUnmounted(() => {
  disconnectWebSocket()
})

// Get the current Node Group data
const currentEnvGroup = computed(() => {
  if (!modelValue.value || modelValue.value === 0)
    return null
  return props.envGroups.find(g => g.id === Number(modelValue.value))
})

// Get the list of nodes in the current group
const syncNodes = computed(() => {
  if (!currentEnvGroup.value)
    return []

  if (!currentEnvGroup.value.sync_node_ids)
    return []

  return currentEnvGroup.value.sync_node_ids
    .map(id => environmentsMap.value[id])
    .filter(Boolean)
})

// Handle reload Nginx on all sync nodes
async function handleReloadNginx() {
  if (!currentEnvGroup.value || !syncNodes.value.length)
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
  if (!currentEnvGroup.value || !syncNodes.value.length)
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
    <ATabs :active-key="modelValue" @update:active-key="modelValue = $event">
      <ATabPane :key="0" :tab="$gettext('All')" />
      <ATabPane v-for="c in envGroups" :key="c.id" :tab="c.name" />
    </ATabs>

    <!-- Display node information -->
    <ACard
      v-if="modelValue && modelValue !== 0 && syncNodes.length > 0"
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
