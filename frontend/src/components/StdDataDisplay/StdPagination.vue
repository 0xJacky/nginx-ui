<script setup lang="ts">
import {useGettext} from 'vue3-gettext'
import {computed} from 'vue'

const props = defineProps(['pagination', 'size'])
const emit = defineEmits(['change', 'changePageSize'])
const {$gettext} = useGettext()

function change(num: number, pageSize: number) {
    emit('change', num, pageSize)
}

const pageSize = computed({
    get() {
        return props.pagination.per_page
    },
    set(v) {
        emit('changePageSize', v)
        props.pagination.per_page = v
    }
})
</script>

<template>
    <div class="pagination-container" v-if="pagination.total>pagination.per_page">
        <a-pagination
                :current="pagination.current_page"
                v-model:pageSize="pageSize"
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
