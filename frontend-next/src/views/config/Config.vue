<template>
    <a-card :title="$gettext('Configurations')">
        <std-table
            :api="api"
            :columns="columns"
            :deletable="false"
            :disable_search="true"
            data_key="configs"
            row-key="name"
            @clickEdit="r => {
                $router.push({
                    path: '/config/' + r
                })
            }"
        />
    </a-card>
</template>

<script>
import StdTable from '@/components/StdDataDisplay/StdTable'
import $gettext from '@/lib/translate/gettext'

const columns = [{
    title: $gettext('Name'),
    dataIndex: 'name',
    scopedSlots: {customRender: 'name'},
    sorter: true,
    pithy: true
}, {
    title: $gettext('Updated at'),
    dataIndex: 'modify',
    datetime: true,
    scopedSlots: {customRender: 'modify'},
    sorter: true,
    pithy: true
}, {
    title: $gettext('Action'),
    dataIndex: 'action',
    scopedSlots: {customRender: 'action'}
}]

export default {
    name: 'Config',
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
