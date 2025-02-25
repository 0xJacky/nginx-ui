import type { CustomRender } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import { NotificationTypeT } from '@/constants'
import notifications from './notifications'

export function detailRender(args: CustomRender) {
  try {
    return (
      <div>
        <div class="mb-2">
          {
            notifications[args.record.title].content(args.record.details)
          }
        </div>
        {args.record.type !== NotificationTypeT.Success && (
          <div>
            { JSON.stringify(args.record.details.response) }
          </div>
        )}
      </div>
    )
  }
  // eslint-disable-next-line sonarjs/no-ignored-exceptions,unused-imports/no-unused-vars
  catch (e) {
    return args.text
  }
}
