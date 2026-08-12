// Site health check status constants
export const SiteStatus = {
  ONLINE: 'online',
  OFFLINE: 'offline',
  ERROR: 'error',
  CHECKING: 'checking',
} as const

// Type for site status
export type SiteStatusType = typeof SiteStatus[keyof typeof SiteStatus]

// Health check failure categories reported by the backend in `error_type`.
// They let the dashboard explain *why* a site is down instead of surfacing a
// raw Go error string.
export const SiteError = {
  TLS: 'tls',
  DNS: 'dns',
  CONNECTION_REFUSED: 'connection_refused',
  TIMEOUT: 'timeout',
  NETWORK: 'network',
  STATUS_CODE: 'status_code',
  CONTENT: 'content',
  REQUEST: 'request',
} as const

// Type for site error category
export type SiteErrorType = typeof SiteError[keyof typeof SiteError]

// Status display configuration
export const SiteStatusConfig = {
  [SiteStatus.ONLINE]: {
    label: 'Online',
    color: 'success',
    icon: 'CheckCircleOutlined',
  },
  [SiteStatus.OFFLINE]: {
    label: 'Offline',
    color: 'error',
    icon: 'CloseCircleOutlined',
  },
  [SiteStatus.ERROR]: {
    label: 'Error',
    color: 'warning',
    icon: 'ExclamationCircleOutlined',
  },
  [SiteStatus.CHECKING]: {
    label: 'Checking',
    color: 'processing',
    icon: 'SyncOutlined',
  },
} as const
