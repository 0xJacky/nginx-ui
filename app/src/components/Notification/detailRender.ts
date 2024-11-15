import type { CustomRender } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import { syncCertificateError, syncCertificateSuccess } from '@/components/Notification/cert'
import {
  deleteSiteError,
  deleteSiteSuccess,
  disableSiteError,
  disableSiteSuccess,
  enableSiteError,
  enableSiteSuccess,
  renameSiteError,
  renameSiteSuccess,
  saveSiteError,
  saveSiteSuccess,
  syncConfigError,
  syncConfigSuccess,
  syncRenameConfigError,
  syncRenameConfigSuccess,
} from '@/components/Notification/config'

export function detailRender(args: CustomRender) {
  try {
    switch (args.record.title) {
      case 'Sync Certificate Success':
        return syncCertificateSuccess(args.text)
      case 'Sync Certificate Error':
        return syncCertificateError(args.text)
      case 'Rename Remote Config Success':
        return syncRenameConfigSuccess(args.text)
      case 'Rename Remote Config Error':
        return syncRenameConfigError(args.text)

      case 'Save Remote Site Success':
        return saveSiteSuccess(args.text)
      case 'Save Remote Site Error':
        return saveSiteError(args.text)
      case 'Delete Remote Site Success':
        return deleteSiteSuccess(args.text)
      case 'Delete Remote Site Error':
        return deleteSiteError(args.text)
      case 'Enable Remote Site Success':
        return enableSiteSuccess(args.text)
      case 'Enable Remote Site Error':
        return enableSiteError(args.text)
      case 'Disable Remote Site Success':
        return disableSiteSuccess(args.text)
      case 'Disable Remote Site Error':
        return disableSiteError(args.text)
      case 'Rename Remote Site Success':
        return renameSiteSuccess(args.text)
      case 'Rename Remote Site Error':
        return renameSiteError(args.text)

      case 'Sync Config Success':
        return syncConfigSuccess(args.text)
      case 'Sync Config Error':
        return syncConfigError(args.text)
      default:
        return args.text
    }
  }
  // eslint-disable-next-line sonarjs/no-ignored-exceptions
  catch (e) {
    return args.text
  }
}
