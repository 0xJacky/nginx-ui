import type { CustomRenderArgs } from '@uozi-admin/curd'
import { NotificationTypeT } from '@/constants'
import notifications from './notifications'

export function detailRender(args: CustomRenderArgs) {
  try {
    return (
      <div>
        <div class="mb-2">
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
