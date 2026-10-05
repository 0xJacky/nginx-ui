// The access a plugin version asks for: its permissions and, with the
// network permission, the addresses it may reach. No addresses means any.
export interface AccessRequest {
  permissions?: string[]
  network_hosts?: string[]
}

/** What a new version asks for that the installed one did not, and back. */
export interface PermissionChanges {
  added: string[]
  removed: string[]
  /** Addresses added to a list the installed version already limited itself to. */
  addedHosts: string[]
  removedHosts: string[]
  /** The new version may reach any address where the installed one had a list. */
  anyHost: boolean
}

function hostsOf(access: AccessRequest): string[] | null {
  if (!access.permissions?.includes('network'))
    return null
  return access.network_hosts ?? []
}

/** Compares the access of an installed version with the next one. */
export function permissionChanges(installed: AccessRequest, next: AccessRequest): PermissionChanges {
  const before = new Set(installed.permissions ?? [])
  const after = new Set(next.permissions ?? [])
  const changes: PermissionChanges = {
    added: [...after].filter(p => !before.has(p)),
    removed: [...before].filter(p => !after.has(p)),
    addedHosts: [],
    removedHosts: [],
    anyHost: false,
  }
  const hostsBefore = hostsOf(installed)
  const hostsAfter = hostsOf(next)
  // A network permission that is new on its own already shows as added.
  if (hostsBefore && hostsAfter) {
    if (hostsBefore.length && !hostsAfter.length) {
      changes.anyHost = true
    }
    else if (hostsBefore.length) {
      changes.addedHosts = hostsAfter.filter(h => !hostsBefore.includes(h))
      changes.removedHosts = hostsBefore.filter(h => !hostsAfter.includes(h))
    }
  }
  return changes
}

/** Whether the next version asks for anything the installed one did not. */
export function asksForMore(changes: PermissionChanges): boolean {
  return changes.added.length > 0 || changes.addedHosts.length > 0 || changes.anyHost
}

/** Whether the next version gives up anything the installed one asked for. */
export function asksForLess(changes: PermissionChanges): boolean {
  return changes.removed.length > 0 || changes.removedHosts.length > 0
}
