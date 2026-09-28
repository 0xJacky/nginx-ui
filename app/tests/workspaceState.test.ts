import type { WorkspaceNode, WorkspaceState } from '@/lib/workspace/state'
import { describe, expect, test } from 'bun:test'
import { getNodeColor, LOCAL_NODE_COLOR, NODE_COLOR_PALETTE, nodeColorFor } from '@/lib/node/color'
import {
  closeTab,
  createWorkspaceState,
  DEFAULT_TAB_ROUTE,
  findNodeTab,
  focusPane,
  liveTabIds,
  normalizeState,
  openNode,
  openSplit,
  openTab,
  paneOfTab,
  parseNodeIdParam,
  setLayout,
  showTab,
  updateTab,
  visibleTabIds,
} from '@/lib/workspace/state'

const local: WorkspaceNode = { id: 0, name: 'Local' }
const edge: WorkspaceNode = { id: 2, name: 'edge' }
const lb: WorkspaceNode = { id: 3, name: 'lb' }

/** Opens one tab per node, in order; returns the state and the tab ids. */
function withTabs(nodes: WorkspaceNode[], base = createWorkspaceState()) {
  let state = base
  const ids: number[] = []
  for (const node of nodes) {
    const opened = openTab(state, node)
    state = opened.state
    ids.push(opened.tabId)
  }
  return { state, ids }
}

function split(state: WorkspaceState, left: number, right: number, focus: 0 | 1 = 0): WorkspaceState {
  return { ...state, layout: 'split', panes: [left, right], focus }
}

describe('opening tabs', () => {
  test('a new tab opens at the dashboard in the focused pane', () => {
    const { state, ids } = withTabs([local])
    expect(state.tabs).toEqual([{ id: 1, node: local, route: DEFAULT_TAB_ROUTE, title: '' }])
    expect(state.panes).toEqual([ids[0], null])
    expect(state.nextId).toBe(2)
  })

  test('the same node may be open in several tabs', () => {
    const { state } = withTabs([edge, edge])
    expect(state.tabs.map(tab => tab.node.id)).toEqual([2, 2])
    expect(state.tabs.map(tab => tab.id)).toEqual([1, 2])
  })

  test('in split layout a new tab replaces the focused pane only', () => {
    const { state, ids } = withTabs([local, edge])
    const next = openTab(split(state, ids[0], ids[1], 1), lb)
    expect(next.state.panes).toEqual([ids[0], next.tabId])
  })

  test('tab ids are never reused after a close', () => {
    const { state, ids } = withTabs([local, edge])
    const closed = closeTab(state, ids[1])
    expect(openTab(closed, lb).tabId).toBe(3)
  })
})

describe('showing tabs', () => {
  test('shows a background tab in the focused pane', () => {
    const { state, ids } = withTabs([local, edge, lb])
    const next = showTab(split(state, ids[0], ids[1], 0), ids[2])
    expect(next.panes).toEqual([ids[2], ids[1]])
  })

  test('swaps the panes when the tab is shown in the other pane', () => {
    const { state, ids } = withTabs([local, edge])
    const next = showTab(split(state, ids[0], ids[1], 0), ids[1])
    expect(next.panes).toEqual([ids[1], ids[0]])
    expect(next.focus).toBe(0)
  })

  test('does nothing visible for the focused pane\'s own tab', () => {
    const { state, ids } = withTabs([local, edge])
    const next = showTab(split(state, ids[0], ids[1], 1), ids[1])
    expect(next.panes).toEqual([ids[0], ids[1]])
  })

  test('single layout keeps a tab out of both panes', () => {
    const { state, ids } = withTabs([local, edge])
    const single = { ...state, layout: 'single' as const, panes: [ids[0], ids[1]] as [number, number] }
    const next = showTab(single, ids[1])
    expect(next.panes).toEqual([ids[1], ids[0]])
  })

  test('ignores unknown tabs', () => {
    const { state } = withTabs([local])
    expect(showTab(state, 99)).toBe(state)
  })

  test('moves the shown tab to the front of the recent list', () => {
    const { state, ids } = withTabs([local, edge, lb])
    expect(state.recent).toEqual([ids[2], ids[1], ids[0]])
    expect(showTab(state, ids[0]).recent).toEqual([ids[0], ids[2], ids[1]])
  })
})

describe('focus and layout', () => {
  test('focusing a pane marks its tab as recently used', () => {
    const { state, ids } = withTabs([local, edge, lb])
    const next = focusPane(split(state, ids[0], ids[1], 1), 0)
    expect(next.focus).toBe(0)
    expect(next.recent[0]).toBe(ids[0])
  })

  test('the right pane cannot take focus in single layout', () => {
    const { state } = withTabs([local])
    expect(focusPane(state, 1)).toBe(state)
  })

  test('going split fills the right pane with the most recent background tab', () => {
    const { state, ids } = withTabs([local, edge, lb])
    const shown = showTab(state, ids[0])
    const next = setLayout(shown, 'split')
    expect(next.layout).toBe('split')
    expect(next.panes).toEqual([ids[0], ids[2]])
  })

  test('going split with a single tab leaves the right pane empty', () => {
    const { state, ids } = withTabs([local])
    expect(setLayout(state, 'split').panes).toEqual([ids[0], null])
  })

  test('going single keeps the focused tab on screen', () => {
    const { state, ids } = withTabs([local, edge])
    const next = setLayout(split(state, ids[0], ids[1], 1), 'single')
    expect(next.layout).toBe('single')
    expect(visibleTabIds(next)).toEqual([ids[1]])
    expect(next.focus).toBe(0)
  })

  test('split -> single -> split brings the other tab back', () => {
    const { state, ids } = withTabs([local, edge])
    const next = setLayout(setLayout(split(state, ids[0], ids[1]), 'single'), 'split')
    expect(next.panes).toEqual([ids[0], ids[1]])
  })
})

describe('closing tabs', () => {
  test('a pane that showed the tab takes the most recent background tab', () => {
    const { state, ids } = withTabs([local, edge, lb])
    const shown = split(showTab(state, ids[0]), ids[0], ids[1])
    const next = closeTab(shown, ids[1])
    expect(next.tabs.map(tab => tab.id)).toEqual([ids[0], ids[2]])
    expect(next.panes).toEqual([ids[0], ids[2]])
  })

  test('a pane stays empty when nothing is left to show', () => {
    const { state, ids } = withTabs([local, edge])
    const next = closeTab(split(state, ids[0], ids[1]), ids[1])
    expect(next.panes).toEqual([ids[0], null])
  })

  test('closing a background tab leaves the panes alone', () => {
    const { state, ids } = withTabs([local, edge, lb])
    const shown = split(state, ids[0], ids[1])
    expect(closeTab(shown, ids[2]).panes).toEqual([ids[0], ids[1]])
  })

  test('closing the last tab empties the workspace', () => {
    const { state, ids } = withTabs([local])
    const next = closeTab(state, ids[0])
    expect(next.tabs).toEqual([])
    expect(next.panes).toEqual([null, null])
    expect(next.recent).toEqual([])
  })

  test('single layout falls back to the remembered right tab', () => {
    const { state, ids } = withTabs([local, edge])
    const single = { ...state, layout: 'single' as const, panes: [ids[0], ids[1]] as [number, number], recent: [ids[0]] }
    const next = closeTab(single, ids[0])
    expect(next.panes[0]).toBe(ids[1])
  })
})

describe('keeping iframes alive', () => {
  test('keeps the visible tabs and the four most recent background tabs', () => {
    const nodes = Array.from({ length: 8 }, (_, index) => ({ id: index + 1, name: `n${index + 1}` }))
    const { state, ids } = withTabs(nodes)
    const shown = split(state, ids[0], ids[1])
    // recent: 8, 7, 6, 5, 4, 3, 2, 1
    expect(liveTabIds(shown)).toEqual([ids[0], ids[1], ids[7], ids[6], ids[5], ids[4]])
  })

  test('a recreated tab counts as recent again', () => {
    const nodes = Array.from({ length: 7 }, (_, index) => ({ id: index + 1, name: `n${index + 1}` }))
    const { state, ids } = withTabs(nodes)
    const evicted = ids[0]
    expect(liveTabIds(state)).not.toContain(evicted)
    const next = showTab(state, evicted)
    expect(liveTabIds(next)[0]).toBe(evicted)
  })

  test('single layout only counts the left pane as visible', () => {
    const { state, ids } = withTabs([local, edge])
    const single = { ...state, layout: 'single' as const, panes: [ids[0], ids[1]] as [number, number] }
    expect(visibleTabIds(single)).toEqual([ids[0]])
    expect(paneOfTab(single, ids[1])).toBeNull()
    expect(liveTabIds(single, 0)).toEqual([ids[0]])
  })
})

describe('opening nodes', () => {
  test('reuses the node\'s most recent tab', () => {
    const { state, ids } = withTabs([edge, local, edge])
    expect(findNodeTab(state, edge.id)).toBe(ids[2])
    const next = openNode(showTab(state, ids[1]), edge)
    expect(next.tabId).toBe(ids[2])
    expect(next.state.tabs).toHaveLength(3)
  })

  test('opens a tab for a node without one', () => {
    const { state } = withTabs([local])
    const next = openNode(state, lb)
    expect(next.state.tabs).toHaveLength(2)
    expect(next.state.panes[0]).toBe(next.tabId)
  })

  test('split reuses tabs and focuses the right pane', () => {
    const { state, ids } = withTabs([local, edge])
    const next = openSplit(state, local, edge)
    expect(next.tabs).toHaveLength(2)
    expect(next.layout).toBe('split')
    expect(next.panes).toEqual([ids[0], ids[1]])
    expect(next.focus).toBe(1)
  })

  test('split of one node with itself uses two tabs', () => {
    const { state, ids } = withTabs([edge])
    const next = openSplit(state, edge, edge)
    expect(next.tabs).toHaveLength(2)
    expect(next.panes[0]).toBe(ids[0])
    expect(next.panes[1]).not.toBe(ids[0])
  })

  test('split from an empty workspace opens both tabs', () => {
    const next = openSplit(createWorkspaceState(), local, lb)
    expect(next.tabs.map(tab => tab.node.id)).toEqual([0, 3])
    expect(next.panes).toEqual([1, 2])
  })
})

describe('updating tabs', () => {
  test('records the route, title and node a pane reports', () => {
    const { state, ids } = withTabs([local])
    const next = updateTab(state, ids[0], { route: '/sites/list', title: 'Sites' })
    expect(next.tabs[0]).toMatchObject({ route: '/sites/list', title: 'Sites' })
    expect(updateTab(next, ids[0], { node: edge }).tabs[0].node).toEqual(edge)
  })

  test('returns the same state when nothing changed', () => {
    const { state, ids } = withTabs([local])
    expect(updateTab(state, ids[0], { route: DEFAULT_TAB_ROUTE })).toBe(state)
    expect(updateTab(state, 42, { title: 'x' })).toBe(state)
  })
})

describe('restoring persisted state', () => {
  test('keeps a valid state as is', () => {
    const { state, ids } = withTabs([local, edge])
    const shown = split(state, ids[0], ids[1], 1)
    expect(normalizeState(JSON.parse(JSON.stringify(shown)))).toEqual(shown)
  })

  test('drops malformed tabs and dangling pane ids', () => {
    const next = normalizeState({
      tabs: [
        { id: 4, node: { id: 2, name: 'edge' }, route: '/sites/list', title: 'Sites' },
        { id: 4, node: { id: 3, name: 'dup' } },
        { id: 'x', node: { id: 1 } },
        { id: 5, node: { id: 1, name: 'n' }, route: 'javascript:alert(1)' },
      ],
      nextId: 2,
      layout: 'split',
      panes: [9, 5],
      focus: 1,
      recent: [5, 9, 5],
    })
    expect(next.tabs.map(tab => tab.id)).toEqual([4, 5])
    expect(next.tabs[1].route).toBe(DEFAULT_TAB_ROUTE)
    expect(next.nextId).toBe(6)
    expect(next.recent).toEqual([5, 4])
    // The missing left tab is refilled with a tab that is not already shown.
    expect(next.panes).toEqual([4, 5])
  })

  test('falls back to an empty workspace', () => {
    expect(normalizeState(null)).toEqual(createWorkspaceState())
    expect(normalizeState('broken')).toEqual(createWorkspaceState())
  })
})

describe('query node ids', () => {
  test('accepts non-negative integers', () => {
    expect(parseNodeIdParam('0')).toBe(0)
    expect(parseNodeIdParam('12')).toBe(12)
    expect(parseNodeIdParam(['7', '8'])).toBe(7)
  })

  test('rejects anything else', () => {
    expect(parseNodeIdParam(undefined)).toBeNull()
    expect(parseNodeIdParam(null)).toBeNull()
    expect(parseNodeIdParam('')).toBeNull()
    expect(parseNodeIdParam('-1')).toBeNull()
    expect(parseNodeIdParam('1.5')).toBeNull()
    expect(parseNodeIdParam('abc')).toBeNull()
  })
})

describe('node colors', () => {
  test('the local node uses the primary blue', () => {
    expect(getNodeColor(0)).toEqual(LOCAL_NODE_COLOR)
    expect(nodeColorFor(0, false)).toBe('#1677ff')
  })

  test('remote nodes get stable palette colors that differ from their neighbours', () => {
    expect(getNodeColor(1)).toEqual(NODE_COLOR_PALETTE[0])
    expect(getNodeColor(2)).not.toEqual(getNodeColor(1))
    expect(getNodeColor(1 + NODE_COLOR_PALETTE.length)).toEqual(getNodeColor(1))
    expect(getNodeColor(5)).toEqual(getNodeColor(5))
  })

  test('no remote color is the local blue', () => {
    for (const color of NODE_COLOR_PALETTE) {
      expect(color.light).not.toBe(LOCAL_NODE_COLOR.light)
      expect(color.dark).not.toBe(LOCAL_NODE_COLOR.dark)
    }
  })

  test('has a lighter variant for dark mode', () => {
    expect(nodeColorFor(1, true)).toBe(NODE_COLOR_PALETTE[0].dark)
    expect(nodeColorFor(1, false)).toBe(NODE_COLOR_PALETTE[0].light)
  })
})
