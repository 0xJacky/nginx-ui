import type { WorkspaceNode } from './state'

/**
 * Every pane is a separate app instance whose window name picks its own
 * settings entry (`LOCAL_<window.name>`, see the settings store). Writing that
 * entry before the iframe exists selects the pane's node without touching the
 * settings of any other window.
 */

const PANE_NAME_PREFIX = 'ws-'
const PANE_NAME_PATTERN = /^ws-(\d+)$/
const SETTINGS_KEY_PREFIX = 'LOCAL_'

/** Entries of the previous fixed two-iframe workspace. */
const LEGACY_PANE_KEYS = ['LOCAL_split-view-left', 'LOCAL_split-view-right']

type KeyValueStorage = Pick<Storage, 'getItem' | 'setItem' | 'removeItem' | 'key' | 'length'>

export function paneWindowName(tabId: number): string {
  return `${PANE_NAME_PREFIX}${tabId}`
}

/** The tab id a pane window name belongs to, or null for any other window. */
export function tabIdFromWindowName(name: string | null | undefined): number | null {
  const match = PANE_NAME_PATTERN.exec(name ?? '')
  return match ? Number(match[1]) : null
}

export function paneSettingsKey(tabId: number): string {
  return `${SETTINGS_KEY_PREFIX}${paneWindowName(tabId)}`
}

/** Stores the pane's node, keeping whatever else the pane saved there. */
export function writePaneNode(storage: KeyValueStorage, tabId: number, node: WorkspaceNode) {
  const key = paneSettingsKey(tabId)
  let saved: Record<string, unknown> = {}
  try {
    const parsed = JSON.parse(storage.getItem(key) ?? '{}')
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed))
      saved = parsed
  }
  catch {
    // A corrupt entry is replaced.
  }

  storage.setItem(key, JSON.stringify({ ...saved, node: { id: node.id, name: node.name } }))
}

export function removePaneSettings(storage: KeyValueStorage, tabId: number) {
  storage.removeItem(paneSettingsKey(tabId))
}

/** Removes the settings of panes whose tab no longer exists. */
export function prunePaneSettings(storage: KeyValueStorage, tabIds: number[]) {
  const keep = new Set(tabIds)
  const stale: string[] = [...LEGACY_PANE_KEYS]

  for (let index = 0; index < storage.length; index++) {
    const key = storage.key(index)
    if (!key?.startsWith(SETTINGS_KEY_PREFIX))
      continue

    const tabId = tabIdFromWindowName(key.slice(SETTINGS_KEY_PREFIX.length))
    if (tabId != null && !keep.has(tabId))
      stale.push(key)
  }

  stale.forEach(key => storage.removeItem(key))
}
