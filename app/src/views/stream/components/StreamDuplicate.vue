<script setup lang="ts">
import { Form, message } from 'ant-design-vue'

import stream from '@/api/stream'
import gettext from '@/gettext'

const props = defineProps<{
  name: string
}>()

const emit = defineEmits(['duplicated'])

const visible = defineModel<boolean>('visible')

interface Model {
  name: string // site name
}

const modelRef: Model = reactive({ name: '' })

const rulesRef = reactive({
  name: [
    {
      required: true,
      message: () => $gettext('Please input name, '
        + 'this will be used as the filename of the new configuration!'),
    },
  ],
})

const { validate, validateInfos, clearValidate } = Form.useForm(modelRef, rulesRef)

const loading = ref(false)

function onSubmit() {
  validate().then(async () => {
    loading.value = true

    stream.duplicate(props.name, { name: modelRef.name }).then(() => {
      message.success($gettext('Duplicate to local successfully'))
      visible.value = false
      emit('duplicated')
    }).finally(() => {
      loading.value = false
    })
  })
}

watch(visible, v => {
  if (v) {
    modelRef.name = props.name // default with source name
    nextTick(() => clearValidate())
  }
})

watch(() => gettext.current, () => {
  clearValidate()
})
</script>

<template>
  <AModal
    v-model:open="visible"
    :title="$gettext('Duplicate')"
    :confirm-loading="loading"
    :mask="false"
    @ok="onSubmit"
  >
    <AForm layout="vertical">
      <AFormItem
        :label="$gettext('Name')"
        v-bind="validateInfos.name"
      >
        <AInput v-model:value="modelRef.name" />
      </AFormItem>
    </AForm>
  </AModal>
</template>

<style lang="less" scoped>

</style>
