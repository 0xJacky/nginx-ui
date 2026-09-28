import type { AccessListWarning } from '@/api/access_list'

export function warningText(warning: AccessListWarning) {
  const rule = String(warning.rule)
  switch (warning.code) {
    case 'shadowed':
      return $gettext('Rule %{rule} matches every address, so the rules below it never apply.', { rule })
    case 'host_bits':
      return $gettext('Rule %{rule} sets bits below the prefix; Nginx ignores them.', { rule })
    case 'allows_all':
      return $gettext('This list allows every address.')
    case 'empty':
      return $gettext('This list has no rules; only "Everything else" applies.')
    default:
      return warning.code
  }
}
