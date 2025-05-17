import type { FrontendTask } from '../types'
import type { ReportStatusType } from '@/api/self_check'
import { ReportStatus } from '@/api/self_check'

/**
 * HTTPS Check Task
 *
 * Checks if the application is accessed via HTTPS protocol
 * Warns (not errors) when HTTP is used outside of localhost/127.0.0.1
 */
const HttpsCheckTask: FrontendTask = {
  key: 'https-check',
  name: () => $gettext('HTTPS Protocol'),
  description: () => $gettext('Check if HTTPS is enabled. Using HTTP outside localhost is insecure and prevents using Passkeys and clipboard features'),
  check: async (): Promise<ReportStatusType> => {
    // Get current protocol and hostname
    const isSecure = window.location.protocol === 'https:'
    const isLocalhost = ['localhost', '127.0.0.1'].includes(window.location.hostname)
    // Check result
    if (isSecure || isLocalhost) {
      return ReportStatus.Success
    }
    // Return warning for non-localhost HTTP
    return ReportStatus.Warning
  },
}

export default HttpsCheckTask
