<script setup lang="ts">
import {computed, nextTick, reactive, ref, watch} from 'vue'
import {useGettext} from 'vue3-gettext'
import {Form, message, notification} from 'ant-design-vue'
import gettext from '@/gettext'
import domain from '@/api/domain'
import NodeSelector from '@/components/NodeSelector/NodeSelector.vue'
import {useSettingsStore} from '@/pinia'

const {$gettext} = useGettext()

const props = defineProps(['visible', 'name'])
const emit = defineEmits(['update:visible', 'duplicated'])

const settings = useSettingsStore()

const show = computed({
  get() {
    return props.visible
  },
  set(v) {
    emit('update:visible', v)
  }
})

const modelRef = reactive({name: '', target: []})

const rulesRef = reactive({
  name: [
    {
      required: true,
      message: () => $gettext('Please input name, ' +
        'this will be used as the filename of the new configuration!')
    }
  ],
  target: [
    {
      required: true,
      message: () => $gettext('Please select at least one node!')
    }
  ]
})

const {validate, validateInfos, clearValidate} = Form.useForm(modelRef, rulesRef)

const loading = ref(false)

const node_map = reactive({})

function onSubmit() {
  validate().then(async () => {
    loading.value = true

    modelRef.target.forEach(id => {
      if (id === 0) {
        domain.duplicate(props.name, {name: modelRef.name}).then(() => {
          message.success($gettext('Duplicate to local successfully'))
          show.value = false
          emit('duplicated')
        }).catch((e: any) => {
          message.error($gettext(e?.message ?? 'Server error'))
        })
      } else {
        // get source content
        domain.get(props.name).then(r => {
          domain.save(modelRef.name, {
            name: modelRef.name,
            content: r.config
          }, {headers: {'X-Node-ID': id}}).then(() => {
            notification.success({
              message: $gettext('Duplicate successfully'),
              description:
                $gettext('Duplicate %{conf_name} to %{node_name} successfully',
                  {conf_name: props.name, node_name: node_map[id]})
            })
          }).catch(e => {
            notification.error({
              message: $gettext('Duplicate failed'),
              description: $gettext(e?.message ?? 'Server error')
            })
          })
          if (r.enabled) {
            domain.enable(modelRef.name, {headers: {'X-Node-ID': id}}).then(() => {
              notification.success({
                message: $gettext('Enabled successfully')
              })
            })
          }
        })
      }
    })

    loading.value = false
  })
}

watch(() => props.visible, (v) => {
  if (v) {
    modelRef.name = props.name  // default with source name
    modelRef.target = [0]
    nextTick(() => clearValidate())
  }
})

watch(() => gettext.current, () => {
  clearValidate()
})
</script>

<template>
  <a-modal :title="$gettext('Duplicate')" v-model:open="show" @ok="onSubmit"
           :confirm-loading="loading" :mask="null">
    <a-form layout="vertical">
      <a-form-item :label="$gettext('Name')" v-bind="validateInfos.name">
        <a-input v-model:value="modelRef.name"/>
      </a-form-item>
      <a-form-item v-if="!settings.is_remote" :label="$gettext('Target')" v-bind="validateInfos.target">
        <node-selector v-model:target="modelRef.target" :map="node_map"/>
      </a-form-item>
    </a-form>
  </a-modal>
</template>

<style lang="less" scoped>

</style>
