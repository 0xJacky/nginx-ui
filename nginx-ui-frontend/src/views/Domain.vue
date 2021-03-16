<template>
    <a-card title="网站管理">
        <std-table
            :api="api"
            :columns="columns"
            data_key="configs"
            ref="table"
            @clickEdit="r => this.$router.push({
                path: '/domain/' + r.name
            })"
        >
            <template #action="{record}">
                <a v-if="record.enabled" @click="disable(record.name)">禁用</a>
                <a v-else @click="enable(record.name)">启用</a>
                <a-divider type="vertical"/>
            </template>
        </std-table>
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
    title: "状态",
    dataIndex: "enabled",
    badge: true,
    scopedSlots: {customRender: "enabled"},
    mask: {
        true: "启用",
        false: "未启用"
    },
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
    name: "Domain",
    components: {StdTable},
    data() {
        return {
            api: this.$api.domain,
            columns
        }
    },
    methods: {
        enable(name) {
            this.$api.domain.enable(name).then(() => {
                this.$message.success("启用成功")
                this.$refs.table.get_list()
            }).catch(r => {
                console.log(r)
                this.$message.error("启用失败")
            })
        },
        disable(name) {
            this.$api.domain.disable(name).then(() => {
                this.$message.success("禁用成功")
                this.$refs.table.get_list()
            }).catch(r => {
                console.log(r)
                this.$message.error("禁用失败")
            })
        }
    }
}
</script>

<style scoped>

</style>
