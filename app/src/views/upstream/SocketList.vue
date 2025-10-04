<script setup lang="ts">
import type { ColumnsType } from 'ant-design-vue/es/table'
import type { SocketInfo } from '@/api/upstream'
import { ReloadOutlined } from '@ant-design/icons-vue'
import { message, Tag } from 'ant-design-vue'
import upstream from '@/api/upstream'
import { formatDateTime } from '@/lib/helper'
import { useProxyAvailabilityStore } from '@/pinia/moudule/proxyAvailability'

const dataSource = ref<SocketInfo[]>([])
const loading = ref(false)

// Initialize proxy availability store
const proxyAvailabilityStore = useProxyAvailabilityStore()

const columns: ColumnsType<SocketInfo> = [
  {
    title: () => $gettext('Socket'),
    dataIndex: 'socket',
    key: 'socket',
    width: 200,
    fixed: 'left',
  },
  {
    title: () => $gettext('Upstream'),
    dataIndex: 'upstream_name',
    key: 'upstream_name',
    width: 150,
    customRender: ({ record }) => {
      if (!record.upstream_name) {
        return $gettext('Direct')
      }
      return record.upstream_name
    },
  },
  {
    title: () => $gettext('Health Status'),
    key: 'status',
    width: 180,
    customRender: ({ record }) => {
      if (!record.status) {
        return $gettext('No Data')
      }
      const status = record.status
      return h('div', { class: 'flex items-center' }, [
        h(Tag, { color: status.online ? 'success' : 'error', class: 'mr-2' }, () => status.online ? $gettext('Online') : $gettext('Offline')),
        status.online ? h('span', `${status.latency.toFixed(2)}ms`) : null,
      ])
    },
  },
  {
    title: () => $gettext('Last Check'),
    dataIndex: 'last_check',
    key: 'last_check',
    width: 180,
    customRender: ({ text }) => {
      return text ? formatDateTime(text) : '-'
    },
  },
  {
    title: () => $gettext('Health Check'),
    key: 'enabled',
    width: 150,
    fixed: 'right',
  },
]

// Merge socket list with real-time availability data
function mergeSocketData(sockets: SocketInfo[]): SocketInfo[] {
  return sockets.map(socket => {
    // Get real-time status from availability store
    const availabilityResult = proxyAvailabilityStore.availabilityResults[socket.socket]

    if (availabilityResult) {
      return {
        ...socket,
        status: {
          online: availabilityResult.online,
          latency: availabilityResult.latency,
        },
        last_check: new Date().toISOString(),
      }
    }

    return socket
  })
}

// Computed data source that combines socket list with real-time availability
const enrichedDataSource = computed(() => {
  return mergeSocketData(dataSource.value)
})

async function loadData() {
  loading.value = true
  try {
    const res = await upstream.getSocketList()
    dataSource.value = res.data
  }
  catch {
    message.error('Failed to load socket data')
  }
  finally {
    loading.value = false
  }
}

async function handleToggleEnabled(socket: string, enabled: boolean | string | number) {
  const isEnabled = typeof enabled === 'boolean' ? enabled : Boolean(enabled)
  try {
    await upstream.updateSocketConfig(socket, { enabled: isEnabled })
    message.success(`Health check ${isEnabled ? 'enabled' : 'disabled'} for ${socket}`)
    await loadData()
  }
  catch {
    message.error('Failed to update socket configuration')
  }
}

// Start monitoring when component mounts
onMounted(async () => {
  await loadData()
  // Start real-time monitoring for availability updates
  proxyAvailabilityStore.startMonitoring()
})

// Clean up WebSocket connections when component unmounts
onUnmounted(() => {
  proxyAvailabilityStore.stopMonitoring()
})
</script>

<template>
  <ACard :title="$gettext('Upstream Sockets')">
    <template #extra>
      <AButton :loading @click="loadData">
        <template #icon>
          <ReloadOutlined />
        </template>
      </AButton>
    </template>

    <ATable
      :columns="columns"
      :data-source="enrichedDataSource"
      :loading="loading"
      :pagination="{
        pageSize: 20,
        showSizeChanger: true,
        showTotal: (total: number) => `Total ${total} items`,
      }"
      :scroll="{ x: 1400 }"
      row-key="socket"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'socket'">
          <ATag color="default" :bordered="false" class="socket-tag">
            <template #icon>
              <span v-if="record.type === 'upstream'" class="target-type-icon">U</span>
              <span v-else class="target-type-icon">P</span>
            </template>
            {{ record.socket }}
          </ATag>
        </template>
        <template v-if="column.key === 'enabled'">
          <ASwitch
            v-model:checked="record.enabled"
            @change="handleToggleEnabled(record.socket, $event)"
          />
        </template>
      </template>
    </ATable>
  </ACard>
</template>

<style scoped lang="less">
.socket-tag {
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 12px;

  .target-type-icon {
    display: inline-block;
    width: 12px;
    height: 12px;
    line-height: 12px;
    text-align: center;
    border-radius: 2px;
    font-weight: bold;
    font-size: 10px;
    flex-shrink: 0;
  }
}
</style>
