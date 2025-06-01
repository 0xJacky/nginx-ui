<script setup lang="ts">
import type { Config } from '@/api/config'
import { InfoCircleOutlined } from '@ant-design/icons-vue'
import NodeSelector from '@/components/NodeSelector'

const data = defineModel<Config>('data', { required: true })
</script>

<template>
  <div>
    <NodeSelector
      v-model:target="data.sync_node_ids"
      hidden-local
    />
    <div class="node-deploy-control">
      <div class="overwrite">
        <ACheckbox v-model:checked="data.sync_overwrite">
          {{ $gettext('Overwrite') }}
        </ACheckbox>
        <ATooltip placement="bottom">
          <template #title>
            {{ $gettext('Overwrite exist file') }}
          </template>
          <InfoCircleOutlined />
        </ATooltip>
      </div>
    </div>
  </div>
</template>

<style lang="less" scoped>
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
