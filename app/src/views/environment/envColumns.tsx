import type { CustomRender } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import type { Column, JSXElements } from '@/components/StdDesign/types'
import { datetime } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import { input, switcher } from '@/components/StdDesign/StdDataEntry'
import { Badge, Tag } from 'ant-design-vue'
import { h } from 'vue'

const columns: Column[] = [{
  title: () => $gettext('Name'),
  dataIndex: 'name',
  sorter: true,
  pithy: true,
  edit: {
    type: input,
  },
  search: true,
}, {
  title: () => $gettext('URL'),
  dataIndex: 'url',
  sorter: true,
  pithy: true,
  edit: {
    type: input,
    config: {
      placeholder: () => 'https://10.0.0.1:9000',
    },
  },
}, {
  title: () => $gettext('Version'),
  dataIndex: 'version',
  pithy: true,
}, {
  title: () => 'NodeSecret',
  dataIndex: 'token',
  sorter: true,
  hiddenInTable: true,
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
  customRender: (args: CustomRender) => {
    const template: JSXElements = []
    const { text } = args
    if (args.record.enabled) {
      if (text === true || text > 0) {
        template.push(<Badge status="success" />)
        template.push($gettext('Online'))
      }
      else {
        template.push(<Badge status="error" />)
        template.push($gettext('Offline'))
      }
    }
    else {
      template.push(<Badge status="default" />)
      template.push($gettext('Disabled'))
    }

    return h('div', template)
  },
  sorter: true,
  pithy: true,
}, {
  title: () => $gettext('Enabled'),
  dataIndex: 'enabled',
  customRender: (args: CustomRender) => {
    const template: JSXElements = []
    const { text } = args
    if (text === true || text > 0)
      template.push(<Tag color="green">{$gettext('Enabled')}</Tag>)

    else
      template.push(<Tag color="orange">{$gettext('Disabled')}</Tag>)

    return h('div', template)
  },
  edit: {
    type: switcher,
  },
  sorter: true,
  pithy: true,
}, {
  title: () => $gettext('Updated at'),
  dataIndex: 'updated_at',
  customRender: datetime,
  sorter: true,
  pithy: true,
}, {
  title: () => $gettext('Action'),
  dataIndex: 'action',
}]

export default columns
