<script setup lang="tsx">
import type { Site } from '@/api/site'
import { StdCurd } from '@uozi-admin/curd'
import { message, Modal } from 'ant-design-vue'
import site from '@/api/site'
import FooterToolBar from '@/components/FooterToolbar'
import InspectConfig from '@/components/InspectConfig'
import NamespaceTabs from '@/components/NamespaceTabs'
import { ConfigStatus } from '@/constants'
import columns from '@/views/site/site_list/columns'
import SiteDuplicate from '@/views/site/site_list/SiteDuplicate.vue'

const route = useRoute()
const router = useRouter()

const curd = ref()
const inspectConfig = ref()
const selectedSiteNames = ref<string[]>([])
const selectedSites = ref<Site[]>([])
const loadingEnable = ref(false)
const loadingDisable = ref(false)
const [modal, ContextHolder] = Modal.useModal()

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

function clearSelectedSites() {
  selectedSiteNames.value = []
  selectedSites.value = []
}

function refreshAfterBatchStatusChanged() {
  clearSelectedSites()
  curd.value.refresh()
  inspectConfig.value?.test()
}

type BatchStatusAction = 'enable' | 'disable'

function executeBatchStatusAction(action: BatchStatusAction, names: string[]) {
  const isEnable = action === 'enable'
  const loading = isEnable ? loadingEnable : loadingDisable
  const request = isEnable ? site.batchEnable : site.batchDisable
  const successMessage = isEnable ? $gettext('Sites enabled successfully') : $gettext('Sites disabled successfully')

  loading.value = true
  return request(names).then(() => {
    message.success(successMessage)
    refreshAfterBatchStatusChanged()
  }).finally(() => {
    loading.value = false
  })
}

function confirmBatchStatusAction(action: BatchStatusAction) {
  if (selectedSiteNames.value.length === 0) {
    message.warning(action === 'enable'
      ? $gettext('Please select at least one site to enable')
      : $gettext('Please select at least one site to disable'))
    return
  }

  const names = [...selectedSiteNames.value]
  const isEnable = action === 'enable'

  modal.confirm({
    title: isEnable
      ? $gettext('Do you want to enable selected sites?')
      : $gettext('Do you want to disable selected sites?'),
    content: () => h('div', [
      h('p', isEnable
        ? $gettext('The following sites will be enabled:')
        : $gettext('The following sites will be disabled:')),
      h('ul', { class: 'max-h-60 overflow-auto pl-5' }, names.map(name => h('li', { key: name }, name))),
    ]),
    mask: false,
    centered: true,
    okText: isEnable ? $gettext('Enable') : $gettext('Disable'),
    okButtonProps: {
      danger: !isEnable,
    },
    cancelText: $gettext('Cancel'),
    onOk: () => executeBatchStatusAction(action, names),
  })
}

function batchEnableSites() {
  confirmBatchStatusAction('enable')
}

function batchDisableSites() {
  confirmBatchStatusAction('disable')
}
</script>

<template>
  <div>
    <StdCurd
      ref="curd"
      v-model:selected-row-keys="selectedSiteNames"
      v-model:selected-rows="selectedSites"
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
        <InspectConfig ref="inspectConfig" :namespace-id="namespaceId" />
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
    <ContextHolder />

    <FooterToolBar v-if="selectedSiteNames.length > 0">
      <template #extra>
        {{ $gettext('%{count} sites selected', { count: String(selectedSiteNames.length) }) }}
      </template>

      <ASpace>
        <AButton
          :loading="loadingEnable"
          type="primary"
          @click="batchEnableSites"
        >
          {{ $gettext('Enable') }}
        </AButton>

        <AButton
          :loading="loadingDisable"
          danger
          @click="batchDisableSites"
        >
          {{ $gettext('Disable') }}
        </AButton>
      </ASpace>
    </FooterToolBar>
  </div>
</template>

<style scoped>

</style>
