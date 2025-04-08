<script setup lang="tsx">
import type { EnvGroup } from '@/api/env_group'
import type { Site } from '@/api/site'
import type { Column } from '@/components/StdDesign/types'
import type { SSE, SSEvent } from 'sse.js'
import cacheIndex from '@/api/cache_index'
import env_group from '@/api/env_group'
import site from '@/api/site'
import EnvGroupTabs from '@/components/EnvGroupTabs/EnvGroupTabs.vue'
import StdBatchEdit from '@/components/StdDesign/StdDataDisplay/StdBatchEdit.vue'
import StdTable from '@/components/StdDesign/StdDataDisplay/StdTable.vue'
import { ConfigStatus } from '@/constants'
import InspectConfig from '@/views/config/InspectConfig.vue'
import columns from '@/views/site/site_list/columns'
import SiteDuplicate from '@/views/site/site_list/SiteDuplicate.vue'
import { CheckCircleOutlined, LoadingOutlined } from '@ant-design/icons-vue'
import { message } from 'ant-design-vue'

const route = useRoute()
const router = useRouter()

const table = ref()
const inspect_config = ref()

const envGroupId = ref(Number.parseInt(route.query.env_group_id as string) || 0)
const envGroups = ref([]) as Ref<EnvGroup[]>
const isScanning = ref(false)
const sse = ref<SSE>()

watch(route, () => {
  inspect_config.value?.test()
})

onMounted(async () => {
  while (true) {
    try {
      const { data, pagination } = await env_group.get_list()
      if (!data || !pagination)
        return
      envGroups.value.push(...data)
      if (data.length < pagination?.per_page) {
        return
      }
    }
    catch {
      return
    }
  }
})

onMounted(() => {
  setupSSE()
})

// Connect to SSE endpoint and setup handlers
async function setupSSE() {
  if (sse.value) {
    sse.value.close()
  }

  sse.value = cacheIndex.index_status()

  // Handle incoming messages
  if (sse.value) {
    sse.value.onmessage = (e: SSEvent) => {
      try {
        if (!e.data)
          return

        const data = JSON.parse(e.data)
        isScanning.value = data.scanning

        table.value.get_list()
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

onUnmounted(() => {
  if (sse.value) {
    sse.value.close()
  }
})

function destroy(site_name: string) {
  site.destroy(site_name).then(() => {
    table.value.get_list()
    message.success($gettext('Delete site: %{site_name}', { site_name }))
    inspect_config.value?.test()
  })
}

const show_duplicator = ref(false)

const target = ref('')

function handle_click_duplicate(name: string) {
  show_duplicator.value = true
  target.value = name
}

const stdBatchEditRef = useTemplateRef('stdBatchEditRef')

async function handleClickBatchEdit(batchColumns: Column[], selectedRowKeys: string[], selectedRows: Site[]) {
  stdBatchEditRef.value?.showModal(batchColumns, selectedRowKeys, selectedRows)
}

function handleBatchUpdated() {
  table.value?.get_list()
  table.value?.resetSelection()
}
</script>

<template>
  <ACard :title="$gettext('Manage Sites')">
    <template #extra>
      <div class="flex items-center cursor-default">
        <template v-if="isScanning">
          <LoadingOutlined class="mr-2" spin />{{ $gettext('Indexing...') }}
        </template>
        <template v-else>
          <CheckCircleOutlined class="mr-2" />{{ $gettext('Indexed') }}
        </template>
      </div>
    </template>
    <InspectConfig ref="inspect_config" />

    <EnvGroupTabs v-model:active-key="envGroupId" :env-groups="envGroups" />

    <StdTable
      ref="table"
      :api="site"
      :columns="columns"
      row-key="name"
      disable-delete
      disable-view
      :get-params="{
        env_group_id: envGroupId,
      }"
      :scroll-x="1600"
      @click-edit="(r: string) => router.push({
        path: `/sites/${encodeURIComponent(r)}`,
      })"
      @click-batch-modify="handleClickBatchEdit"
    >
      <template #actions="{ record }">
        <AButton
          type="link"
          size="small"
          @click="handle_click_duplicate(record.name)"
        >
          {{ $gettext('Duplicate') }}
        </AButton>
        <APopconfirm
          :cancel-text="$gettext('No')"
          :ok-text="$gettext('OK')"
          :title="$gettext('Are you sure you want to delete?')"
          :disabled="record.status !== ConfigStatus.Disabled"
          @confirm="destroy(record.name)"
        >
          <AButton
            type="link"
            size="small"
            :disabled="record.status !== ConfigStatus.Disabled"
          >
            {{ $gettext('Delete') }}
          </AButton>
        </APopconfirm>
      </template>
    </StdTable>
    <StdBatchEdit
      ref="stdBatchEditRef"
      :api="site"
      :columns
      @save="handleBatchUpdated"
    />
    <SiteDuplicate
      v-model:visible="show_duplicator"
      :name="target"
      @duplicated="() => table.get_list()"
    />
  </ACard>
</template>

<style scoped>

</style>
