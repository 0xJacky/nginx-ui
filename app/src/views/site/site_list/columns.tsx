import type {
  CustomRender,
} from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import type { Column, JSXElements } from '@/components/StdDesign/types'
import env_group from '@/api/env_group'
import {
  actualValueRender,
  datetime,
} from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import { input, select, selector } from '@/components/StdDesign/StdDataEntry'
import envGroupColumns from '@/views/environments/group/columns'
import { Badge } from 'ant-design-vue'

const columns: Column[] = [{
  title: () => $gettext('Name'),
  dataIndex: 'name',
  sorter: true,
  pithy: true,
  edit: {
    type: input,
  },
  search: true,
  width: 120,
}, {
  title: () => $gettext('URLs'),
  dataIndex: 'urls',
  customRender: ({ text, record }) => {
    const template: JSXElements = []
    if (record.enabled) {
      text?.forEach((url: string) => {
        template.push(<a href={url} target="_blank" rel="noopener noreferrer">{url}</a>)
        template.push(<span>, </span>)
      })
      template.pop() // Remove last comma
    }
    else {
      template.push(<span>{text?.join(', ')}</span>)
    }
    return h('div', template)
  },
  width: 120,
}, {
  title: () => $gettext('Node Group'),
  dataIndex: 'env_group_id',
  customRender: actualValueRender('env_group.name'),
  edit: {
    type: selector,
    selector: {
      api: env_group,
      columns: envGroupColumns,
      recordValueIndex: 'name',
      selectionType: 'radio',
    },
  },
  sorter: true,
  pithy: true,
  batch: true,
  width: 100,
}, {
  title: () => $gettext('Status'),
  dataIndex: 'enabled',
  customRender: (args: CustomRender) => {
    const template: JSXElements = []
    const { text } = args
    if (text === true || text > 0) {
      template.push(<Badge status="success" />)
      template.push($gettext('Enabled'))
    }
    else {
      template.push(<Badge status="warning" />)
      template.push($gettext('Disabled'))
    }

    return h('div', template)
  },
  search: {
    type: select,
    mask: {
      true: $gettext('Enabled'),
      false: $gettext('Disabled'),
    },
  },
  sorter: true,
  pithy: true,
  width: 80,
}, {
  title: () => $gettext('Updated at'),
  dataIndex: 'modified_at',
  customRender: datetime,
  sorter: true,
  pithy: true,
  width: 150,
}, {
  title: () => $gettext('Action'),
  dataIndex: 'action',
  width: 80,
  fixed: 'right',
}]

export default columns
