import type { CustomRenderArgs, StdTableColumn } from '@uozi-admin/curd'
import type { JSXElements } from '@/types'
import { datetimeRender, maskRender } from '@uozi-admin/curd'
import { Badge, Tag, Tooltip } from 'ant-design-vue'
import dayjs from 'dayjs'
import { PrivateKeyTypeMask } from '@/constants'

const columns: StdTableColumn[] = [{
  title: () => $gettext('Name'),
  dataIndex: 'name',
  sorter: true,
  pure: true,
  customRender: (args: CustomRenderArgs) => {
    const { text, record } = args
    if (!text)
      return h('div', record.domain)

    return h('div', text)
  },
  search: {
    type: 'input',
  },
}, {
  title: () => $gettext('Type'),
  dataIndex: 'auto_cert',
  customRender: ({ text }: CustomRenderArgs) => {
    const template: JSXElements = []
    const sync = $gettext('Sync Certificate')
    const managed = $gettext('Managed Certificate')
    const general = $gettext('General Certificate')
    if (text === true || text === 1) {
      template.push(
        <Tag bordered={false} color="processing">
          {managed}
        </Tag>,
      )
    }
    else if (text === 2) {
      template.push(
        <Tag bordered={false} color="success">
          {sync}
        </Tag>,
      )
    }
    else {
      template.push(
        <Tag bordered={false} color="purple">
          {general}
        </Tag>,
      )
    }
    return h('div', template)
  },
  sorter: true,
  pure: true,
}, {
  title: () => $gettext('Key Type'),
  dataIndex: 'key_type',
  customRender: maskRender(PrivateKeyTypeMask),
  sorter: true,
  pure: true,
}, {
  title: () => $gettext('Status'),
  dataIndex: 'status',
  pure: true,
  customRender: (args: CustomRenderArgs) => {
    const { record } = args
    if (record.status === 'pending') {
      return h('div', [
        h(Badge, { status: 'processing' }),
        h('span', $gettext('Issuing...')),
      ])
    }
    if (record.status === 'failure') {
      const errorMsg = record.last_error || $gettext('Issuance failed')
      return h(Tooltip, { title: errorMsg }, () =>
        h('div', [
          h(Badge, { status: 'error' }),
          h('span', $gettext('Failed')),
        ]))
    }
    const info = record.certificate_info
    const valid = info?.not_before
      && info?.not_after
      && !dayjs().isBefore(info.not_before)
      && !dayjs().isAfter(info.not_after)
    if (valid) {
      return h('div', [
        h(Badge, { status: 'success' }),
        h('span', $gettext('Valid')),
      ])
    }
    return h('div', [
      h(Badge, { status: 'error' }),
      h('span', $gettext('Expired')),
    ])
  },
}, {
  title: () => $gettext('Not After'),
  dataIndex: ['certificate_info', 'not_after'],
  customRender: datetimeRender,
  sorter: true,
  pure: true,
}, {
  title: () => $gettext('Actions'),
  dataIndex: 'actions',
  fixed: 'right',
}]

export default columns
