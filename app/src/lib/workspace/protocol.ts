import type { WorkspaceNode } from './state'

/**
 * Messages a workspace pane posts to the workspace shell. Every pane is a
 * same-origin iframe; the shell checks the origin and that the posting window
 * really is the pane the envelope claims to come from.
 */

export const WORKSPACE_MESSAGE_SOURCE = 'nginx-ui-workspace'

export type PaneMessage
  = | { type: 'focus' }
    | { type: 'ready' }
    | { type: 'node-changed', node: WorkspaceNode }
    | { type: 'route-changed', path: string, title: string }

export interface PaneEnvelope {
  source: typeof WORKSPACE_MESSAGE_SOURCE
  tabId: number
  message: PaneMessage
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

function isPaneMessage(value: unknown): value is PaneMessage {
  if (!isRecord(value))
    return false

  switch (value.type) {
    case 'focus':
    case 'ready':
      return true
    case 'node-changed':
      return isRecord(value.node)
        && Number.isInteger(value.node.id)
        && (value.node.id as number) >= 0
        && typeof value.node.name === 'string'
    case 'route-changed':
      return typeof value.path === 'string'
        && value.path.startsWith('/')
        && typeof value.title === 'string'
    default:
      return false
  }
}

export function isPaneEnvelope(value: unknown): value is PaneEnvelope {
  return isRecord(value)
    && value.source === WORKSPACE_MESSAGE_SOURCE
    && Number.isInteger(value.tabId)
    && (value.tabId as number) > 0
    && isPaneMessage(value.message)
}

export function createEnvelope(tabId: number, message: PaneMessage): PaneEnvelope {
  return { source: WORKSPACE_MESSAGE_SOURCE, tabId, message }
}

/** Messages the workspace shell posts to one of its panes. */
export const WORKSPACE_SHELL_SOURCE = 'nginx-ui-workspace-shell'

export type ShellMessage
  = | { type: 'focus-state', focused: boolean }

export interface ShellEnvelope {
  source: typeof WORKSPACE_SHELL_SOURCE
  message: ShellMessage
}

export function isShellEnvelope(value: unknown): value is ShellEnvelope {
  return isRecord(value)
    && value.source === WORKSPACE_SHELL_SOURCE
    && isRecord(value.message)
    && value.message.type === 'focus-state'
    && typeof value.message.focused === 'boolean'
}

export function createShellEnvelope(message: ShellMessage): ShellEnvelope {
  return { source: WORKSPACE_SHELL_SOURCE, message }
}

/** localStorage key of the persisted user store (the store id). */
export const USER_STORAGE_KEY = 'user'

/** localStorage key of the shared theme and language settings (the store id). */
export const SETTINGS_STORAGE_KEY = 'settings'

/** Whether a persisted user store value still holds a login token. */
export function hasStoredToken(raw: string | null): boolean {
  if (!raw)
    return false

  try {
    const parsed = JSON.parse(raw)
    return isRecord(parsed) && typeof parsed.token === 'string' && parsed.token !== ''
  }
  catch {
    return false
  }
}

export interface SharedSettings {
  language?: string
  theme?: string
  preference_theme?: string
}

/** The theme and language fields of a persisted settings store value. */
export function parseSharedSettings(raw: string | null): SharedSettings {
  if (!raw)
    return {}

  try {
    const parsed = JSON.parse(raw)
    if (!isRecord(parsed))
      return {}

    const result: SharedSettings = {}
    for (const key of ['language', 'theme', 'preference_theme'] as const) {
      if (typeof parsed[key] === 'string' && parsed[key])
        result[key] = parsed[key] as string
    }
    return result
  }
  catch {
    return {}
  }
}
