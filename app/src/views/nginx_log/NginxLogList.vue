<script setup lang="tsx">
import type { CustomRender } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import type { Column } from '@/components/StdDesign/types'
import type { SSE, SSEvent } from 'sse.js'
import nginxLog from '@/api/nginx_log'
import StdCurd from '@/components/StdDesign/StdDataDisplay/StdCurd.vue'
import { input, select } from '@/components/StdDesign/StdDataEntry'
import { CheckCircleOutlined, LoadingOutlined } from '@ant-design/icons-vue'
import { Tag } from 'ant-design-vue'
import { onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'

const router = useRouter()
const isScanning = ref(false)
const stdCurdRef = ref()
const sse = ref<SSE | null>(null)

const columns: Column[] = [
  {
    title: () => $gettext('Type'),
    dataIndex: 'type',
    customRender: (args: CustomRender) => {
      return args.record?.type === 'access' ? <Tag color="success">{ $gettext('Access Log') }</Tag> : <Tag color="orange">{ $gettext('Error Log') }</Tag>
    },
    sorter: true,
    search: {
      type: select,
      mask: {
        access: () => $gettext('Access Log'),
        error: () => $gettext('Error Log'),
      },
    },
    width: 200,
  },
  {
    title: () => $gettext('Name'),
    dataIndex: 'name',
    sorter: true,
    search: {
      type: input,
    },
  },
  {
    title: () => $gettext('Path'),
    dataIndex: 'path',
    sorter: true,
    search: {
      type: input,
    },
  },
  {
    title: () => $gettext('Action'),
    dataIndex: 'action',
  },
]

function viewLog(record: { type: string, path: string }) {
  router.push({
    path: `/nginx_log/${record.type}`,
    query: {
      log_path: record.path,
    },
  })
}

// Connect to SSE endpoint and setup handlers
function setupSSE() {
  if (sse.value) {
    sse.value.close()
  }

  sse.value = nginxLog.logs_live()

  // Handle incoming messages
  if (sse.value) {
    sse.value.onmessage = (e: SSEvent) => {
      try {
        if (!e.data)
          return

        const data = JSON.parse(e.data)
        isScanning.value = data.scanning

        stdCurdRef.value.get_list()
      }
      catch (error) {
        console.error('Error parsing SSE message:', error)
      }
    }

    sse.value.onerror = () => {
      // Reconnect on error
      setTimeout(() => {
        setupSSE()
      }, 5000)
    }
  }
}

onMounted(() => {
  setupSSE()
})

onUnmounted(() => {
  if (sse.value) {
    sse.value.close()
  }
})
</script>

<template>
  <StdCurd
    ref="stdCurdRef"
    :title="$gettext('Log List')"
    :columns="columns"
    :api="nginxLog"
    disable-add
    disable-delete
    disable-view
    disable-modify
  >
    <template #extra>
      <APopover placement="bottomRight">
        <template #content>
          <div>
            {{ $gettext('Automatically indexed from site and stream configurations.') }}
            <br>
            {{ $gettext('If logs are not indexed, please check if the log file is under the directory in Nginx.LogDirWhiteList.') }}
          </div>
        </template>
        <div class="flex items-center cursor-pointer">
          <template v-if="isScanning">
            <LoadingOutlined class="mr-2" spin />{{ $gettext('Indexing...') }}
          </template>
          <template v-else>
            <CheckCircleOutlined class="mr-2" />{{ $gettext('Indexed') }}
          </template>
        </div>
      </APopover>
    </template>

    <template #actions="{ record }">
      <AButton type="link" size="small" @click="viewLog(record)">
        {{ $gettext('View') }}
      </AButton>
    </template>
  </StdCurd>
</template>

<style scoped lang="less">

</style>
