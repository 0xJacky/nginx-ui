<script setup lang="ts">
import type { Pagination } from '@/api/curd'

const props = withDefaults(defineProps<{
  pagination: Pagination
  size?: 'default' | 'small'
  loading?: boolean
  showSizeChanger?: boolean
}>(), {
  showSizeChanger: true,
})

const emit = defineEmits(['change', 'changePageSize', 'update:pagination'])

function change(num: number, pageSize: number) {
  emit('change', num, pageSize)
}

const pageSize = computed({
  get() {
    return props.pagination.per_page
  },
  set(v) {
    emit('changePageSize', v)
    emit('update:pagination', { ...props.pagination, per_page: v })
  },
})
</script>

<template>
  <div
    v-if="pagination.total > pagination.per_page"
    class="pagination-container"
  >
    <APagination
      v-model:page-size="pageSize"
      :disabled="loading"
      :current="pagination.current_page"
      :show-size-changer="showSizeChanger"
      :show-total="(total:number) => $ngettext('Total %{total} item', 'Total %{total} items', total, { total: total.toString() })"
      :size="size"
      :total="pagination.total"
      @change="change"
    />
  </div>
</template>

<style lang="less">
.ant-pagination-total-text {
  @media (max-width: 450px) {
    display: block;
  }
}
</style>

<style lang="less" scoped>
.pagination-container {
  padding: 10px 0 0 0;
  display: flex;
  justify-content: right;
  @media (max-width: 450px) {
    justify-content: center;
  }
}
</style>
