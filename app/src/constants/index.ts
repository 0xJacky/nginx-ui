export const DATE_FORMAT = 'YYYY-MM-DD'

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
