<script setup lang="tsx">
import StdTable from '@/components/StdDataDisplay/StdTable.vue'
import StdCurd from '@/components/StdDataDisplay/StdCurd.vue'


import {customRender, datetime} from '@/components/StdDataDisplay/StdTableTransformer'
import {useGettext} from 'vue3-gettext'

const {$gettext, interpolate} = useGettext()

import log from '@/api/log'
import {Badge, message} from 'ant-design-vue'
import {h, ref} from 'vue'
import {input} from '@/components/StdDataEntry'

import {useRouter} from 'vue-router'

const router = useRouter()


const columns = [
    {
    title: () => $gettext('id'),
    dataIndex: 'id',
    sorter: true,
    pithy: true,
    // edit: {
    //     type: input
    // },
    }, 
    {
    title: () => $gettext('name'),
    dataIndex: 'name',
    sorter: true,
    pithy: true,
    edit: {
        type: input
    },
    }, 

    {
    title: () => $gettext('file path'),
    dataIndex: 'path',
    sorter: true,
    pithy: true,
    edit: {
        type: input
    },
    },  
    {
    title: () => $gettext('modify or delete'),
    dataIndex: 'action',
    },
    // add view action
    {
    title: () => $gettext('view log'),
      dataIndex: 'view',
     customRender: (args: customRender) => {
        const template: any = []
        const {text, record} = args
        template.push(<a-button type="button" class="btn btn-outline-secondary" onClick={() => router.push({
            path: '/nginx_log/' + record.name
        })}>{$gettext('view')}</a-button>)
        return h('div', template)
    },
    }
]

const table = ref(null)

interface Table {
    get_list(): void
}

</script>

<template>
    <std-curd 
    :title="$gettext('Manage Logs')" 
    :columns="columns" 
    :api="log" 
    row-key="name" 
    ref="table"
    @selected="onSelect"
    />
     
</template>

<style scoped>

</style>
