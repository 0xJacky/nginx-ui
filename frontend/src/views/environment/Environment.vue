<script setup lang="tsx">
import {useGettext} from 'vue3-gettext'
import {customRender, datetime} from '@/components/StdDataDisplay/StdTableTransformer'
import environment from '@/api/environment'
import StdCurd from '@/components/StdDataDisplay/StdCurd.vue'
import {input} from '@/components/StdDataEntry'
import {h} from 'vue'
import {Badge} from 'ant-design-vue'

const {$gettext, interpolate} = useGettext()

const columns = [{
    title: () => $gettext('Name'),
    dataIndex: 'name',
    sorter: true,
    pithy: true,
    edit: {
        type: input
    }
}, {
    title: () => $gettext('URL'),
    dataIndex: 'url',
    sorter: true,
    pithy: true,
    edit: {
        type: input,
        placeholder: () => 'https://10.0.0.1:9000'
    }
}, {
    title: () => $gettext('Token'),
    dataIndex: 'token',
    sorter: true,
    display: false,
    edit: {
        type: input
    }
}, {
    title: () => $gettext('Status'),
    dataIndex: 'status',
    customRender: (args: customRender) => {
        const template: any = []
        const {text} = args
        if (text === true || text > 0) {
            template.push(<Badge status="success"/>)
            template.push($gettext('Online'))
        } else {
            template.push(<Badge status="error"/>)
            template.push($gettext('Offline'))
        }
        return h('div', template)
    },
    sorter: true,
    pithy: true
}, {
    title: () => $gettext('Updated at'),
    dataIndex: 'updated_at',
    customRender: datetime,
    sorter: true,
    pithy: true
}, {
    title: () => $gettext('Action'),
    dataIndex: 'action'
}]

</script>

<template>
    <std-curd :title="$gettext('Environment')" :api="environment" :columns="columns"/>
</template>

<style lang="less" scoped>

</style>
