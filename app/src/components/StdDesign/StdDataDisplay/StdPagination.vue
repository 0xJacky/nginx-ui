<script setup lang="ts">

import type { Pagination } from '@/api/curd'

const props = defineProps<{
  pagination: Pagination
  size?: 'small' | 'default'
}>()

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
      v-model:pageSize="pageSize"
      :current="pagination.current_page"
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
