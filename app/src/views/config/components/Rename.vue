<script setup lang="ts">
import config from '@/api/config'
import NodeSelector from '@/components/NodeSelector'
import use2FAModal from '@/components/TwoFA/use2FAModal'

const emit = defineEmits(['renamed'])
const { message } = useGlobalApp()
const visible = ref(false)
const isDirFlag = ref(false)

const data = ref({
  basePath: '',
  orig_name: '',
  new_name: '',
  sync_node_ids: [] as number[],
})

// eslint-disable-next-line vue/require-typed-ref
const refForm = ref()

function open(basePath: string, origName: string, isDir: boolean) {
  visible.value = true
  data.value.orig_name = origName
  data.value.new_name = origName
  data.value.basePath = basePath
  isDirFlag.value = isDir
}

defineExpose({
  open,
})

function ok() {
  refForm.value.validate().then(() => {
    const { basePath, orig_name, new_name, sync_node_ids } = data.value

    const otpModal = use2FAModal()

    otpModal.open().then(() => {
      // Note: API will handle URL encoding of path segments
      config.rename(basePath, orig_name, new_name, sync_node_ids).then(() => {
        visible.value = false
        message.success($gettext('Rename successfully'))

        emit('renamed')
      })
    })
  })
}
</script>

<template>
  <AModal
    v-model:open="visible"
    :mask="false"
    :title="$gettext('Rename')"
    @ok="ok"
  >
    <AForm
      ref="refForm"
      layout="vertical"
      :model="data"
      :rules="{
        new_name: [
          { required: true, message: $gettext('Please input a filename') },
          { pattern: /^[^\\/]+$/, message: $gettext('Invalid filename') },
        ],
      }"
    >
      <AFormItem :label="$gettext('Original name')">
        <p>{{ data.orig_name }}</p>
      </AFormItem>
      <AFormItem
        :label="$gettext('New name')"
        name="new_name"
      >
        <AInput v-model:value="data.new_name" />
      </AFormItem>
      <AFormItem
        v-if="isDirFlag"
        :label="$gettext('Sync')"
      >
        <NodeSelector
          v-model:target="data.sync_node_ids"
          hidden-local
        />
      </AFormItem>
    </AForm>
  </AModal>
</template>

<style scoped lang="less">

</style>
