<script setup lang="tsx">
import type { EnvGroup } from '@/api/env_group'
import type { Site } from '@/api/site'
import type { Column } from '@/components/StdDesign/types'
import env_group from '@/api/env_group'
import site from '@/api/site'
import EnvGroupTabs from '@/components/EnvGroupTabs'
import StdBatchEdit from '@/components/StdDesign/StdDataDisplay/StdBatchEdit.vue'
import StdTable from '@/components/StdDesign/StdDataDisplay/StdTable.vue'
import { ConfigStatus } from '@/constants'
import InspectConfig from '@/views/config/InspectConfig.vue'
import columns from '@/views/site/site_list/columns'
import SiteDuplicate from '@/views/site/site_list/SiteDuplicate.vue'
import { message } from 'ant-design-vue'

const route = useRoute()
const router = useRouter()

const table = ref()
const inspectConfig = ref()

const envGroupId = ref(Number.parseInt(route.query.env_group_id as string) || 0)
const envGroups = ref([]) as Ref<EnvGroup[]>

watch(route, () => {
  inspectConfig.value?.test()
})

onMounted(async () => {
  let page = 1
  while (true) {
    try {
      const { data, pagination } = await env_group.get_list({ page })
      if (!data || !pagination)
        return
      envGroups.value.push(...data)
      if (data.length < pagination?.per_page) {
        return
      }
      page++
    }
    catch {
      return
    }
  }
})

function destroy(site_name: string) {
  site.destroy(site_name).then(() => {
    table.value.get_list()
    message.success($gettext('Delete site: %{site_name}', { site_name }))
    inspectConfig.value?.test()
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
    <InspectConfig ref="inspectConfig" />

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
