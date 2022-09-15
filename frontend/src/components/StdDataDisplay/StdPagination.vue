<script setup lang="ts">
import {useGettext} from 'vue3-gettext'

const {pagination, size} = defineProps(['pagination', 'size'])
const emit = defineEmits(['changePage'])
const {$gettext} = useGettext()

function changePage(num: number) {
    emit('changePage', num)
}
</script>

<template>
    <div v-if="pagination.total>pagination.per_page">
        <a-pagination
                :current="pagination.current_page"
                :pageSize="pagination.per_page"
                :size="size"
                :total="pagination.total"
                class="pagination"
                @change="changePage"
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
.pagination {
    padding: 10px 0 0 0;
    float: right;
    @media (max-width: 450px) {
        float: unset;
        text-align: center;
    }
}
</style>
