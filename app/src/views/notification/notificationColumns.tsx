import type { CustomRender } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import type { Column } from '@/components/StdDesign/types'
import { Tag } from 'ant-design-vue'
import { detailRender } from '@/components/Notification/detailRender'
import { datetime } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import { NotificationTypeT } from '@/constants'

const columns: Column[] = [{
  title: () => $gettext('Type'),
  dataIndex: 'type',
  customRender: (args: CustomRender) => {
    if (args.text === NotificationTypeT.Error) {
      return (
        <Tag color="error">
          {$gettext('Error')}
        </Tag>
      )
    }
    else if (args.text === NotificationTypeT.Warning) {
      return (
        <Tag color="warning">
          {$gettext('Warning')}
        </Tag>
      )
    }
    else if (args.text === NotificationTypeT.Info) {
      return (
        <Tag color="blue">
          {$gettext('Info')}
        </Tag>
      )
    }
    else if (args.text === NotificationTypeT.Success) {
      return (
        <Tag color="success">
          {$gettext('Success')}
        </Tag>
      )
    }
  },
  sorter: true,
  pithy: true,
  width: 100,
}, {
  title: () => $gettext('Created at'),
  dataIndex: 'created_at',
  sorter: true,
  customRender: datetime,
  pithy: true,
  width: 180,
}, {
  title: () => $gettext('Title'),
  dataIndex: 'title',
  customRender: (args: CustomRender) => {
    return h('span', $gettext(args.text))
  },
  pithy: true,
  width: 250,
}, {
  title: () => $gettext('Details'),
  dataIndex: 'details',
  customRender: detailRender,
  pithy: true,
  width: 500,
}, {
  title: () => $gettext('Action'),
  dataIndex: 'action',
  fixed: 'right',
}]

export default columns
