<script setup lang="ts">
import type { ComputedRef } from 'vue'
import _ from 'lodash'
import type { Column } from '@/components/StdDesign/types'
import { labelRender } from '@/components/StdDesign/StdDataEntry'
import { CustomRender } from '@/components/StdDesign/StdDataDisplay/components/CustomRender'

const props = defineProps<{
  columns: Column[]
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
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
      <CustomRender v-bind="{ column: c, record: data, index, text: _.get(data, c.dataIndex!), isDetail: true }" />
    </ADescriptionsItem>
  </ADescriptions>
</template>

<style scoped lang="less">

</style>
