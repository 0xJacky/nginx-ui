import type { CustomRenderProps } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import type { Column, JSXElements } from '@/components/StdDesign/types'
import { datetime, mask } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import { input } from '@/components/StdDesign/StdDataEntry'
import { PrivateKeyTypeMask } from '@/constants'
import { Badge, Tag } from 'ant-design-vue'
import dayjs from 'dayjs'

const columns: Column[] = [{
  title: () => $gettext('Name'),
  dataIndex: 'name',
  sorter: true,
  pithy: true,
  customRender: (args: CustomRenderProps) => {
    const { text, record } = args
    if (!text)
      return h('div', record.domain)

    return h('div', text)
  },
  search: {
    type: input,
  },
}, {
  title: () => $gettext('Type'),
  dataIndex: 'auto_cert',
  customRender: (args: CustomRenderProps) => {
    const template: JSXElements = []
    const { text } = args
    const sync = $gettext('Sync Certificate')
    const managed = $gettext('Managed Certificate')
    const general = $gettext('General Certificate')
    if (text === true || text === 1) {
      template.push(
        <Tag bordered={false} color="processing">
          { managed }
        </Tag>,
      )
    }
    else if (text === 2) {
      template.push(
        <Tag bordered={false} color="success">
          { sync }
        </Tag>,
      )
    }
    else {
      template.push(
        <Tag bordered={false} color="purple">
          {
            general
          }
        </Tag>,
      )
    }

    return h('div', template)
  },
  sorter: true,
  pithy: true,
}, {
  title: () => $gettext('Key Type'),
  dataIndex: 'key_type',
  customRender: mask(PrivateKeyTypeMask),
  sorter: true,
  pithy: true,
}, {
  title: () => $gettext('Status'),
  dataIndex: 'certificate_info',
  pithy: true,
  customRender: (args: CustomRenderProps) => {
    const template: JSXElements = []

    const text = args.text?.not_before
      && args.text?.not_after
      && !dayjs().isBefore(args.text?.not_before)
      && !dayjs().isAfter(args.text?.not_after)

    if (text) {
      template.push(<Badge status="success" />)
      template.push($gettext('Valid'))
    }
    else {
      template.push(<Badge status="error" />)
      template.push($gettext('Expired'))
    }

    return h('div', template)
  },
}, {
  title: () => $gettext('Not After'),
  dataIndex: ['certificate_info', 'not_after'],
  customRender: datetime,
  sorter: true,
  pithy: true,
}, {
  title: () => $gettext('Action'),
  dataIndex: 'action',
}]

export default columns
