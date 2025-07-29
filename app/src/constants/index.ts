export const DATE_FORMAT = 'YYYY-MM-DD'

export enum ConfigStatus {
  Enabled = 'enabled',
  Disabled = 'disabled',
  Maintenance = 'maintenance',
}

export enum AutoCertState {
  Disable = 0,
  Enable = 1,
}

export enum NotificationTypeT {
  Error,
  Warning,
  Info,
  Success,
}

export const NotificationType = {
  [NotificationTypeT.Error]: () => $gettext('Error'),
  [NotificationTypeT.Warning]: () => $gettext('Warning'),
  [NotificationTypeT.Info]: () => $gettext('Info'),
  [NotificationTypeT.Success]: () => $gettext('Success'),
} as const

export enum NginxStatus {
  Running,
  Reloading,
  Restarting,
  Stopped,
}

export const PostSyncActionMask = {
  none: () => $gettext('No Action'),
  reload_nginx: () => $gettext('Reload Nginx'),
} as const

export const UpstreamTestTypeMask = {
  local: () => $gettext('Local'),
  remote: () => $gettext('Remote'),
  mirror: () => $gettext('Mirror'),
} as const

export const PrivateKeyTypeMask = {
  2048: 'RSA2048',
  3072: 'RSA3072',
  4096: 'RSA4096',
  8192: 'RSA8192',
  P256: 'EC256',
  P384: 'EC384',
} as const

export const PrivateKeyTypeList
    = Object.entries(PrivateKeyTypeMask).map(([key, name]) =>
      ({ key, name }))

export type PrivateKeyType = keyof typeof PrivateKeyTypeMask
export const PrivateKeyTypeEnum = {
  2048: '2048',
  3072: '3072',
  4096: '4096',
  8192: '8192',
  P256: 'P256',
  P384: 'P384',
} as const
