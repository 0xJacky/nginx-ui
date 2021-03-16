<template>
    <div v-if="Object.keys(pagination).length !== 0">
        <a-pagination
            :current="pagination.current_page"
            :hideOnSinglePage="true"
            :pageSize="pagination.per_page"
            :size="size"
            :total="pagination.total"
            class="pagination"
            @change="changePage"
        />
        <div class="clear"></div>
    </div>
</template>

<script>
export default {
    name: 'StdPagination',
    props: {
        pagination: Object,
        size: {
            default: ''
        }
    },
    methods: {
        changePage(num) {
            return this.$emit('changePage', num)
        }
    }
}
</script>

<style scoped>
.pagination {
    padding: 10px 0 0 0;
    float: right;
}
</style>
