<script setup lang="ts">
import type { FormInstance } from 'antdv-next'
import site from '@/api/site'
import gettext from '@/gettext'

const props = defineProps<{
  visible: boolean
  name: string
}>()

const emit = defineEmits(['update:visible', 'duplicated'])
const { message } = useGlobalApp()

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

const formRef = ref<FormInstance>()

const loading = ref(false)

function onSubmit() {
  formRef.value?.validate().then(async () => {
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
    nextTick(() => formRef.value?.clearValidate())
  }
})

watch(() => gettext.current, () => {
  formRef.value?.clearValidate()
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
