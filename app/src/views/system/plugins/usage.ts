import type { PluginUsage } from '@/api/plugin'

export interface UsagePreview {
  /** The first names, at most `max` of them. */
  names: string[]
  /** How many more depend on the plugin beyond the listed names. */
  more: number
}

/** The first few names of a usage report and how many are left over. */
export function previewUsage(usage: PluginUsage, max = 3): UsagePreview {
  const names = usage.items.slice(0, max).map(item => item.name).filter(Boolean)
  return { names, more: Math.max(0, usage.total - names.length) }
}

/** The preview as one line, e.g. "a, b, c and 2 more". */
export function formatUsagePreview(preview: UsagePreview): string {
  // Chinese and Japanese separate list items with an ideographic comma.
  const listed = preview.names.join($pgettext('list separator', ', '))
  if (preview.more === 0)
    return listed
  const more = $gettext('and %{n} more', { n: String(preview.more) })
  return listed ? `${listed} ${more}` : more
}

/** Sentence that says how many certificates renew through the plugin. */
export function certificateUsageText(count: number): string {
  return $ngettext(
    '%{count} certificate renews through it and will not renew while it is off:',
    '%{count} certificates renew through it and will not renew while it is off:',
    count,
    { count: String(count) },
  )
}
