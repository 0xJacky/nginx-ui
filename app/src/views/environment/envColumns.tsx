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
  width: 200,
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
  width: 300,
}, {
  title: () => $gettext('Version'),
  dataIndex: 'version',
  pithy: true,
  width: 150,
}, {
  title: () => 'NodeSecret',
  dataIndex: 'token',
  sorter: true,
  hiddenInTable: true,
  edit: {
    type: input,
  },
}, {
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
  width: 200,
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
  width: 150,
}, {
  title: () => $gettext('Updated at'),
  dataIndex: 'updated_at',
  customRender: datetime,
  sorter: true,
  pithy: true,
  width: 150,
}, {
  title: () => $gettext('Action'),
  dataIndex: 'action',
  fixed: 'right',
  width: 200,
}]

export default columns
