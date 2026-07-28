import type { CustomRenderArgs, StdTableColumn } from '@uozi-admin/curd'
import type { JSX } from 'vue/jsx-runtime'
import { datetimeRender } from '@uozi-admin/curd'
import { Badge, Tag } from 'ant-design-vue'
import { h } from 'vue'

const columns: StdTableColumn[] = [{
  title: () => $gettext('Name'),
  dataIndex: 'name',
  sorter: true,
  pure: true,
  edit: {
    type: 'input',
  },
  search: true,
  width: 200,
}, {
  title: () => $gettext('URL'),
  dataIndex: 'url',
  sorter: true,
  pure: true,
  edit: {
    type: 'input',
    input: {
      placeholder: 'https://10.0.0.1:9000',
    },
  },
  width: 260,
}, {
  title: () => $gettext('Version'),
  dataIndex: 'version',
  pure: true,
  width: 120,
}, {
  title: () => $gettext('Authentication'),
  dataIndex: 'auth_method',
  customRender: ({ record }: CustomRenderArgs) => {
    const isPaired = record.auth_method === 'paired_ed25519'
    return (
      <div class="flex flex-col gap-1">
        <Tag color={isPaired ? 'green' : 'orange'}>
          {isPaired ? $gettext('Paired signature') : $gettext('Legacy secret')}
        </Tag>
        <span class="text-xs opacity-60">{record.credential_status}</span>
      </div>
    )
  },
  pure: true,
  width: 160,
}, {
  title: () => $gettext('Status'),
  dataIndex: 'status',
  customRender: (args: CustomRenderArgs) => {
    const template: JSX.Element[] = []
    const { text } = args
    if (args.record.enabled) {
      if (text === true || text > 0) {
        template.push(<Badge status="success" />)
        template.push(<span>{$gettext('Online')}</span>)
      }
      else {
        template.push(<Badge status="error" />)
        template.push(<span>{$gettext('Offline')}</span>)
      }
    }
    else {
      template.push(<Badge status="default" />)
      template.push(<span>{$gettext('Disabled')}</span>)
    }

    return h('div', template)
  },
  sorter: true,
  pure: true,
  width: 120,
}, {
  title: () => $gettext('Enabled'),
  dataIndex: 'enabled',
  customRender: (args: CustomRenderArgs) => {
    const template: JSX.Element[] = []
    const { text } = args
    if (text === true || text > 0)
      template.push(<Tag color="green">{$gettext('Enabled')}</Tag>)

    else
      template.push(<Tag color="orange">{$gettext('Disabled')}</Tag>)

    return h('div', template)
  },
  edit: {
    type: 'switch',
  },
  sorter: true,
  pure: true,
  width: 120,
}, {
  title: () => $gettext('Updated at'),
  dataIndex: 'updated_at',
  customRender: datetimeRender,
  sorter: true,
  pure: true,
  width: 150,
}, {
  title: () => $gettext('Actions'),
  dataIndex: 'actions',
  fixed: 'right',
  width: 200,
}]

export default columns
