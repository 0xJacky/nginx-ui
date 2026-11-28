import type { NotificationLevel } from '@/api/external_notify'

export const notificationLevels: NotificationLevel[] = ['info', 'success', 'warning', 'error']

/**
 * Returns the levels a notifier receives. The server sends every level to a
 * notifier without a selection, so null and undefined mean all of them.
 */
export function receivedLevels(levels?: NotificationLevel[] | null): NotificationLevel[] {
  return levels ?? notificationLevels
}

export function notificationLevelOptions() {
  return [
    {
      label: $gettext('Info'),
      value: 'info',
    },
    {
      label: $gettext('Success'),
      value: 'success',
    },
    {
      label: $gettext('Warning'),
      value: 'warning',
    },
    {
      label: $gettext('Error'),
      value: 'error',
    },
  ] satisfies { label: string, value: NotificationLevel }[]
}
