import type { CustomRenderArgs, StdTableColumn } from '@uozi-admin/curd'
import type { JSXElements } from '@/types'
import { datetimeRender, maskRender } from '@uozi-admin/curd'
import { Badge, Tag } from 'ant-design-vue'
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
  dataIndex: 'certificate_info',
  pure: true,
  customRender: (args: CustomRenderArgs) => {
    const template: JSXElements = []

    const text = args.text?.not_before
      && args.text?.not_after
      && !dayjs().isBefore(args.text?.not_before)
      && !dayjs().isAfter(args.text?.not_after)

    if (text) {
      template.push(<Badge status="success" />)
      template.push(h('span', $gettext('Valid')))
    }
    else {
      template.push(<Badge status="error" />)
      template.push(h('span', $gettext('Expired')))
    }

    return h('div', template)
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
