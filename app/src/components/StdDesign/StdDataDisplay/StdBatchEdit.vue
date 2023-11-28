<script setup lang="ts">
import { message } from 'ant-design-vue'
import gettext from '@/gettext'
import StdDataEntry from '@/components/StdDesign/StdDataEntry'

const props = defineProps<{
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  api: (ids: number[], data: any) => Promise<void>
  beforeSave?: () => Promise<void>
}>()

const emit = defineEmits(['onSave'])

const { $gettext } = gettext

const batchColumns = ref([])

const visible = ref(false)

const selectedRowKeys = ref([])
// eslint-disable-next-line @typescript-eslint/no-explicit-any
function showModal(c: any, rowKeys: any) {
  visible.value = true
  selectedRowKeys.value = rowKeys
  batchColumns.value = c
}

defineExpose({
  showModal,
})

const data = reactive({})
const error = reactive({})
const loading = ref(false)

async function ok() {
  loading.value = true

  await props.beforeSave?.()

  await props.api(selectedRowKeys.value, data).then(async () => {
    message.success($gettext('Save successfully'))
    emit('onSave')
  }).catch(e => {
    message.error($gettext(e?.message) ?? $gettext('Server error'))
  }).finally(() => {
    loading.value = false
  })
}
</script>

<template>
  <AModal
    v-model:open="visible"
    class="std-curd-edit-modal"
    :mask="false"
    :title="$gettext('Batch Modify')"
    :cancel-text="$gettext('Cancel')"
    :ok-text="$gettext('OK')"
    :confirm-loading="loading"
    :width="600"
    destroy-on-close
    @ok="ok"
  >
    <StdDataEntry
      :data-list="batchColumns"
      :data-source="data"
      :error="error"
    />

    <slot name="extra" />
  </AModal>
</template>

<style scoped>

</style>
