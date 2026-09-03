<script setup lang="ts">
import type { FormInstance } from 'antdv-next'
import stream from '@/api/stream'
import gettext from '@/gettext'

const props = defineProps<{
  name: string
}>()

const emit = defineEmits(['duplicated'])
const { message } = useGlobalApp()

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

const formRef = ref<FormInstance>()

const loading = ref(false)

function onSubmit() {
  formRef.value?.validate().then(async () => {
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
    nextTick(() => formRef.value?.clearValidate())
  }
})

watch(() => gettext.current, () => {
  formRef.value?.clearValidate()
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
    <AForm
      ref="formRef"
      layout="vertical"
      :model="modelRef"
      :rules="rulesRef"
    >
      <AFormItem
        :label="$gettext('Name')"
        name="name"
      >
        <AInput v-model:value="modelRef.name" />
      </AFormItem>
    </AForm>
  </AModal>
</template>

<style lang="less" scoped>

</style>
