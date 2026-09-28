import type { PaneIndex, WorkspaceLayout, WorkspaceNode, WorkspaceState, WorkspaceTab } from '@/lib/workspace/state'
import {
  closeTab as closeTabState,
  createWorkspaceState,
  findTab,
  focusPane as focusPaneState,
  liveTabIds,
  normalizeState,
  openNode as openNodeState,
  openSplit as openSplitState,
  openTab as openTabState,
  setLayout as setLayoutState,
  showTab as showTabState,
  updateTab,
  visibleTabIds,
} from '@/lib/workspace/state'

/**
 * Tabs, panes and layout of the workspace view. Persisted so reloading
 * `/workspace` brings every tab back at its last page. The transitions live in
 * `lib/workspace/state.ts`; this store only holds the result.
 */
export const useWorkspaceStore = defineStore('workspace', () => {
  const state = ref<WorkspaceState>(createWorkspaceState())
  /** Width of the left pane in split layout, in percent. */
  const paneSize = ref(50)

  const tabs = computed(() => state.value.tabs)
  const layout = computed(() => state.value.layout)
  const panes = computed(() => state.value.panes)
  const focus = computed(() => state.value.focus)
  const visibleIds = computed(() => visibleTabIds(state.value))
  const liveIds = computed(() => liveTabIds(state.value))

  function getTab(tabId: number | null | undefined): WorkspaceTab | undefined {
    return findTab(state.value, tabId)
  }

  /** Repairs whatever was restored from localStorage. */
  function restore() {
    state.value = normalizeState(state.value)
    if (!Number.isFinite(paneSize.value) || paneSize.value < 20 || paneSize.value > 80)
      paneSize.value = 50
  }

  function openTab(node: WorkspaceNode) {
    const result = openTabState(state.value, node)
    state.value = result.state
    return result.tabId
  }

  function openNode(node: WorkspaceNode) {
    const result = openNodeState(state.value, node)
    state.value = result.state
    return result.tabId
  }

  function openSplit(left: WorkspaceNode, right: WorkspaceNode) {
    state.value = openSplitState(state.value, left, right)
  }

  function showTab(tabId: number) {
    state.value = showTabState(state.value, tabId)
  }

  function closeTab(tabId: number) {
    state.value = closeTabState(state.value, tabId)
  }

  function focusPane(index: PaneIndex) {
    state.value = focusPaneState(state.value, index)
  }

  function setLayout(value: WorkspaceLayout) {
    state.value = setLayoutState(state.value, value)
  }

  function setTabNode(tabId: number, node: WorkspaceNode) {
    state.value = updateTab(state.value, tabId, { node: { id: node.id, name: node.name } })
  }

  function setTabRoute(tabId: number, route: string, title: string) {
    state.value = updateTab(state.value, tabId, { route, title })
  }

  return {
    state,
    paneSize,
    tabs,
    layout,
    panes,
    focus,
    visibleIds,
    liveIds,
    getTab,
    restore,
    openTab,
    openNode,
    openSplit,
    showTab,
    closeTab,
    focusPane,
    setLayout,
    setTabNode,
    setTabRoute,
  }
}, {
  persist: {
    pick: ['state', 'paneSize'],
  },
})
