/**
 * Pure state transitions of the workspace: tabs, the two panes, focus, layout
 * and which pane iframes stay alive. Every function returns a new state and
 * never mutates its input, so the store only assigns the result.
 */

export type WorkspaceLayout = 'single' | 'split'
export type PaneIndex = 0 | 1

export interface WorkspaceNode {
  id: number
  name: string
}

export interface WorkspaceTab {
  id: number
  node: WorkspaceNode
  /** Last route the pane reported, as a hash-router full path. */
  route: string
  /** Page title the pane reported for `route`. */
  title: string
}

export interface WorkspaceState {
  tabs: WorkspaceTab[]
  /** Next tab id. Ids are never reused: they name the pane window and its storage. */
  nextId: number
  layout: WorkspaceLayout
  /**
   * Tab shown in the left and in the right pane. In single layout only the
   * left pane is visible; the right one keeps its tab for when split returns.
   * A tab is never in both panes.
   */
  panes: [number | null, number | null]
  focus: PaneIndex
  /** Tab ids, most recently shown first. */
  recent: number[]
}

/** Where a new tab opens. */
export const DEFAULT_TAB_ROUTE = '/dashboard'

/** Background tabs that keep their iframe alive besides the visible ones. */
export const MAX_BACKGROUND_TABS = 4

export function createWorkspaceState(): WorkspaceState {
  return {
    tabs: [],
    nextId: 1,
    layout: 'single',
    panes: [null, null],
    focus: 0,
    recent: [],
  }
}

function otherPane(index: PaneIndex): PaneIndex {
  return index === 0 ? 1 : 0
}

function touch(recent: number[], tabId: number): number[] {
  return [tabId, ...recent.filter(id => id !== tabId)]
}

export function findTab(state: WorkspaceState, tabId: number | null | undefined): WorkspaceTab | undefined {
  if (tabId == null)
    return undefined

  return state.tabs.find(tab => tab.id === tabId)
}

/** Panes the user can see in the current layout. */
export function visiblePaneIndexes(state: WorkspaceState): PaneIndex[] {
  return state.layout === 'split' ? [0, 1] : [0]
}

/** Tabs currently on screen. */
export function visibleTabIds(state: WorkspaceState): number[] {
  return visiblePaneIndexes(state)
    .map(index => state.panes[index])
    .filter((id): id is number => id != null)
}

/** The pane showing a tab, when it is on screen. */
export function paneOfTab(state: WorkspaceState, tabId: number): PaneIndex | null {
  const index = visiblePaneIndexes(state).find(pane => state.panes[pane] === tabId)
  return index ?? null
}

/**
 * Tabs that keep a live iframe: the visible ones plus the most recently used
 * background tabs. The rest are unloaded and rebuilt at their last route.
 */
export function liveTabIds(state: WorkspaceState, maxBackground = MAX_BACKGROUND_TABS): number[] {
  const visible = visibleTabIds(state)
  const known = new Set(state.tabs.map(tab => tab.id))
  const background = [
    ...state.recent,
    // Tabs missing from the MRU list (should not happen) come last.
    ...state.tabs.map(tab => tab.id).filter(id => !state.recent.includes(id)),
  ].filter(id => known.has(id) && !visible.includes(id))

  return [...visible, ...background.slice(0, Math.max(0, maxBackground))]
}

/** The most recently used tab that no pane is showing. */
function nextBackgroundTab(state: WorkspaceState): number | null {
  const shown = new Set(state.panes.filter((id): id is number => id != null))
  return liveTabIds({ ...state, panes: [null, null] }, Number.POSITIVE_INFINITY)
    .find(id => !shown.has(id)) ?? null
}

/**
 * Shows a tab in the focused pane. A tab already shown in the other pane swaps
 * places with the focused pane's tab, so it is never shown twice.
 */
export function showTab(state: WorkspaceState, tabId: number): WorkspaceState {
  if (!findTab(state, tabId))
    return state

  const focus: PaneIndex = state.layout === 'split' ? state.focus : 0
  const other = otherPane(focus)
  const panes: [number | null, number | null] = [...state.panes]

  if (panes[other] === tabId)
    panes[other] = panes[focus]

  panes[focus] = tabId

  return { ...state, panes, focus, recent: touch(state.recent, tabId) }
}

/** Adds a tab for a node and shows it in the focused pane. */
export function openTab(state: WorkspaceState, node: WorkspaceNode, route = DEFAULT_TAB_ROUTE): { state: WorkspaceState, tabId: number } {
  const tabId = state.nextId
  const tab: WorkspaceTab = { id: tabId, node: { id: node.id, name: node.name }, route, title: '' }
  const next = { ...state, tabs: [...state.tabs, tab], nextId: tabId + 1 }

  return { state: showTab(next, tabId), tabId }
}

/** The most recently used tab of a node, skipping `exclude`. */
export function findNodeTab(state: WorkspaceState, nodeId: number, exclude: number[] = []): number | null {
  const ordered = liveTabIds({ ...state, panes: [null, null] }, Number.POSITIVE_INFINITY)
  return ordered.find(id => !exclude.includes(id) && findTab(state, id)?.node.id === nodeId) ?? null
}

/** Brings a node on screen: its most recent tab if it has one, otherwise a new tab. */
export function openNode(state: WorkspaceState, node: WorkspaceNode): { state: WorkspaceState, tabId: number } {
  const focus: PaneIndex = state.layout === 'split' ? state.focus : 0
  const focusedTab = findTab(state, state.panes[focus])
  if (focusedTab?.node.id === node.id)
    return { state: showTab(state, focusedTab.id), tabId: focusedTab.id }

  const existing = findNodeTab(state, node.id)
  if (existing != null)
    return { state: showTab(state, existing), tabId: existing }

  return openTab(state, node)
}

/**
 * Splits the workspace with one node on each side, reusing the nodes' most
 * recent tabs. The same node on both sides gets two tabs. The right pane,
 * the one just asked for, takes focus.
 */
export function openSplit(state: WorkspaceState, left: WorkspaceNode, right: WorkspaceNode): WorkspaceState {
  let next = state

  let leftId = findNodeTab(next, left.id)
  if (leftId == null) {
    const opened = openTab(next, left)
    next = opened.state
    leftId = opened.tabId
  }

  let rightId = findNodeTab(next, right.id, [leftId])
  if (rightId == null) {
    const opened = openTab(next, right)
    next = opened.state
    rightId = opened.tabId
  }

  return {
    ...next,
    layout: 'split',
    panes: [leftId, rightId],
    focus: 1,
    recent: touch(touch(next.recent, leftId), rightId),
  }
}

/** Focuses a pane; in single layout only the left pane exists. */
export function focusPane(state: WorkspaceState, index: PaneIndex): WorkspaceState {
  if (state.layout !== 'split' && index !== 0)
    return state

  const tabId = state.panes[index]
  if (state.focus === index && (tabId == null || state.recent[0] === tabId))
    return state

  return { ...state, focus: index, recent: tabId != null ? touch(state.recent, tabId) : state.recent }
}

/**
 * Switches between single and split layout. Going single keeps the focused
 * tab on screen; going split fills an empty pane with the most recent
 * background tab.
 */
export function setLayout(state: WorkspaceState, layout: WorkspaceLayout): WorkspaceState {
  if (state.layout === layout)
    return state

  if (layout === 'single') {
    const panes: [number | null, number | null] = state.focus === 1
      ? [state.panes[1], state.panes[0]]
      : [...state.panes]

    return { ...state, layout, panes, focus: 0 }
  }

  let next: WorkspaceState = { ...state, layout, panes: [...state.panes] }
  for (const index of [0, 1] as PaneIndex[]) {
    if (next.panes[index] != null)
      continue

    const fill = nextBackgroundTab(next)
    if (fill != null) {
      const panes: [number | null, number | null] = [...next.panes]
      panes[index] = fill
      next = { ...next, panes }
    }
  }

  return next
}

/**
 * Closes a tab. A pane that showed it takes the most recent background tab,
 * or stays empty when there is none.
 */
export function closeTab(state: WorkspaceState, tabId: number): WorkspaceState {
  if (!findTab(state, tabId))
    return state

  let next: WorkspaceState = {
    ...state,
    tabs: state.tabs.filter(tab => tab.id !== tabId),
    recent: state.recent.filter(id => id !== tabId),
    panes: state.panes.map(id => id === tabId ? null : id) as [number | null, number | null],
  }

  for (const index of [0, 1] as PaneIndex[]) {
    if (state.panes[index] !== tabId)
      continue

    const fill = nextBackgroundTab(next)
    const panes: [number | null, number | null] = [...next.panes]
    panes[index] = fill
    next = { ...next, panes }
  }

  // An empty left pane in single layout would hide the remembered right tab.
  if (next.layout === 'single' && next.panes[0] == null && next.panes[1] != null)
    next = { ...next, panes: [next.panes[1], null] }

  return next
}

/** Updates what a pane reported about its tab. */
export function updateTab(state: WorkspaceState, tabId: number, patch: Partial<Omit<WorkspaceTab, 'id'>>): WorkspaceState {
  const tab = findTab(state, tabId)
  if (!tab)
    return state

  const changed = (Object.keys(patch) as (keyof typeof patch)[])
    .some(key => JSON.stringify(tab[key]) !== JSON.stringify(patch[key]))
  if (!changed)
    return state

  return {
    ...state,
    tabs: state.tabs.map(item => item.id === tabId ? { ...item, ...patch } : item),
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

function toTab(value: unknown): WorkspaceTab | null {
  if (!isRecord(value) || !isRecord(value.node))
    return null

  const id = Number(value.id)
  const nodeId = Number(value.node.id)
  if (!Number.isInteger(id) || id <= 0 || !Number.isInteger(nodeId) || nodeId < 0)
    return null

  const route = typeof value.route === 'string' && value.route.startsWith('/') ? value.route : DEFAULT_TAB_ROUTE

  return {
    id,
    node: { id: nodeId, name: typeof value.node.name === 'string' ? value.node.name : '' },
    route,
    title: typeof value.title === 'string' ? value.title : '',
  }
}

/**
 * Repairs a persisted state: drops malformed tabs and dangling ids, so a
 * stale or hand-edited localStorage entry cannot break the workspace.
 */
export function normalizeState(raw: unknown): WorkspaceState {
  const base = createWorkspaceState()
  if (!isRecord(raw))
    return base

  const tabs: WorkspaceTab[] = []
  for (const item of Array.isArray(raw.tabs) ? raw.tabs : []) {
    const tab = toTab(item)
    if (tab && !tabs.some(existing => existing.id === tab.id))
      tabs.push(tab)
  }

  const ids = new Set(tabs.map(tab => tab.id))
  const maxId = tabs.reduce((max, tab) => Math.max(max, tab.id), 0)
  const nextId = Math.max(Number.isInteger(raw.nextId) ? raw.nextId as number : 1, maxId + 1)

  const toId = (value: unknown) => typeof value === 'number' && ids.has(value) ? value : null
  const rawPanes = Array.isArray(raw.panes) ? raw.panes : []
  const left = toId(rawPanes[0])
  const rightCandidate = toId(rawPanes[1])
  const right = rightCandidate === left ? null : rightCandidate

  const recent = (Array.isArray(raw.recent) ? raw.recent : [])
    .filter((id, index, list): id is number => typeof id === 'number' && ids.has(id) && list.indexOf(id) === index)
  const missing = tabs.map(tab => tab.id).filter(id => !recent.includes(id))

  const layout: WorkspaceLayout = raw.layout === 'split' ? 'split' : 'single'
  const focus: PaneIndex = layout === 'split' && raw.focus === 1 ? 1 : 0

  let state: WorkspaceState = { tabs, nextId, layout, panes: [left, right], focus, recent: [...recent, ...missing] }

  // Put something on screen when the saved panes pointed at closed tabs.
  if (state.panes[0] == null && tabs.length) {
    const fill = nextBackgroundTab(state)
    if (fill != null)
      state = { ...state, panes: [fill, state.panes[1]] }
  }

  return state
}

/** Reads a node id from a `#/workspace?l=..&r=..` query value. */
export function parseNodeIdParam(value: unknown): number | null {
  const raw = Array.isArray(value) ? value[0] : value
  if (typeof raw !== 'string' && typeof raw !== 'number')
    return null
  if (typeof raw === 'string' && !/^\d+$/.test(raw.trim()))
    return null

  const id = Number(raw)
  return Number.isInteger(id) && id >= 0 ? id : null
}
