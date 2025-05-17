<script setup lang="ts">
import { Form, message } from 'ant-design-vue'

import site from '@/api/site'
import gettext from '@/gettext'

const props = defineProps<{
  visible: boolean
  name: string
}>()

const emit = defineEmits(['update:visible', 'duplicated'])

const show = computed({
  get() {
    return props.visible
  },
  set(v) {
    emit('update:visible', v)
  },
})

interface Model {
  name: string // site name
}

const modelRef: Model = reactive({ name: '' })

const rulesRef = reactive({
  name: [
    {
      required: true,
      message: () => $gettext('Please input name, '
        + 'this will be used as the filename of the new configuration.'),
    },
  ],
})

const { validate, validateInfos, clearValidate } = Form.useForm(modelRef, rulesRef)

const loading = ref(false)

function onSubmit() {
  validate().then(async () => {
    loading.value = true

    site.duplicate(props.name, { name: modelRef.name }).then(() => {
      message.success($gettext('Duplicate to local successfully'))
      show.value = false
      emit('duplicated')
    })

    loading.value = false
  })
}

watch(() => props.visible, v => {
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
    v-model:open="show"
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
