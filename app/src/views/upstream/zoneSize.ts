// The form edits a shared memory zone size in KB; nginx stores it as a size
// string such as 64k, 1m or a plain byte count.

export const DEFAULT_ZONE_SIZE_KB = 64
export const MIN_ZONE_SIZE_KB = 32

/**
 * Converts an nginx size string into KB for the form, or null when it cannot
 * be read (the backend then reports the invalid value).
 */
export function zoneSizeToKb(size: string | undefined): number | null {
  const match = /^(\d+)([km]?)$/i.exec((size ?? '').trim())
  if (!match)
    return null
  const value = Number(match[1])
  switch (match[2].toLowerCase()) {
    case 'm':
      return value * 1024
    case 'k':
      return value
    default:
      return Math.ceil(value / 1024)
  }
}

/** Formats a KB value from the form as an nginx size string. */
export function kbToZoneSize(kb: number | null | undefined): string {
  return kb === null || kb === undefined ? '' : `${Math.trunc(kb)}k`
}
