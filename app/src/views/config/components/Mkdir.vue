<script setup lang="ts">
import config from '@/api/config'
import use2FAModal from '@/components/TwoFA/use2FAModal'
import { message } from 'ant-design-vue'

const emit = defineEmits(['created'])
const visible = ref(false)

const data = ref({
  basePath: '',
  name: '',
})

// eslint-disable-next-line vue/require-typed-ref
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
    const otpModal = use2FAModal()

    otpModal.open().then(() => {
      config.mkdir(data.value.basePath, data.value.name).then(() => {
        visible.value = false

        message.success($gettext('Created successfully'))
        emit('created')
      })
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
