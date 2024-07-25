<script setup lang="ts">

import { message } from 'ant-design-vue'
import config from '@/api/config'
import useOTPModal from '@/components/OTP/useOTPModal'

const emit = defineEmits(['created'])
const visible = ref(false)

const data = ref({
  basePath: '',
  name: '',
})

const refForm = ref()
function open(basePath: string) {
  visible.value = true
  data.value.name = ''
  data.value.basePath = basePath
}

defineExpose({
  open,
})

function ok() {
  refForm.value.validate().then(() => {
    const otpModal = useOTPModal()

    otpModal.open({
      onOk() {
        config.mkdir(data.value.basePath, data.value.name).then(() => {
          visible.value = false

          message.success($gettext('Created successfully'))
          emit('created')
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
    :title="$gettext('Create Folder')"
    @ok="ok"
  >
    <AForm
      ref="refForm"
      layout="vertical"
      :model="data"
      :rules="{
        name: [
          { required: true, message: $gettext('Please input a folder name') },
          { pattern: /^[^\\/]+$/, message: $gettext('Invalid folder name') },
        ],
      }"
    >
      <AFormItem name="name">
        <AInput
          v-model:value="data.name"
          :placeholder="$gettext('Name')"
        />
      </AFormItem>
    </AForm>
  </AModal>
</template>

<style scoped lang="less">

</style>
