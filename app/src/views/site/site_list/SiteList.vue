<script setup lang="tsx">
import type { Site } from '@/api/site'
import type { SiteCategory } from '@/api/site_category'
import type { Column } from '@/components/StdDesign/types'
import site from '@/api/site'
import site_category from '@/api/site_category'
import StdBatchEdit from '@/components/StdDesign/StdDataDisplay/StdBatchEdit.vue'
import StdTable from '@/components/StdDesign/StdDataDisplay/StdTable.vue'
import InspectConfig from '@/views/config/InspectConfig.vue'
import columns from '@/views/site/site_list/columns'
import SiteDuplicate from '@/views/site/site_list/SiteDuplicate.vue'
import { message } from 'ant-design-vue'

const route = useRoute()
const router = useRouter()

const table = ref()
const inspect_config = ref()

const siteCategoryId = ref(Number.parseInt(route.query.site_category_id as string) || 0)
const siteCategories = ref([]) as Ref<SiteCategory[]>

watch(route, () => {
  inspect_config.value?.test()
})

onMounted(async () => {
  while (true) {
    try {
      const { data, pagination } = await site_category.get_list()
      if (!data || !pagination)
        return
      siteCategories.value.push(...data)
      if (data.length < pagination?.per_page) {
        return
      }
    }
    // eslint-disable-next-line ts/no-explicit-any
    catch (e: any) {
      message.error(e?.message ?? $gettext('Server error'))
      return
    }
  }
})

function enable(name: string) {
  site.enable(name).then(() => {
    message.success($gettext('Enabled successfully'))
    table.value?.get_list()
    inspect_config.value?.test()
  }).catch(r => {
    message.error($gettext('Failed to enable %{msg}', { msg: r.message ?? '' }), 10)
  })
}

function disable(name: string) {
  site.disable(name).then(() => {
    message.success($gettext('Disabled successfully'))
    table.value?.get_list()
    inspect_config.value?.test()
  }).catch(r => {
    message.error($gettext('Failed to disable %{msg}', { msg: r.message ?? '' }))
  })
}

function destroy(site_name: string) {
  site.destroy(site_name).then(() => {
    table.value.get_list()
    message.success($gettext('Delete site: %{site_name}', { site_name }))
    inspect_config.value?.test()
  }).catch(e => {
    message.error(e?.message ?? $gettext('Server error'))
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
    <InspectConfig ref="inspect_config" />

    <ATabs v-model:active-key="siteCategoryId">
      <ATabPane :key="0" :tab="$gettext('All')" />
      <ATabPane v-for="c in siteCategories" :key="c.id" :tab="c.name" />
    </ATabs>

    <StdTable
      ref="table"
      :api="site"
      :columns="columns"
      row-key="name"
      disable-delete
      disable-view
      :get-params="{
        site_category_id: siteCategoryId,
      }"
      @click-edit="(r: string) => router.push({
        path: `/sites/${r}`,
      })"
      @click-batch-modify="handleClickBatchEdit"
    >
      <template #actions="{ record }">
        <AButton
          v-if="record.enabled"
          type="link"
          size="small"
          @click="disable(record.name)"
        >
          {{ $gettext('Disable') }}
        </AButton>
        <AButton
          v-else
          type="link"
          size="small"
          @click="enable(record.name)"
        >
          {{ $gettext('Enable') }}
        </AButton>
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
          :disabled="record.enabled"
          @confirm="destroy(record.name)"
        >
          <AButton
            type="link"
            size="small"
            :disabled="record.enabled"
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
