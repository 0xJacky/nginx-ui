import { describe, expect, test } from 'bun:test'
import {
  paneSettingsKey,
  paneWindowName,
  prunePaneSettings,
  removePaneSettings,
  tabIdFromWindowName,
  writePaneNode,
} from '@/lib/workspace/paneStorage'
import {
  createEnvelope,
  createShellEnvelope,
  hasStoredToken,
  isPaneEnvelope,
  isShellEnvelope,
  parseSharedSettings,
  WORKSPACE_MESSAGE_SOURCE,
  WORKSPACE_SHELL_SOURCE,
} from '@/lib/workspace/protocol'

function memoryStorage(initial: Record<string, string> = {}) {
  const data = new Map(Object.entries(initial))
  return {
    data,
    get length() {
      return data.size
    },
    key: (index: number) => [...data.keys()][index] ?? null,
    getItem: (key: string) => data.get(key) ?? null,
    setItem: (key: string, value: string) => {
      data.set(key, value)
    },
    removeItem: (key: string) => {
      data.delete(key)
    },
  }
}

describe('pane window names', () => {
  test('round-trip a tab id', () => {
    expect(paneWindowName(7)).toBe('ws-7')
    expect(tabIdFromWindowName('ws-7')).toBe(7)
    expect(paneSettingsKey(7)).toBe('LOCAL_ws-7')
  })

  test('other windows are not panes', () => {
    expect(tabIdFromWindowName('')).toBeNull()
    expect(tabIdFromWindowName(undefined)).toBeNull()
    expect(tabIdFromWindowName('main')).toBeNull()
    expect(tabIdFromWindowName('split-view-left')).toBeNull()
    expect(tabIdFromWindowName('ws-')).toBeNull()
    expect(tabIdFromWindowName('ws-1x')).toBeNull()
  })
})

describe('pane settings storage', () => {
  test('writes the node and keeps the other saved fields', () => {
    const storage = memoryStorage({ 'LOCAL_ws-3': JSON.stringify({ node: { id: 1, name: 'a' }, server_name: 'srv', route_path: '/x' }) })
    writePaneNode(storage, 3, { id: 2, name: 'edge' })
    expect(JSON.parse(storage.getItem('LOCAL_ws-3')!)).toEqual({ node: { id: 2, name: 'edge' }, server_name: 'srv', route_path: '/x' })
  })

  test('replaces a corrupt entry', () => {
    const storage = memoryStorage({ 'LOCAL_ws-3': '{broken' })
    writePaneNode(storage, 3, { id: 0, name: 'Local' })
    expect(JSON.parse(storage.getItem('LOCAL_ws-3')!)).toEqual({ node: { id: 0, name: 'Local' } })
  })

  test('removes one pane entry', () => {
    const storage = memoryStorage({ 'LOCAL_ws-3': '{}', 'LOCAL_ws-4': '{}' })
    removePaneSettings(storage, 3)
    expect([...storage.data.keys()]).toEqual(['LOCAL_ws-4'])
  })

  test('prunes entries of closed tabs and of the old workspace only', () => {
    const storage = memoryStorage({
      'LOCAL_main': '{}',
      'LOCAL_ws-1': '{}',
      'LOCAL_ws-2': '{}',
      'LOCAL_split-view-left': '{}',
      'LOCAL_split-view-right': '{}',
      'settings': '{}',
      'user': '{}',
    })
    prunePaneSettings(storage, [2])
    expect([...storage.data.keys()].sort()).toEqual(['LOCAL_main', 'LOCAL_ws-2', 'settings', 'user'])
  })
})

describe('pane messages', () => {
  test('accepts the messages a pane sends', () => {
    expect(isPaneEnvelope(createEnvelope(1, { type: 'focus' }))).toBe(true)
    expect(isPaneEnvelope(createEnvelope(1, { type: 'ready' }))).toBe(true)
    expect(isPaneEnvelope(createEnvelope(1, { type: 'node-changed', node: { id: 0, name: 'Local' } }))).toBe(true)
    expect(isPaneEnvelope(createEnvelope(1, { type: 'route-changed', path: '/sites/list', title: 'Sites' }))).toBe(true)
  })

  test('rejects foreign or malformed messages', () => {
    expect(isPaneEnvelope(null)).toBe(false)
    expect(isPaneEnvelope('focus')).toBe(false)
    expect(isPaneEnvelope({ source: 'other', tabId: 1, message: { type: 'focus' } })).toBe(false)
    expect(isPaneEnvelope({ source: WORKSPACE_MESSAGE_SOURCE, tabId: 0, message: { type: 'focus' } })).toBe(false)
    expect(isPaneEnvelope({ source: WORKSPACE_MESSAGE_SOURCE, tabId: 1, message: { type: 'reload' } })).toBe(false)
    expect(isPaneEnvelope({ source: WORKSPACE_MESSAGE_SOURCE, tabId: 1, message: { type: 'node-changed', node: { id: -1, name: 'x' } } })).toBe(false)
    expect(isPaneEnvelope({ source: WORKSPACE_MESSAGE_SOURCE, tabId: 1, message: { type: 'route-changed', path: 'https://evil.example', title: '' } })).toBe(false)
  })
})

describe('shell messages', () => {
  test('accepts the focus state the shell sends', () => {
    expect(isShellEnvelope(createShellEnvelope({ type: 'focus-state', focused: true }))).toBe(true)
    expect(isShellEnvelope(createShellEnvelope({ type: 'focus-state', focused: false }))).toBe(true)
  })

  test('rejects pane messages and malformed focus states', () => {
    expect(isShellEnvelope(createEnvelope(1, { type: 'focus' }))).toBe(false)
    expect(isShellEnvelope({ source: WORKSPACE_SHELL_SOURCE, message: { type: 'focus-state' } })).toBe(false)
    expect(isShellEnvelope({ source: WORKSPACE_SHELL_SOURCE, message: { type: 'focus-state', focused: 'yes' } })).toBe(false)
    expect(isShellEnvelope({ source: WORKSPACE_SHELL_SOURCE, message: { type: 'reload' } })).toBe(false)
  })
})

describe('shared storage values', () => {
  test('detects a cleared login token', () => {
    expect(hasStoredToken(JSON.stringify({ token: 'abc' }))).toBe(true)
    expect(hasStoredToken(JSON.stringify({ token: '' }))).toBe(false)
    expect(hasStoredToken(null)).toBe(false)
    expect(hasStoredToken('{broken')).toBe(false)
  })

  test('reads theme and language', () => {
    expect(parseSharedSettings(JSON.stringify({ language: 'zh_CN', theme: 'dark', preference_theme: 'auto', other: 1 })))
      .toEqual({ language: 'zh_CN', theme: 'dark', preference_theme: 'auto' })
    expect(parseSharedSettings(JSON.stringify({ language: '' }))).toEqual({})
    expect(parseSharedSettings('{broken')).toEqual({})
    expect(parseSharedSettings(null)).toEqual({})
  })
})
