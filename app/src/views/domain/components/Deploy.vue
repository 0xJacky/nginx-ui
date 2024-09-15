<script setup lang="ts">
import { InfoCircleOutlined } from '@ant-design/icons-vue'
import { Modal, notification } from 'ant-design-vue'
import type { Ref } from 'vue'
import domain from '@/api/domain'
import NodeSelector from '@/components/NodeSelector/NodeSelector.vue'

const node_map = ref({})
const target = ref([])
const overwrite = ref(false)
const enabled = ref(false)
const name = inject('name') as Ref<string>
const [modal, ContextHolder] = Modal.useModal()
function deploy() {
  modal.confirm({
    title: () => $ngettext('Do you want to deploy this file to remote server?',
      'Do you want to deploy this file to remote servers?', target.value.length),
    mask: false,
    centered: true,
    okText: $gettext('OK'),
    cancelText: $gettext('Cancel'),
    onOk() {
      target.value.forEach(id => {
        const node_name = node_map.value[id]

        // get source content
        domain.get(name.value).then(r => {
          domain.save(name.value, {
            name: name.value,
            content: r.config,
            overwrite: overwrite.value,

          }, { headers: { 'X-Node-ID': id } }).then(async () => {
            notification.success({
              message: $gettext('Deploy successfully'),
              description:
                $gettext('Deploy %{conf_name} to %{node_name} successfully',
                  { conf_name: name.value, node_name }),
            })
            if (enabled.value) {
              domain.enable(name.value, { headers: { 'X-Node-ID': id } }).then(() => {
                notification.success({
                  message: $gettext('Enable successfully'),
                  description:
                    $gettext('Enable %{conf_name} in %{node_name} successfully',
                      { conf_name: name.value, node_name }),
                })
              }).catch(e => {
                notification.error({
                  message: $gettext('Enable %{conf_name} in %{node_name} failed', {
                    conf_name: name.value,
                    node_name,
                  }),
                  description: $gettext(e?.message ?? 'Server error'),
                })
              })
            }
          }).catch(e => {
            notification.error({
              message: $gettext('Deploy %{conf_name} to %{node_name} failed', {
                conf_name: name.value,
                node_name,
              }),
              description: $gettext(e?.message ?? 'Server error'),
            })
          })
        })
      })
    },
  })
}
</script>

<template>
  <div>
    <ContextHolder />
    <NodeSelector
      v-model:target="target"
      v-model:map="node_map"
      hidden-local
    />
    <div class="node-deploy-control">
      <ACheckbox v-model:checked="enabled">
        {{ $gettext('Enable') }}
      </ACheckbox>
      <div class="overwrite">
        <ACheckbox v-model:checked="overwrite">
          {{ $gettext('Overwrite') }}
        </ACheckbox>
        <ATooltip placement="bottom">
          <template #title>
            {{ $gettext('Overwrite exist file') }}
          </template>
          <InfoCircleOutlined />
        </ATooltip>
      </div>

      <AButton
        :disabled="target.length === 0"
        type="primary"
        ghost
        @click="deploy"
      >
        {{ $gettext('Deploy') }}
      </AButton>
    </div>
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
