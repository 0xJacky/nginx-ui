import type { CustomRenderArgs } from '@uozi-admin/curd'
import { NotificationTypeT } from '@/constants'
import notifications from './notifications'

export function detailRender(args: Pick<CustomRenderArgs, 'record' | 'text'>) {
  try {
    return (
      <div>
        <div>
          {
            notifications[args.record.title]?.content(args.record.details)
            || args.record.content || args.record.details
          }
        </div>
        {args.record.details?.response && args.record.type !== NotificationTypeT.Success && (
          <div>
            { JSON.stringify(args.record.details.response) }
          </div>
        )}
      </div>
    )
  }
  catch {
    return args.text
  }
}
