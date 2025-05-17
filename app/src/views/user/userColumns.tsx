import type { CustomRender } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import type { Column, JSXElements } from '@/components/StdDesign/types'
import { Tag } from 'ant-design-vue'
import { h } from 'vue'
import { datetime } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import { input, password } from '@/components/StdDesign/StdDataEntry'

const columns: Column[] = [{
  title: () => $gettext('Username'),
  dataIndex: 'name',
  sorter: true,
  pithy: true,
  edit: {
    type: input,
  },
  search: true,
}, {
  title: () => $gettext('Password'),
  dataIndex: 'password',
  sorter: true,
  pithy: true,
  edit: {
    type: password,
    config: {
      placeholder: () => $gettext('Leave blank for no change'),
      generate: true,
    },
  },
  hiddenInTable: true,
  hiddenInDetail: true,
}, {
  title: () => $gettext('2FA'),
  dataIndex: 'enabled_2fa',
  customRender: (args: CustomRender) => {
    const template: JSXElements = []
    const { text } = args
    if (text === true || text > 0)
      template.push(<Tag color="green">{$gettext('Enabled')}</Tag>)

    else
      template.push(<Tag color="orange">{$gettext('Disabled')}</Tag>)

    return h('div', template)
  },
  sorter: true,
  pithy: true,
}, {
  title: () => $gettext('Created at'),
  dataIndex: 'created_at',
  customRender: datetime,
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
  fixed: 'right',
}]

export default columns
