import type { AccessList, AccessServerState } from '@/api/access_list'

export interface AccessOption {
  label: string
  value: string
}

// Value of the "public" choice in the selects. List slugs never collide with
// it because a slug cannot contain a colon.
export const PublicValue = ':public'
// Value of the "inherit the server" choice of a location.
export const InheritValue = ':inherit'

export function listOptions(lists: AccessList[], currentSlug?: string): AccessOption[] {
  const options = lists.map(l => ({ label: l.name, value: l.slug }))
  // A file can include a list that no longer exists or was created on another
  // node; keep it selectable so the select does not show a raw value.
  if (currentSlug && !lists.some(l => l.slug === currentSlug))
    options.push({ label: $gettext('%{slug} (missing)', { slug: currentSlug }), value: currentSlug })
  return options
}

export function serverAccessLabel(state: AccessServerState | undefined, listName: (slug?: string) => string) {
  switch (state?.mode) {
    case 'list':
      return listName(state.slug)
    case 'manual':
      return $gettext('Custom rules')
    default:
      return $gettext('Public')
  }
}
