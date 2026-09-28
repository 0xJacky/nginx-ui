import { ConfigStatus } from '@/constants'

// Status colors shared by the interactive status select and the read-only
// status tag, so a site or stream looks the same wherever its status shows.
export const configStatusColors: Record<ConfigStatus, string> = {
  [ConfigStatus.Enabled]: '#1890ff',
  [ConfigStatus.Disabled]: '#ff4d4f',
  [ConfigStatus.Maintenance]: '#faad14',
}

export function configStatusLabel(status: ConfigStatus): string {
  const labels: Record<ConfigStatus, string> = {
    [ConfigStatus.Enabled]: $gettext('Enabled'),
    [ConfigStatus.Disabled]: $gettext('Disabled'),
    [ConfigStatus.Maintenance]: $gettext('Maintenance'),
  }
  return labels[status] ?? status
}
