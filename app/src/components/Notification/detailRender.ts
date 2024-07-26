import type { customRender } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import { syncCertificateError, syncCertificateSuccess } from '@/components/Notification/cert'
import {
  syncConfigError,
  syncConfigSuccess,
  syncRenameConfigError,
  syncRenameConfigSuccess,
} from '@/components/Notification/config'

export const detailRender = (args: customRender) => {
  switch (args.record.title) {
    case 'Sync Certificate Success':
      return syncCertificateSuccess(args.text)
    case 'Sync Certificate Error':
      return syncCertificateError(args.text)
    case 'Rename Remote Config Success':
      return syncRenameConfigSuccess(args.text)
    case 'Rename Remote Config Error':
      return syncRenameConfigError(args.text)
    case 'Sync Config Success':
      return syncConfigSuccess(args.text)
    case 'Sync Config Error':
      return syncConfigError(args.text)
    default:
      return args.text
  }
}
