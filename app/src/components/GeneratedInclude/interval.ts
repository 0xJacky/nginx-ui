/** A refresh interval in seconds as a short human readable duration. */
export function formatInterval(seconds?: number): string {
  if (!seconds)
    return ''
  if (seconds % 86400 === 0)
    return $gettext('%{n} d', { n: String(seconds / 86400) })
  if (seconds % 3600 === 0)
    return $gettext('%{n} h', { n: String(seconds / 3600) })
  if (seconds % 60 === 0)
    return $gettext('%{n} min', { n: String(seconds / 60) })
  return $gettext('%{n} s', { n: String(seconds) })
}
