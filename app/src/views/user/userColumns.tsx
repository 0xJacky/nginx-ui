import type { CustomRenderArgs, StdTableColumn } from '@uozi-admin/curd'
import type { JSXElements } from '@/types'
import { datetimeRender } from '@uozi-admin/curd'
import { Tag } from 'ant-design-vue'

const columns: StdTableColumn[] = [{
  title: () => $gettext('Username'),
  dataIndex: 'name',
  sorter: true,
  pure: true,
  edit: {
    type: 'input',
  },
  search: true,
}, {
  title: () => $gettext('Password'),
  dataIndex: 'password',
  sorter: true,
  pure: true,
  edit: {
    type: 'password',
    password: {
      placeholder: $gettext('Leave blank for no change'),
      generate: true,
    },
  },
  hiddenInTable: true,
  hiddenInDetail: true,
}, {
  title: () => $gettext('2FA'),
  dataIndex: 'enabled_2fa',
  customRender: (args: CustomRenderArgs) => {
    const template: JSXElements = []
    const { text } = args
    if (text === true || text > 0)
      template.push(<Tag color="green">{$gettext('Enabled')}</Tag>)

    else
      template.push(<Tag color="orange">{$gettext('Disabled')}</Tag>)

    return h('div', template)
  },
  sorter: true,
  pure: true,
}, {
  title: () => $gettext('Created at'),
  dataIndex: 'created_at',
  customRender: datetimeRender,
  sorter: true,
  pure: true,
}, {
  title: () => $gettext('Updated at'),
  dataIndex: 'updated_at',
  customRender: datetimeRender,
  sorter: true,
  pure: true,
}, {
  title: () => $gettext('Actions'),
  dataIndex: 'actions',
  fixed: 'right',
  width: 250,
}]

export default columns
