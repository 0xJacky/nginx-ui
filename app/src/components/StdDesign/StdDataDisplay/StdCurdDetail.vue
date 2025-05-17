<script setup lang="ts">
import type { ComputedRef } from 'vue'
import type { Column } from '@/components/StdDesign/types'
import { get } from 'lodash'
import { CustomRender } from '@/components/StdDesign/StdDataDisplay/components/CustomRender'
import { labelRender } from '@/components/StdDesign/StdDataEntry'

const props = defineProps<{
  columns: Column[]
  // eslint-disable-next-line ts/no-explicit-any
  data: any
}>()

const displayColumns: ComputedRef<Column[]> = computed(() => {
  return props.columns.filter(c => !c.hiddenInDetail && c.dataIndex !== 'action')
})
</script>

<template>
  <ADescriptions
    :column="1"
    bordered
  >
    <ADescriptionsItem
      v-for="(c, index) in displayColumns"
      :key="index"
      :label="labelRender(c.title)"
    >
      <CustomRender v-bind="{ column: c, record: data, index, text: get(data, c.dataIndex!), isDetail: true }" />
    </ADescriptionsItem>
  </ADescriptions>
</template>

<style scoped lang="less">

</style>
