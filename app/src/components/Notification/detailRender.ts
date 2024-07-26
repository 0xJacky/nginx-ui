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
    case 'Sync Rename Configuration Success':
      return syncRenameConfigSuccess(args.text)
    case 'Sync Rename Configuration Error':
      return syncRenameConfigError(args.text)
    case 'Sync Configuration Success':
      return syncConfigSuccess(args.text)
    case 'Sync Configuration Error':
      return syncConfigError(args.text)
    default:
      return args.text
  }
}
