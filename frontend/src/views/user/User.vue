<template>
    <std-curd :columns="columns" :api="api" :disable_search="true"/>
</template>

<script>

import StdCurd from '@/components/StdDataDisplay/StdCurd'
import $gettext from '@/lib/translate/gettext'

const columns = [{
    title: $gettext('Username'),
    dataIndex: 'name',
    sorter: true,
    pithy: true,
    edit: {
        type: 'input'
    }
}, {
    title: $gettext('Password'),
    dataIndex: 'password',
    sorter: true,
    pithy: true,
    edit: {
        type: 'input',
        placeholder: $gettext('Leave blank for no change')
    },
    display: false
}, {
    title: $gettext('Created at'),
    dataIndex: 'created_at',
    datetime: true,
    sorter: true,
    pithy: true
}, {
    title: $gettext('Updated at'),
    dataIndex: 'updated_at',
    datetime: true,
    sorter: true,
    pithy: true
}, {
    title: $gettext('Action'),
    dataIndex: 'action'
}]

export default {
    name: 'User',
    components: {StdCurd},
    data() {
        return {
            api: this.$api.user,
            columns
        }
    },
    methods: {}
}
</script>

<style scoped>

</style>
