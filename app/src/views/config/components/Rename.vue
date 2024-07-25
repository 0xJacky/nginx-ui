<script setup lang="ts">
import { message } from 'ant-design-vue'
import config from '@/api/config'
import useOTPModal from '@/components/OTP/useOTPModal'

const emit = defineEmits(['renamed'])
const visible = ref(false)

const data = ref({
  basePath: '',
  orig_name: '',
  new_name: '',
})

const refForm = ref()
function open(basePath: string, origName: string) {
  visible.value = true
  data.value.orig_name = origName
  data.value.new_name = origName
  data.value.basePath = basePath
}

defineExpose({
  open,
})

function ok() {
  refForm.value.validate().then(() => {
    const { basePath, orig_name, new_name } = data.value

    const otpModal = useOTPModal()

    otpModal.open({
      onOk() {
        config.rename(basePath, orig_name, new_name).then(() => {
          visible.value = false
          message.success($gettext('Rename successfully'))
          emit('renamed')
        }).catch(e => {
          message.error(`${$gettext('Server error')} ${e?.message}`)
        })
      },
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
    </AForm>
  </AModal>
</template>

<style scoped lang="less">

</style>
