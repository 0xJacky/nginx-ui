import { ConfigStatus } from '@/constants'

// Pure decisions for the Add Site wizard's draft bookkeeping. Entering the
// SSL step saves the site as a disabled draft; renaming it afterwards writes a
// new file, so the draft under the old name must be cleaned up.

/**
 * Returns the name of the draft that a save under `nextName` leaves behind,
 * or undefined when there is none (no draft yet, or the name is unchanged).
 */
export function staleDraftName(draftName: string | undefined, nextName: string | undefined): string | undefined {
  const previous = draftName?.trim() ?? ''
  const next = nextName?.trim() ?? ''

  return previous && next && previous !== next ? previous : undefined
}

export type StaleDraftAction = 'delete' | 'keep'

/**
 * Only a draft that is still disabled may be deleted. An enabled one (e.g. a
 * failed HTTPS run left it serving plain HTTP) or one in maintenance is live
 * and stays; an unknown status is treated as live too.
 */
export function staleDraftAction(status: string | undefined): StaleDraftAction {
  return status === ConfigStatus.Disabled ? 'delete' : 'keep'
}
