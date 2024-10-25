<script setup lang="ts">
import NodeSelector from '@/components/NodeSelector/NodeSelector.vue'

const node_map = ref({})
const target = ref([])
</script>

<template>
  <NodeSelector
    v-model:target="target"
    v-model:map="node_map"
    class="mb-4"
    hidden-local
  />
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
