<script setup lang="ts">
import StdTable from '@/components/StdDataDisplay/StdTable.vue'
import gettext from '@/gettext'
import config from '@/api/config'
import {datetime} from '@/components/StdDataDisplay/StdTableTransformer'

const {$gettext} = gettext

const api = config

const columns = [{
    title: () => $gettext('Name'),
    dataIndex: 'name',
    sorter: true,
    pithy: true
}, {
    title: () => $gettext('Updated at'),
    dataIndex: 'modify',
    customRender: datetime,
    datetime: true,
    sorter: true,
    pithy: true
}, {
    title: () => $gettext('Action'),
    dataIndex: 'action'
}]
</script>
<template>
    <a-card :title="$gettext('Configurations')">
        <std-table
            :api="api"
            :columns="columns"
            :deletable="false"
            :disable_search="true"
            row-key="name"
            @clickEdit="r => {
                $router.push({
                    path: '/config/' + r
                })
            }"
        />
    </a-card>
</template>

<style scoped>

</style>
