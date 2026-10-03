/** The part of a plugin or a manifest the conflict rules read. */
export interface ConflictSubject {
  id: string
  conflicts?: string[]
}

/** An installed plugin as the conflict rules see it. */
export interface ConflictCandidate extends ConflictSubject {
  enabled: boolean
}

/**
 * Ids of the installed plugins that conflict with the subject. Either side may
 * declare it, so a plugin that names the subject counts as well.
 */
export function conflictingIds(subject: ConflictSubject, installed: ConflictSubject[]): string[] {
  const ids = new Set(subject.conflicts ?? [])
  for (const other of installed) {
    if (other.conflicts?.includes(subject.id))
      ids.add(other.id)
  }
  ids.delete(subject.id)
  return [...ids].sort()
}

/** The enabled plugins among the installed ones that conflict with the subject. */
export function enabledConflicts<T extends ConflictCandidate>(subject: ConflictSubject, installed: T[]): T[] {
  const ids = new Set(conflictingIds(subject, installed))
  return installed.filter(other => other.enabled && ids.has(other.id))
}

/** Display names for plugin ids, the id itself when the plugin is not installed. */
export function conflictNames<T extends { id: string }>(
  ids: string[],
  installed: T[],
  nameOf: (plugin: T) => string,
): string[] {
  return ids.map(id => {
    const plugin = installed.find(item => item.id === id)
    return plugin ? nameOf(plugin) : id
  })
}

/** Names joined for a sentence. */
export function joinNames(names: string[]): string {
  return names.join(', ')
}

/** Shown on a plugin that cannot be on together with others. */
export function conflictNote(names: string[]): string {
  return $gettext('Cannot be enabled together with %{names}', { names: joinNames(names) })
}

/** Asked before a plugin is turned on while others that conflict with it are on. */
export function enableReplacesText(name: string, names: string[]): string {
  return $gettext('Enabling %{name} will disable %{others}, because these plugins cannot be enabled at the same time.', {
    name,
    others: joinNames(names),
  })
}

/** Shown before a package is installed and turned on while others are on. */
export function installReplacesText(names: string[]): string {
  return $gettext('Enabling this plugin will disable %{names}, because these plugins cannot be enabled at the same time.', {
    names: joinNames(names),
  })
}
