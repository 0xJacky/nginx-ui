<script setup lang="tsx">
import { useGettext } from 'vue3-gettext'
import { h } from 'vue'
import { Badge } from 'ant-design-vue'
import type { customRender } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import { datetime } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import environment from '@/api/environment'
import StdCurd from '@/components/StdDesign/StdDataDisplay/StdCurd.vue'
import { input } from '@/components/StdDesign/StdDataEntry'

const { $gettext } = useGettext()

const columns = [{
  title: () => $gettext('Name'),
  dataIndex: 'name',
  sorter: true,
  pithy: true,
  edit: {
    type: input,
  },
},
{
  title: () => $gettext('URL'),
  dataIndex: 'url',
  sorter: true,
  pithy: true,
  edit: {
    type: input,
    placeholder: () => 'https://10.0.0.1:9000',
  },
},
{
  title: () => 'NodeSecret',
  dataIndex: 'token',
  sorter: true,
  display: false,
  edit: {
    type: input,
  },
},

//     {
//     title: () => $gettext('OperationSync'),
//     dataIndex: 'operation_sync',
//     sorter: true,
//     pithy: true,
//     edit: {
//         type: antSwitch
//     },
//     extra: $gettext('Whether config api regex that will redo on this environment'),
//     customRender: (args: customRender) => {
//         const {operation_sync} = args.record
//         if (operation_sync) {
//             return h(Tag, {color: 'success'}, {default: ()=> h('span', $gettext('Yes'))})
//         } else {
//             return h(Tag, {color: 'default'}, {default: ()=> h('span', $gettext('No'))})
//         }
//     },
// }, {
//     title: () => $gettext('SyncApiRegex'),
//     dataIndex: 'sync_api_regex',
//     sorter: true,
//     pithy: true,
//     display: false,
//     edit: {
//       type: textarea,
//       show: (data) => {
//         const {operation_sync} = data
//         return operation_sync
//       }
//     },
//     extra: $gettext('Such as Reload and Configs, regex can configure as `/api/nginx/reload|/api/nginx/test|/api/config/.+`, please see system api'),
// },
{
  title: () => $gettext('Status'),
  dataIndex: 'status',
  customRender: (args: customRender) => {
    const template = []
    const { text } = args
    if (text === true || text > 0) {
      template.push(<Badge status="success"/>)
      template.push($gettext('Online'))
    }
    else {
      template.push(<Badge status="error"/>)
      template.push($gettext('Offline'))
    }

    return h('div', template)
  },
  sorter: true,
  pithy: true,
},
{
  title: () => $gettext('Updated at'),
  dataIndex: 'updated_at',
  customRender: datetime,
  sorter: true,
  pithy: true,
},
{
  title: () => $gettext('Action'),
  dataIndex: 'action',
}]

</script>

<template>
  <StdCurd
    :title="$gettext('Environment')"
    :api="environment"
    :columns="columns"
  />
</template>

<style lang="less" scoped>

</style>
