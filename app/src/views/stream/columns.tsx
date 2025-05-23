import type { CustomRenderArgs, StdTableColumn } from '@uozi-admin/curd'
import type { JSXElements } from '@/types'
import { actualFieldRender, datetimeRender } from '@uozi-admin/curd'
import { Badge } from 'ant-design-vue'
import env_group from '@/api/env_group'
import { ConfigStatus } from '@/constants'
import envGroupColumns from '@/views/environments/group/columns'

const columns: StdTableColumn[] = [{
  title: () => $gettext('Name'),
  dataIndex: 'name',
  sorter: true,
  pure: true,
  edit: {
    type: 'input',
  },
  search: true,
  width: 150,
}, {
  title: () => $gettext('Node Group'),
  dataIndex: 'env_group_id',
  customRender: actualFieldRender('env_group.name'),
  edit: {
    type: 'selector',
    selector: {
      getListApi: env_group.getList,
      columns: envGroupColumns,
      valueKey: 'id',
      displayKey: 'name',
      selectionType: 'radio',
    },
  },
  sorter: true,
  pure: true,
  width: 150,
}, {
  title: () => $gettext('Status'),
  dataIndex: 'status',
  customRender: (args: CustomRenderArgs) => {
    const template: JSXElements = []
    const { text } = args
    if (text === ConfigStatus.Enabled) {
      template.push(<Badge status="success" />)
      template.push(h('span', $gettext('Enabled')))
    }
    else if (text === ConfigStatus.Disabled) {
      template.push(<Badge status="warning" />)
      template.push(h('span', $gettext('Disabled')))
    }

    return h('div', template)
  },
  sorter: true,
  pure: true,
  width: 200,
}, {
  title: () => $gettext('Updated at'),
  dataIndex: 'modified_at',
  customRender: datetimeRender,
  sorter: true,
  pure: true,
  width: 200,
}, {
  title: () => $gettext('Actions'),
  dataIndex: 'actions',
  width: 250,
  fixed: 'right',
}]

export default columns
