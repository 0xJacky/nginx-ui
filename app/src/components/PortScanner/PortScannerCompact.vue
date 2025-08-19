<script setup lang="ts">
import type { PortInfo, PortScanRequest } from '@/api/port_scan'
import { Badge, message } from 'ant-design-vue'
import portScan from '@/api/port_scan'

interface FormData {
  startPort: number
  endPort: number
}

const loading = ref(false)
const formData = reactive<FormData>({
  startPort: 80,
  endPort: 8080,
})

const tableData = ref<PortInfo[]>([])
const pagination = reactive({
  current: 1,
  pageSize: 20,
  total: 0,
  showSizeChanger: false,
  showQuickJumper: false,
  simple: true,
  size: 'small' as const,
})

const columns = [
  {
    title: $gettext('Port'),
    dataIndex: 'port',
    key: 'port',
    width: 60,
  },
  {
    title: $gettext('Status'),
    dataIndex: 'status',
    key: 'status',
    width: 80,
    customRender: ({ text }: { text: string }) => {
      const statusMap = {
        listening: { color: 'orange', text: $gettext('Listening') },
        open: { color: 'blue', text: $gettext('Open') },
        closed: { color: 'green', text: $gettext('Closed') },
      }
      const status = statusMap[text as keyof typeof statusMap] || { status: 'error', text: $gettext('Unknown') }
      return h(Badge, {
        color: status.color,
        text: h('span', { style: 'font-size: 11px;' }, status.text),
      })
    },
  },
  {
    title: $gettext('Process'),
    dataIndex: 'process',
    key: 'process',
    ellipsis: true,
    customRender: ({ text }: { text: string }) => {
      if (!text)
        return '-'

      // Extract process name from format like "1234/nginx: master process" or "1234/nginx"
      // Use a more specific regex to avoid backtracking issues
      const match = text.match(/^\d+\/([^:]+)/)
      if (match) {
        const processName = match[1].trim()
        return h('span', { title: text, style: 'font-size: 12px;' }, processName)
      }

      // Fallback: if no match, show first 10 characters
      return h('span', { title: text, style: 'font-size: 12px;' }, text.substring(0, 10))
    },
  },
]

const isFormValid = computed(() => {
  return formData.startPort >= 1
    && formData.endPort <= 65535
    && formData.startPort <= formData.endPort
})

async function scanPorts() {
  if (!isFormValid.value) {
    message.error($gettext('Please enter a valid port range'))
    return
  }

  loading.value = true
  pagination.current = 1

  try {
    await loadData()
    message.success($gettext('Scan completed'))
  }
  catch (error) {
    console.error('Port scan failed:', error)
    message.error($gettext('Scan failed'))
  }
  finally {
    loading.value = false
  }
}

async function loadData() {
  const request: PortScanRequest = {
    start_port: formData.startPort,
    end_port: formData.endPort,
    page: pagination.current,
    page_size: pagination.pageSize,
  }

  const response = await portScan.scan(request)
  tableData.value = response.data
  pagination.total = response.total
}

async function handleTableChange(pag: { current?: number, pageSize?: number }) {
  if (pag.current) {
    pagination.current = pag.current
  }

  if (pagination.total > 0) {
    loading.value = true
    try {
      await loadData()
    }
    finally {
      loading.value = false
    }
  }
}

function quickScan(start: number, end: number) {
  formData.startPort = start
  formData.endPort = end
  scanPorts()
}
</script>

<template>
  <div class="port-scanner-compact px-6 mb-6">
    <div class="scan-form">
      <ASpace direction="vertical" size="small" style="width: 100%">
        <ARow :gutter="8">
          <ACol :span="11">
            <AInputNumber
              v-model:value="formData.startPort"
              :min="1"
              :max="65535"
              :placeholder="$gettext('Start')"
              size="small"
              style="width: 100%"
            />
          </ACol>
          <ACol :span="2" class="text-center">
            <span style="line-height: 24px;">-</span>
          </ACol>
          <ACol :span="11">
            <AInputNumber
              v-model:value="formData.endPort"
              :min="1"
              :max="65535"
              :placeholder="$gettext('End')"
              size="small"
              style="width: 100%"
            />
          </ACol>
        </ARow>

        <AButton
          type="primary"
          size="small"
          :loading="loading"
          :disabled="!isFormValid"
          block
          @click="scanPorts"
        >
          {{ $gettext('Scan Ports') }}
        </AButton>

        <div class="quick-actions">
          <ASpace size="small" wrap>
            <AButton size="small" @click="quickScan(80, 443)">
              Web
            </AButton>
            <AButton size="small" @click="quickScan(20, 22)">
              SSH/FTP
            </AButton>
            <AButton size="small" @click="quickScan(3306, 5432)">
              DB
            </AButton>
            <AButton size="small" @click="quickScan(1, 1024)">
              {{ $gettext('System') }}
            </AButton>
          </ASpace>
        </div>
      </ASpace>
    </div>

    <div v-if="pagination.total > 0" class="scan-results">
      <ADivider style="margin: 12px 0;">
        <span style="font-size: 12px; color: #666;">
          {{ $gettext('Scan Results') }} ({{ pagination.total }})
        </span>
      </ADivider>

      <ATable
        :columns="columns"
        :data-source="tableData"
        :pagination="pagination"
        :loading="loading"
        size="small"
        :scroll="{ y: 300 }"
        @change="handleTableChange"
      />
    </div>
  </div>
</template>

<style scoped lang="less">
.port-scanner-compact {
  .scan-form {
    padding: 8px 0;
  }

  .quick-actions {
    :deep(.ant-btn) {
      font-size: 11px;
      height: 20px;
      padding: 0 6px;
    }
  }

  .scan-results {
    :deep(.ant-table) {
      font-size: 12px;

      .ant-table-thead > tr > th {
        padding: 4px 8px;
        font-size: 11px;
        background-color: var(--ant-color-fill-alter);
      }

      .ant-table-tbody > tr > td {
        padding: 4px 8px;
      }

      .ant-pagination {
        margin: 8px 0 0 0;
        text-align: center;

        .ant-pagination-item,
        .ant-pagination-prev,
        .ant-pagination-next {
          min-width: 24px;
          height: 24px;
          line-height: 22px;
          font-size: 12px;
        }
      }
    }

    // Custom styling for badge status indicators
    :deep(.ant-badge) {
      .ant-badge-status-dot {
        width: 8px;
        height: 8px;
      }

      .ant-badge-status-text {
        margin-left: 6px;
        font-size: 11px;
      }
    }
  }

  .text-center {
    text-align: center;
  }
}
</style>
