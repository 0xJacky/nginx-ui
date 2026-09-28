// Turn a site name into a safe log file base name. Letters and digits of any
// script are kept: an ASCII-only filter maps every CJK character to "_", so
// two sites whose names differ only in those characters would share one log.
export function toLogFileBaseName(candidate: string): string {
  return candidate
    .replace(/[^\p{L}\p{N}\p{M}_.-]/gu, '_')
    .replace(/^\.+/, '') || 'site'
}
