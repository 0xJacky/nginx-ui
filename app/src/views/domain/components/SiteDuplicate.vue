<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import { useGettext } from 'vue3-gettext'
import { Form, message, notification } from 'ant-design-vue'
import gettext from '@/gettext'
import domain from '@/api/domain'
import NodeSelector from '@/components/NodeSelector/NodeSelector.vue'
import { useSettingsStore } from '@/pinia'

const props = defineProps<{
  visible: boolean
  name: string
}>()

const emit = defineEmits(['update:visible', 'duplicated'])

const { $gettext } = useGettext()

const settings = useSettingsStore()

const show = computed({
  get() {
    return props.visible
  },
  set(v) {
    emit('update:visible', v)
  },
})

const modelRef = reactive({ name: '', target: [] })

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

const node_map = reactive({})

function onSubmit() {
  validate().then(async () => {
    loading.value = true

    modelRef.target.forEach(id => {
      if (id === 0) {
        // eslint-disable-next-line promise/no-nesting
        domain.duplicate(props.name, { name: modelRef.name }).then(() => {
          message.success($gettext('Duplicate to local successfully'))
          show.value = false
          emit('duplicated')
          // eslint-disable-next-line promise/no-nesting
        }).catch(e => {
          message.error($gettext(e?.message ?? 'Server error'))
        })
      }
      else {
        // get source content
        // eslint-disable-next-line promise/no-nesting
        domain.get(props.name).then(r => {
          domain.save(modelRef.name, {
            name: modelRef.name,
            content: r.config,
            // eslint-disable-next-line promise/no-nesting
          }, { headers: { 'X-Node-ID': id } }).then(() => {
            notification.success({
              message: $gettext('Duplicate successfully'),
              description:
                $gettext('Duplicate %{conf_name} to %{node_name} successfully',
                  { conf_name: props.name, node_name: node_map[id] }),
            })
            // eslint-disable-next-line promise/no-nesting
          }).catch(e => {
            notification.error({
              message: $gettext('Duplicate failed'),
              description: $gettext(e?.message ?? 'Server error'),
            })
          })
          if (r.enabled) {
            // eslint-disable-next-line promise/no-nesting
            domain.enable(modelRef.name, { headers: { 'X-Node-ID': id } }).then(() => {
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
    :mask="null"
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
          :map="node_map"
        />
      </AFormItem>
    </AForm>
  </AModal>
</template>

<style lang="less" scoped>

</style>
