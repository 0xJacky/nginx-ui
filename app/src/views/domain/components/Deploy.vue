<script setup lang="ts">
import NodeSelector from '@/components/NodeSelector/NodeSelector.vue'
import {useGettext} from 'vue3-gettext'
import {inject, reactive, ref} from 'vue'
import {InfoCircleOutlined} from '@ant-design/icons-vue'
import Modal from 'ant-design-vue/lib/modal'
import domain from '@/api/domain'
import {notification} from 'ant-design-vue'
import template from '@/api/template'

const {$gettext, $ngettext} = useGettext()

const node_map = reactive({})
const target = ref([])
const overwrite = ref(false)
const enabled = ref(false)
const name = inject('name')

function deploy() {
  Modal.confirm({
    title: () => $ngettext('Do you want to deploy this file to remote server?',
      'Do you want to deploy this file to remote servers?', target.value.length),
    mask: false,
    centered: true,
    okText: $gettext('OK'),
    cancelText: $gettext('Cancel'),
    onOk() {
      target.value.forEach(id => {
        const node_name = node_map[id]
        // get source content
        domain.get(name.value).then(r => {
          domain.save(name.value, {
            name: name.value,
            content: r.config,
            overwrite: overwrite.value
          }, {headers: {'X-Node-ID': id}}).then(async () => {
            notification.success({
              message: $gettext('Deploy successfully'),
              description:
                $gettext('Deploy %{conf_name} to %{node_name} successfully',
                  {conf_name: name.value, node_name: node_name})
            })
            if (enabled.value) {
              domain.enable(name.value).then(() => {
                notification.success({
                  message: $gettext('Enable successfully'),
                  description:
                    $gettext(`Enable %{conf_name} in %{node_name} successfully`,
                      {conf_name: name.value, node_name: node_name})
                })
              }).catch(e => {
                notification.error({
                  message: $gettext('Enable %{conf_name} in %{node_name} failed', {
                    conf_name: name.value,
                    node_name: node_name
                  }),
                  description: $gettext(e?.message ?? 'Server error')
                })
              })
            }
          }).catch(e => {
            notification.error({
              message: $gettext('Deploy %{conf_name} to %{node_name} failed', {
                conf_name: name.value,
                node_name: node_name
              }),
              description: $gettext(e?.message ?? 'Server error')
            })
          })
        })
      })
    }
  })
}
</script>

<template>
  <node-selector v-model:target="target" :hidden_local="true" :map="node_map"/>
  <div class="node-deploy-control">
    <a-checkbox v-model:checked="enabled">{{ $gettext('Enabled') }}</a-checkbox>
    <div class="overwrite">
      <a-checkbox v-model:checked="overwrite">{{ $gettext('Overwrite') }}</a-checkbox>
      <a-tooltip placement="bottom">
        <template #title>{{ $gettext('Overwrite exist file') }}</template>
        <info-circle-outlined/>
      </a-tooltip>
    </div>

    <a-button :disabled="target.length===0" type="primary" @click="deploy" ghost>{{ $gettext('Deploy') }}</a-button>
  </div>
</template>

<style scoped lang="less">
.overwrite {
  margin-right: 15px;

  span {
    color: #9b9b9b;
  }
}

.node-deploy-control {
  display: flex;
  justify-content: flex-end;
  margin-top: 10px;
  align-items: center;
}
</style>
