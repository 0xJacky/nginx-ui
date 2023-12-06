import gettext from '@/gettext'

const { $gettext } = gettext
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
