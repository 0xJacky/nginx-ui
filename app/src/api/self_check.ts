import type { Container } from '@/language'
import type { CosyError } from '@/lib/http'
import { http } from '@uozi-admin/request'

export const ReportStatus = {
  Success: 'success',
  Warning: 'warning',
  Error: 'error',
} as const

export type ReportStatusType = typeof ReportStatus[keyof typeof ReportStatus]

export interface SelfCheckAccessOptions {
  installSecret?: string
  setupAuth?: boolean
  debugMode?: 'frontend'
}

export interface TaskReport {
  key: string
  name: Container
  description: Container
  fixable?: boolean
  err?: CosyError
  status: ReportStatusType
}

function installSecretHeaders(installSecret?: string) {
  if (!installSecret) {
    return undefined
  }

  return {
    'X-Install-Secret': installSecret,
  }
}

function getSelfCheckPath(path: string = '', options?: SelfCheckAccessOptions) {
  const basePath = options?.setupAuth ? '/setup/self_check' : '/self_check'

  if (!path) {
    return basePath
  }

  return `${basePath}/${path}`
}

function getRequestOptions(options?: SelfCheckAccessOptions) {
  return {
    headers: options?.setupAuth ? installSecretHeaders(options.installSecret) : undefined,
    skipAuthRedirect: !!options?.setupAuth,
  }
}

const selfCheck = {
  run(options?: SelfCheckAccessOptions): Promise<TaskReport[]> {
    return http.get(getSelfCheckPath('', options), getRequestOptions(options))
  },
  fix(taskName: string, options?: SelfCheckAccessOptions) {
    return http.post(getSelfCheckPath(`${taskName}/fix`, options), undefined, getRequestOptions(options))
  },
  getWebsocketUrl(options?: SelfCheckAccessOptions) {
    return options?.setupAuth ? '/api/setup/self_check/websocket' : '/api/self_check/websocket'
  },
  getWebsocketQuery(options?: SelfCheckAccessOptions) {
    if (!options?.setupAuth || !options.installSecret) {
      return {}
    }

    return {
      install_secret: options.installSecret,
    }
  },
  timeoutCheck(options?: SelfCheckAccessOptions) {
    return http.get(getSelfCheckPath('timeout', options), getRequestOptions(options))
  },
}

export default selfCheck
