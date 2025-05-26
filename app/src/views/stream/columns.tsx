import type { CustomRenderArgs, StdTableColumn } from '@uozi-admin/curd'
import type { SiteStatus } from '@/api/site'
import type { Stream } from '@/api/stream'
import type { JSXElements } from '@/types'
import { actualFieldRender, datetimeRender } from '@uozi-admin/curd'
import env_group from '@/api/env_group'
import ProxyTargets from '@/components/ProxyTargets'
import envGroupColumns from '@/views/environments/group/columns'
import StreamStatusSelect from '@/views/stream/components/StreamStatusSelect.vue'

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
  customRender: ({ text }: CustomRenderArgs<Stream>) => {
    const template: JSXElements = []

    // Add stream name
    template.push(
      <div style="margin-bottom: 8px;">{text}</div>,
    )

    return h('div', {}, template)
  },
}, {
  title: () => $gettext('Proxy Targets'),
  dataIndex: 'proxy_targets',
  width: 200,
  customRender: ({ record }: CustomRenderArgs<Stream>) => {
    if (record.proxy_targets && record.proxy_targets.length > 0) {
      return h(ProxyTargets, {
        targets: record.proxy_targets,
      })
    }
    return h('span', '-')
  },
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
  batchEdit: true,
  sorter: true,
  pure: true,
  width: 100,
}, {
  title: () => $gettext('Updated at'),
  dataIndex: 'modified_at',
  customRender: datetimeRender,
  sorter: true,
  pure: true,
  width: 150,
}, {
  title: () => $gettext('Status'),
  dataIndex: 'status',
  customRender: (args: CustomRenderArgs<Stream>) => {
    const { record } = args
    return h(StreamStatusSelect, {
      'status': record.status,
      'streamName': record.name,
      'onStatusChanged': ({ status }: { status: SiteStatus }) => {
        record.status = status
      },
      'onUpdate:status': (val?: SiteStatus) => {
        // This will be handled by the component internal events
        record.status = val!
      },
    })
  },
  search: {
    type: 'select',
    select: {
      options: [
        {
          label: $gettext('Enabled'),
          value: 'enabled',
        },
        {
          label: $gettext('Disabled'),
          value: 'disabled',
        },
      ],
    },
  },
  sorter: true,
  pure: true,
  width: 100,
  fixed: 'right',
}, {
  title: () => $gettext('Actions'),
  dataIndex: 'actions',
  width: 80,
  fixed: 'right',
}]

export default columns
