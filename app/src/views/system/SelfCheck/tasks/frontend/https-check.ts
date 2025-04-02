import type { FrontendTask, TaskReport } from '../types'

/**
 * HTTPS Check Task
 *
 * Checks if the application is accessed via HTTPS protocol
 * Warns (not errors) when HTTP is used outside of localhost/127.0.0.1
 */
const HttpsCheckTask: FrontendTask = {
  name: () => $gettext('HTTPS Protocol'),
  description: () => $gettext('Check if HTTPS is enabled. Using HTTP outside localhost is insecure and prevents using Passkeys and clipboard features.'),
  type: 'frontend',
  check: async (): Promise<TaskReport> => {
    // Get current protocol and hostname
    const isSecure = window.location.protocol === 'https:'
    const isLocalhost = ['localhost', '127.0.0.1'].includes(window.location.hostname)

    // Task name for the report
    const name = 'Frontend-HttpsCheck'

    // Check result
    if (isSecure) {
      return {
        name,
        status: 'success',
        type: 'frontend',
        message: 'HTTPS protocol is enabled.',
      }
    }

    if (isLocalhost) {
      return {
        name,
        status: 'success',
        type: 'frontend',
        message: 'HTTP is acceptable for localhost.',
      }
    }

    // Return warning for non-localhost HTTP
    return {
      name,
      status: 'warning',
      type: 'frontend',
      message: 'HTTP protocol detected. Consider enabling HTTPS for security features.',
      err: new Error('HTTP protocol detected. Consider enabling HTTPS for security features.'),
    }
  },
}

export default HttpsCheckTask
