<template>
    <a-card title="配置文件">
        <std-table
            :api="api"
            :columns="columns"
            :deletable="false"
            :disable_search="true"
            data_key="configs"
            row-key="name"
            @clickEdit="item => {
                $router.push({
                    path: '/config/' + item.name
                })
            }"
        />
    </a-card>
</template>

<script>
import StdTable from "@/components/StdDataDisplay/StdTable"

const columns = [{
    title: "名称",
    dataIndex: "name",
    scopedSlots: {customRender: "名称"},
    sorter: true,
    pithy: true
}, {
    title: "修改时间",
    dataIndex: "modify",
    datetime: true,
    scopedSlots: {customRender: "modify"},
    sorter: true,
    pithy: true
}, {
    title: "操作",
    dataIndex: "action",
    scopedSlots: {customRender: "action"}
}]

export default {
    name: "Config",
    components: {StdTable},
    data() {
        return {
            api: this.$api.config,
            columns
        }
    }
}
</script>

<style scoped>

</style>
