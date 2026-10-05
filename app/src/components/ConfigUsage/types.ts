import type { ConfigStatus } from '@/constants'

export type ConfigUsageKind = 'site' | 'stream' | 'config'

export interface ConfigUsageItem {
  /** Unique key of the file, such as its path. */
  key: string
  label: string
  kind: ConfigUsageKind
  /** The page that edits the file. */
  to: string
  /** Set for sites and streams whose status is known. */
  status?: ConfigStatus
}

export const usageColor: Record<ConfigUsageKind, string> = { site: 'blue', stream: 'purple', config: 'default' }
