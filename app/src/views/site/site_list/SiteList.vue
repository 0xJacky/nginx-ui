<script setup lang="tsx">
import { StdCurd } from '@uozi-admin/curd'
import { message } from 'ant-design-vue'
import site from '@/api/site'
import NamespaceTabs from '@/components/NamespaceTabs'
import { ConfigStatus } from '@/constants'
import InspectConfig from '@/views/config/InspectConfig.vue'
import columns from '@/views/site/site_list/columns'
import SiteDuplicate from '@/views/site/site_list/SiteDuplicate.vue'

const route = useRoute()
const router = useRouter()

const curd = ref()
const inspectConfig = ref()

const namespaceId = ref(Number.parseInt(route.query.namespace_id as string) || 0)

watch(route, () => {
  inspectConfig.value?.test()
})

function destroy(site_name: string) {
  site.deleteItem(site_name).then(() => {
    curd.value.refresh()
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
</script>

<template>
  <div>
    <StdCurd
      ref="curd"
      :title="$gettext('Manage Sites')"
      :api="site"
      :columns="columns"
      :table-props="{
        rowKey: 'name',
      }"
      disable-add
      disable-delete
      disable-trash
      disable-view
      disable-export
      row-selection-type="checkbox"
      :custom-query-params="{
        namespace_id: namespaceId,
      }"
      :scroll-x="1600"
      @edit-item="record => router.push({
        path: `/sites/${encodeURIComponent(record.name)}`,
      })"
    >
      <template #beforeListActions>
        <AButton
          type="link"
          size="small"
          @click="router.push({
            path: '/sites/add',
          })"
        >
          {{ $gettext('Add') }}
        </AButton>
      </template>
      <template #beforeCardBody>
        <InspectConfig ref="inspectConfig" />
        <NamespaceTabs v-model:active-key="namespaceId" />
      </template>
      <template #afterActions="{ record }">
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
    </StdCurd>
    <SiteDuplicate
      v-model:visible="show_duplicator"
      :name="target"
      @duplicated="() => curd.refresh()"
    />
  </div>
</template>

<style scoped>

</style>
