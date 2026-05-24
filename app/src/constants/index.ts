export const DATE_FORMAT = 'YYYY-MM-DD'

export enum ConfigStatus {
  Enabled = 'enabled',
  Disabled = 'disabled',
  Maintenance = 'maintenance',
}

export enum AutoCertState {
  Disable = -1,
  Enable = 1,
  Sync = 2,
  SelfSigned = 3,
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

export const DeployModeMask = {
  local: () => $gettext('Local'),
  remote: () => $gettext('Remote'),
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

// Maps both legacy frontend keys (P256, 2048…) and canonical backend values
// (EC256, RSA2048…) to a single form key. The backend's helper.GetKeyType
// normalizes to the canonical form on write, so cert.key_type read from the
// API may be either form depending on when the row was written.
const PrivateKeyTypeAliasMap: Record<string, string> = {
  2048: '2048',
  RSA2048: '2048',
  3072: '3072',
  RSA3072: '3072',
  4096: '4096',
  RSA4096: '4096',
  8192: '8192',
  RSA8192: '8192',
  P256: 'P256',
  EC256: 'P256',
  P384: 'P384',
  EC384: 'P384',
}

// normalizePrivateKeyType collapses any accepted key_type form to the
// legacy frontend key, so it matches PrivateKeyTypeEnum / form ASelect
// options. Unknown values pass through unchanged.
export function normalizePrivateKeyType(value: string | undefined | null): string {
  if (!value)
    return ''
  return PrivateKeyTypeAliasMap[value] ?? value
}

// formatPrivateKeyType returns the display label for any accepted form,
// falling back to '/' so table cells stay aligned with maskRender output
// when the value is missing or unknown.
export function formatPrivateKeyType(value: string | undefined | null): string {
  if (!value)
    return '/'
  const normalized = PrivateKeyTypeAliasMap[value]
  if (!normalized)
    return '/'
  return PrivateKeyTypeMask[normalized as PrivateKeyType]
}
