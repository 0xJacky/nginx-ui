<script setup lang="ts">
import { Form, message, notification } from 'ant-design-vue'

import stream from '@/api/stream'
import NodeSelector from '@/components/NodeSelector/NodeSelector.vue'
import { useSettingsStore } from '@/pinia'
import gettext from '@/gettext'

const props = defineProps<{
  visible: boolean
  name: string
}>()

const emit = defineEmits(['update:visible', 'duplicated'])

const settings = useSettingsStore()

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
  target: number[] // ids of deploy targets
}

const modelRef: Model = reactive({ name: '', target: [] })

const rulesRef = reactive({
  name: [
    {
      required: true,
      message: () => $gettext('Please input name, '
        + 'this will be used as the filename of the new configuration!'),
    },
  ],
  target: [
    {
      required: true,
      message: () => $gettext('Please select at least one node!'),
    },
  ],
})

const { validate, validateInfos, clearValidate } = Form.useForm(modelRef, rulesRef)

const loading = ref(false)

const node_map: Record<number, string> = reactive({})

function onSubmit() {
  validate().then(async () => {
    loading.value = true

    modelRef.target.forEach(id => {
      if (id === 0) {
        stream.duplicate(props.name, { name: modelRef.name }).then(() => {
          message.success($gettext('Duplicate to local successfully'))
          show.value = false
          emit('duplicated')
        }).catch(e => {
          message.error($gettext(e?.message ?? 'Server error'))
        })
      }
      else {
        // get source content

        stream.get(props.name).then(r => {
          stream.save(modelRef.name, {
            name: modelRef.name,
            content: r.config,

          }, { headers: { 'X-Node-ID': id } }).then(() => {
            notification.success({
              message: $gettext('Duplicate successfully'),
              description:
                $gettext('Duplicate %{conf_name} to %{node_name} successfully',
                  { conf_name: props.name, node_name: node_map[id] }),
            })
          }).catch(e => {
            notification.error({
              message: $gettext('Duplicate failed'),
              description: $gettext(e?.message ?? 'Server error'),
            })
          })
          if (r.enabled) {
            stream.enable(modelRef.name, { headers: { 'X-Node-ID': id } }).then(() => {
              notification.success({
                message: $gettext('Enabled successfully'),
              })
            })
          }
        })
      }
    })

    loading.value = false
  })
}

watch(() => props.visible, v => {
  if (v) {
    modelRef.name = props.name // default with source name
    modelRef.target = [0]
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
      <AFormItem
        v-if="!settings.is_remote"
        :label="$gettext('Target')"
        v-bind="validateInfos.target"
      >
        <NodeSelector
          v-model:target="modelRef.target"
          v-model:map="node_map"
        />
      </AFormItem>
    </AForm>
  </AModal>
</template>

<style lang="less" scoped>

</style>
