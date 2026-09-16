import type { CustomRenderArgs, StdTableColumn } from '@uozi-admin/curd'
import { datetimeRender } from '@uozi-admin/curd'
import { Tag } from 'antdv-next'
import { detailRender } from '@/components/Notification/detailRender'
import { NotificationType, NotificationTypeT } from '@/constants'

function getNotificationLink(args: CustomRenderArgs): string {
  const record = args.record as {
    url?: string | null
    details?: Record<string, unknown> | string | null
  }

  if (typeof record?.url === 'string' && record.url.trim())
    return record.url

  if (record?.details && typeof record.details === 'object') {
    const details = record.details as Record<string, unknown>
    const url = details.url
    if (typeof url === 'string' && url.trim())
      return url
  }

  return ''
}

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
  search: {
    type: 'select',
    select: {
      mask: NotificationType,
    },
  },
  sorter: true,
  pure: true,
  width: 90,
}, {
  title: () => $gettext('Created at'),
  dataIndex: 'created_at',
  sorter: true,
  customRender: datetimeRender,
  pure: true,
  width: 170,
}, {
  title: () => $gettext('Title'),
  dataIndex: 'title',
  customRender: (args: CustomRenderArgs) => {
    return h('span', $gettext(args.text))
  },
  pure: true,
  width: 210,
}, {
  title: () => $gettext('Details'),
  dataIndex: 'details',
  customRender: detailRender,
  pure: true,
  width: 560,
}, {
  title: () => $gettext('Go To'),
  dataIndex: 'jump_to',
  customRender: (args: CustomRenderArgs) => {
    const url = getNotificationLink(args)

    if (!url)
      return null

    return h('a', {
      href: url,
    }, $gettext('View'))
  },
  pure: true,
  width: 80,
}, {
  title: () => $gettext('Actions'),
  dataIndex: 'actions',
  fixed: 'right',
  width: 160,
}]

export default columns
