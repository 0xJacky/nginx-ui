// Site health check status constants
export const SiteStatus = {
  ONLINE: 'online',
  OFFLINE: 'offline',
  ERROR: 'error',
  CHECKING: 'checking',
} as const

// Type for site status
export type SiteStatusType = typeof SiteStatus[keyof typeof SiteStatus]

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
