<template>
    <std-curd :columns="columns" :api="api" :disable_search="true"/>
</template>

<script>

import StdCurd from "@/components/StdDataDisplay/StdCurd"

const columns = [{
    title: "用户名",
    dataIndex: "name",
    sorter: true,
    pithy: true,
    edit: {
        type: 'input'
    }
}, {
    title: "密码",
    dataIndex: "password",
    sorter: true,
    pithy: true,
    edit: {
        type: 'input',
        placeholder: '留空不修改'
    },
    display: false
}, {
    title: "创建时间",
    dataIndex: "created_at",
    datetime: true,
    sorter: true,
    pithy: true
}, {
    title: "修改时间",
    dataIndex: "updated_at",
    datetime: true,
    sorter: true,
    pithy: true
}, {
    title: "操作",
    dataIndex: "action"
}]

export default {
    name: "User",
    components: {StdCurd},
    data() {
        return {
            api: this.$api.user,
            columns
        }
    },
    methods: {

    }
}
</script>

<style scoped>

</style>
