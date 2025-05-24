import type {
  CustomRenderArgs,
  StdTableColumn,
} from '@uozi-admin/curd'
import type { Site, SiteStatus } from '@/api/site'
import type { JSXElements } from '@/types'
import { actualFieldRender, datetimeRender } from '@uozi-admin/curd'
import { Tag } from 'ant-design-vue'
import env_group from '@/api/env_group'
import { ConfigStatus } from '@/constants'
import envGroupColumns from '@/views/environments/group/columns'
import SiteStatusSelect from '@/views/site/components/SiteStatusSelect.vue'

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
  customRender: ({ text, record }: CustomRenderArgs) => {
    const template: JSXElements = []

    // Add site name
    template.push(
      <div style="margin-bottom: 8px;">{text}</div>,
    )

    // Add URLs below the name
    if (record.urls && record.urls.length > 0) {
      const urlsContainer: JSXElements = []

      if (record.status !== ConfigStatus.Disabled) {
        record.urls.forEach((url: string) => {
          const displayUrl = url.replace(/^https?:\/\//, '')
          urlsContainer.push(
            <a href={url} target="_blank" rel="noopener noreferrer">
              <Tag color="blue" bordered={false} style="margin-right: 8px; margin-bottom: 4px;">
                {displayUrl}
              </Tag>
            </a>,
          )
        })
      }
      else {
        record.urls.forEach((url: string) => {
          const displayUrl = url.replace(/^https?:\/\//, '')
          urlsContainer.push(<Tag bordered={false} style="margin-right: 8px; margin-bottom: 4px;">{displayUrl}</Tag>)
        })
      }

      template.push(
        <div style="display: flex; flex-wrap: wrap;">{urlsContainer}</div>,
      )
    }

    return h('div', {}, template)
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
  customRender: (args: CustomRenderArgs<Site>) => {
    const { text, record } = args
    return h(SiteStatusSelect, {
      'modelValue': text,
      'siteName': record.name,
      'enabled': record.status !== ConfigStatus.Disabled,
      'onUpdate:modelValue': (val: string) => {
        // This will be handled by the component internal events
        record.status = val as SiteStatus
      },
      'onStatusChanged': ({ status }: { status: SiteStatus }) => {
        record.status = status
      },
    })
  },
  search: {
    type: 'select',
    select: {
      options: [
        {
          label: $gettext('Enabled'),
          value: ConfigStatus.Enabled,
        },
        {
          label: $gettext('Disabled'),
          value: ConfigStatus.Disabled,
        },
        {
          label: $gettext('Maintenance'),
          value: ConfigStatus.Maintenance,
        },
      ],
    },
  },
  sorter: true,
  pure: true,
  width: 50,
  fixed: 'right',
}, {
  title: () => $gettext('Actions'),
  dataIndex: 'actions',
  width: 80,
  fixed: 'right',
}]

export default columns
