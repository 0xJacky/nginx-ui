import type { CustomRenderArgs, StdTableColumn } from '@uozi-admin/curd'
import { datetimeRender } from '@uozi-admin/curd'
import { Tag } from 'ant-design-vue'
import { detailRender } from '@/components/Notification/detailRender'
import { NotificationTypeT } from '@/constants'

const columns: StdTableColumn[] = [{
  title: () => $gettext('Type'),
  dataIndex: 'type',
  customRender: (args: CustomRenderArgs) => {
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
    return args.text
  },
  sorter: true,
  pure: true,
  width: 100,
}, {
  title: () => $gettext('Created at'),
  dataIndex: 'created_at',
  sorter: true,
  customRender: datetimeRender,
  pure: true,
  width: 180,
}, {
  title: () => $gettext('Title'),
  dataIndex: 'title',
  customRender: (args: CustomRenderArgs) => {
    return h('span', $gettext(args.text))
  },
  pure: true,
  width: 250,
}, {
  title: () => $gettext('Details'),
  dataIndex: 'details',
  customRender: detailRender,
  pure: true,
  width: 500,
}, {
  title: () => $gettext('Actions'),
  dataIndex: 'actions',
  fixed: 'right',
  width: 200,
}]

export default columns
