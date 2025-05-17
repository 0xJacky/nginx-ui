import type { SiteStatus } from '@/api/site'
import type {
  CustomRender,
} from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import type { Column, JSXElements } from '@/components/StdDesign/types'
import { Tag } from 'ant-design-vue'
import env_group from '@/api/env_group'
import {
  actualValueRender,
  datetime,
} from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import { input, select, selector } from '@/components/StdDesign/StdDataEntry'
import { ConfigStatus } from '@/constants'
import envGroupColumns from '@/views/environments/group/columns'
import SiteStatusSegmented from '@/views/site/components/SiteStatusSegmented.vue'

const columns: Column[] = [{
  title: () => $gettext('Name'),
  dataIndex: 'name',
  sorter: true,
  pithy: true,
  edit: {
    type: input,
  },
  search: true,
  width: 150,
  customRender: ({ text, record }) => {
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
  title: () => $gettext('Updated at'),
  dataIndex: 'modified_at',
  customRender: datetime,
  sorter: true,
  pithy: true,
  width: 150,
}, {
  title: () => $gettext('Status'),
  dataIndex: 'status',
  customRender: (args: CustomRender) => {
    const { text, record } = args
    return h(SiteStatusSegmented, {
      'modelValue': text,
      'siteName': record.name,
      'enabled': record.status !== ConfigStatus.Disabled,
      'onUpdate:modelValue': (val: string) => {
        // This will be handled by the component internal events
        record.status = val
      },
      'onStatusChanged': ({ status }: { status: SiteStatus }) => {
        record.status = status
      },
    })
  },
  search: {
    type: select,
    mask: {
      [ConfigStatus.Enabled]: $gettext('Enabled'),
      [ConfigStatus.Disabled]: $gettext('Disabled'),
      [ConfigStatus.Maintenance]: $gettext('Maintenance'),
    },
  },
  sorter: true,
  pithy: true,
  width: 110,
  fixed: 'right',
}, {
  title: () => $gettext('Action'),
  dataIndex: 'action',
  width: 80,
  fixed: 'right',
}]

export default columns
