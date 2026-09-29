import type { SettingsField } from '@/api/plugin'

/** Empty values compare equal whatever their shape, so an untouched field stays clean. */
function comparable(value: unknown): string {
  if (value === undefined || value === null || value === '')
    return ''
  if (Array.isArray(value) && value.length === 0)
    return ''
  return JSON.stringify(value)
}

/** Keys of the fields whose value differs from the saved one. */
export function changedSettingKeys(
  fields: SettingsField[],
  values: Record<string, unknown>,
  saved: Record<string, unknown>,
): string[] {
  return fields
    .filter(field => comparable(values[field.key]) !== comparable(saved[field.key]))
    .map(field => field.key)
}

/** A deep copy that keeps the form and the saved snapshot apart. */
export function cloneSettings(values: Record<string, unknown>): Record<string, unknown> {
  return JSON.parse(JSON.stringify(values ?? {})) as Record<string, unknown>
}

/** Text a plugin supplied, translated when the plugin registered a translation for it. */
export function translatePluginText(text: string | undefined): string {
  return text ? $gettext(text) : ''
}
