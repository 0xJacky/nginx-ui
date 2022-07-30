<template>
    <div v-if="Object.keys(pagination).length !== 0">
        <a-pagination
            :current="pagination.current_page"
            :hideOnSinglePage="true"
            :pageSize="pagination.per_page"
            :size="size"
            :total="pagination.total"
            :show-total="(total, range) => `当前显示${range[0]}-${range[1]}条数据，共${total}条数据`"
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
