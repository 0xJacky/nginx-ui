<script lang="ts" setup>
import type { SplitpanesResizedPayload } from 'splitpanes'
import type { PaneRect } from './components/WorkspacePane.vue'
import type { AnalyticNode } from '@/api/node'
import type { PaneIndex, WorkspaceNode } from '@/lib/workspace/state'
import { BorderOutlined, CloseOutlined, PlusOutlined, SplitCellsOutlined } from '@antdv-next/icons'
import { useElementBounding, useEventListener } from '@vueuse/core'
import { storeToRefs } from 'pinia'
import { Pane, Splitpanes } from 'splitpanes'
import nodeApi from '@/api/node'
import settingsApi from '@/api/settings'
import NodeSwitcher from '@/components/NodeSwitcher'
import SetLanguage from '@/components/SetLanguage'
import SwitchAppearance from '@/components/SwitchAppearance'
import { getNodeSwitchBlocker } from '@/lib/node/switch'
import { listenToPanes, postToPane, watchSharedLogout } from '@/lib/workspace/bridge'
import { paneWindowName, prunePaneSettings, removePaneSettings, tabIdFromWindowName } from '@/lib/workspace/paneStorage'
import { paneOfTab, parseNodeIdParam, visiblePaneIndexes } from '@/lib/workspace/state'
import { useSettingsStore, useUserStore } from '@/pinia'
import { useWorkspaceStore } from '@/pinia/moudule/workspace'
import { version } from '@/version.json'
import WorkspacePane from './components/WorkspacePane.vue'
import WorkspaceTabs from './components/WorkspaceTabs.vue'
import 'splitpanes/dist/splitpanes.css'

const router = useRouter()
const route = useRoute()
const { message } = useGlobalApp()

const workspace = useWorkspaceStore()
const { state, paneSize } = storeToRefs(workspace)
const settingsStore = useSettingsStore()
const userStore = useUserStore()

const isDark = computed(() => settingsStore.theme === 'dark')

// The main window's settings may point at a remote node; the local server name
// is fetched past the node proxy.
const serverName = ref(settingsStore.server_name)
const localName = computed(() => serverName.value || $gettext('Local'))
settingsApi.get_server_name().then(r => {
  serverName.value = r.name
}).catch(() => {})

// ------------------------------------------------------------------ layout

const paneIndexes = computed(() => visiblePaneIndexes(state.value))
const isSplit = computed(() => state.value.layout === 'split')

function paneSizeOf(index: PaneIndex) {
  if (!isSplit.value)
    return 100
  return index === 0 ? paneSize.value : 100 - paneSize.value
}

const isResizing = ref(false)

function onResized({ panes }: SplitpanesResizedPayload) {
  isResizing.value = false
  if (isSplit.value && panes[0])
    paneSize.value = panes[0].size
}

// Iframes swallow pointer events, so the splitter drag must disable them from
// the very first pointerdown rather than from the first `resize` event.
function onBodyPointerDown(event: PointerEvent) {
  if ((event.target as HTMLElement | null)?.closest('.splitpanes__splitter'))
    isResizing.value = true
}

useEventListener(window, 'pointerup', () => {
  isResizing.value = false
})

// ------------------------------------------------------ pane geometry

// Pane iframes are never moved in the DOM (moving an iframe reloads it). They
// live in one layer and are positioned over the pane slots measured here, so a
// swap or a layout change only moves them visually.
const bodyRef = useTemplateRef<HTMLElement>('bodyRef')
const leftSlot = ref<HTMLElement | null>(null)
const rightSlot = ref<HTMLElement | null>(null)

function setSlotRef(index: PaneIndex, el: unknown) {
  const target = index === 0 ? leftSlot : rightSlot
  target.value = el instanceof HTMLElement ? el : null
}

const bodyBox = useElementBounding(bodyRef)
const leftBox = useElementBounding(leftSlot)
const rightBox = useElementBounding(rightSlot)

function toRect(box: ReturnType<typeof useElementBounding>): PaneRect | null {
  if (!box.width.value || !box.height.value)
    return null

  return {
    left: box.left.value - bodyBox.left.value,
    top: box.top.value - bodyBox.top.value,
    width: box.width.value,
    height: box.height.value,
  }
}

const paneRects = computed<[PaneRect | null, PaneRect | null]>(() => [toRect(leftBox), toRect(rightBox)])

const liveFrames = computed(() => {
  const focusRect = paneRects.value[state.value.focus] ?? paneRects.value[0]

  // Ordered by id so an existing iframe never has to move to keep the order.
  return workspace.liveIds
    .map(id => workspace.getTab(id))
    .filter(tab => !!tab)
    .sort((a, b) => a.id - b.id)
    .map(tab => {
      const pane = paneOfTab(state.value, tab.id)
      return {
        tab,
        visible: pane !== null,
        // The pane tab clicks go to; always the visible one in single layout.
        focused: pane !== null && (state.value.layout !== 'split' || pane === state.value.focus),
        // Background tabs keep a real size so they need no relayout when shown.
        rect: pane !== null ? paneRects.value[pane] : focusRect,
      }
    })
})

function paneTab(index: PaneIndex) {
  return workspace.getTab(state.value.panes[index])
}

// ------------------------------------------------------------- tabs

function showTab(tabId: number) {
  workspace.showTab(tabId)
}

function closeTab(tabId: number) {
  workspace.closeTab(tabId)
  // After the iframe is gone, so it cannot write its settings back.
  nextTick(() => removePaneSettings(localStorage, tabId))

  // A workspace without tabs has nothing to show; closing the last one leaves.
  if (!workspace.tabs.length)
    exitWorkspace()
}

function openTab(node: WorkspaceNode) {
  workspace.openTab(node)
}

function openTabInPane(index: PaneIndex, node: WorkspaceNode) {
  workspace.focusPane(index)
  workspace.openTab(node)
}

// ----------------------------------------------------------- focus

function focusTabPane(tabId: number | null) {
  if (tabId == null)
    return

  const pane = paneOfTab(state.value, tabId)
  if (pane !== null)
    workspace.focusPane(pane)
}

// Keyboard focus moving into an iframe blurs this window; the pane's own
// `focus` message covers clicks and focus moving between panes.
useEventListener(window, 'blur', () => {
  setTimeout(() => {
    const active = document.activeElement
    if (active instanceof HTMLIFrameElement)
      focusTabPane(tabIdFromWindowName(active.name))
  })
})

// Answers a pane that just booted (first load or reload) with its focus state;
// later changes are pushed by WorkspacePane.
function sendInitialFocusState(tabId: number) {
  const frame = document.querySelector<HTMLIFrameElement>(`iframe[name="${paneWindowName(tabId)}"]`)
  const focused = liveFrames.value.find(item => item.tab.id === tabId)?.focused ?? false
  if (frame?.contentWindow)
    postToPane(frame.contentWindow, { type: 'focus-state', focused })
}

const stopPaneListener = listenToPanes((tabId, paneMessage) => {
  switch (paneMessage.type) {
    case 'focus':
      focusTabPane(tabId)
      break
    case 'ready':
      sendInitialFocusState(tabId)
      break
    case 'node-changed':
      workspace.setTabNode(tabId, paneMessage.node)
      break
    case 'route-changed':
      workspace.setTabRoute(tabId, paneMessage.path, paneMessage.title)
      break
  }
})

// A logout or an expired session in any pane logs the whole workspace out,
// instead of every pane showing its own login page.
const stopLogoutWatcher = watchSharedLogout(() => {
  userStore.$hydrate()
  router.push({ path: '/login', query: { next: '/workspace' } })
})

onBeforeUnmount(() => {
  stopPaneListener()
  stopLogoutWatcher()
})

// ----------------------------------------------------------- entry

let nodeList: Promise<AnalyticNode[]> | null = null

async function resolveNode(nodeId: number): Promise<WorkspaceNode | null> {
  if (nodeId === 0)
    return { id: 0, name: 'Local' }

  nodeList ??= nodeApi.getLocalList().catch(() => [])
  const target = (await nodeList).find(item => item.id === nodeId)
  if (!target || getNodeSwitchBlocker(target, version)) {
    message.warning($gettext('Node %{name} cannot be opened', { name: target?.name ?? String(nodeId) }))
    return null
  }

  return { id: target.id, name: target.name }
}

/**
 * `#/workspace?l=<nodeId>` brings that node on screen (the header entry);
 * adding `&r=<nodeId>` splits the two nodes side by side. Without a query the
 * persisted tabs come back, or the main window's node opens as the first tab.
 */
async function handleEntry() {
  const left = parseNodeIdParam(route.query.l)
  const right = parseNodeIdParam(route.query.r)

  if (left === null && right === null) {
    if (!workspace.tabs.length)
      workspace.openNode({ id: settingsStore.node.id, name: settingsStore.node.name })
    return
  }

  // Consumed once, so a reload restores the tabs rather than adding more.
  router.replace({ path: '/workspace' })

  const [leftNode, rightNode] = await Promise.all([
    left !== null ? resolveNode(left) : null,
    right !== null ? resolveNode(right) : null,
  ])

  if (leftNode && rightNode)
    workspace.openSplit(leftNode, rightNode)
  else if (leftNode ?? rightNode)
    workspace.openNode((leftNode ?? rightNode)!)

  // Neither node could be opened and nothing was restored: nothing to show.
  if (!workspace.tabs.length)
    exitWorkspace()
}

workspace.restore()
prunePaneSettings(localStorage, workspace.tabs.map(tab => tab.id))
handleEntry()

watch(() => [route.query.l, route.query.r], ([l, r]) => {
  if (l != null || r != null)
    handleEntry()
})

function exitWorkspace() {
  router.push('/')
}
</script>

<template>
  <div class="workspace">
    <header class="workspace-bar">
      <ATooltip :title="$gettext('Exit workspace')" placement="bottomLeft">
        <button
          type="button"
          class="bar-button exit-button"
          :aria-label="$gettext('Exit workspace')"
          @click="exitWorkspace"
        >
          <CloseOutlined />
        </button>
      </ATooltip>

      <WorkspaceTabs
        class="bar-tabs"
        :tabs="state.tabs"
        :panes="state.panes"
        :layout="state.layout"
        :focus="state.focus"
        :local-name="localName"
        :is-dark="isDark"
        @show="showTab"
        @close="closeTab"
      />

      <NodeSwitcher picker @select="openTab">
        <button
          type="button"
          class="bar-button"
          :title="$gettext('New tab')"
          :aria-label="$gettext('New tab')"
        >
          <PlusOutlined />
        </button>
      </NodeSwitcher>

      <div class="bar-actions">
        <div class="layout-toggle" role="group">
          <button
            type="button"
            class="bar-button"
            :class="{ active: !isSplit }"
            :title="$gettext('Single pane')"
            :aria-label="$gettext('Single pane')"
            :aria-pressed="!isSplit"
            @click="workspace.setLayout('single')"
          >
            <BorderOutlined />
          </button>
          <button
            type="button"
            class="bar-button"
            :class="{ active: isSplit }"
            :title="$gettext('Split view')"
            :aria-label="$gettext('Split view')"
            :aria-pressed="isSplit"
            @click="workspace.setLayout('split')"
          >
            <SplitCellsOutlined />
          </button>
        </div>

        <SetLanguage />

        <SwitchAppearance />
      </div>
    </header>

    <div
      ref="bodyRef"
      class="workspace-body"
      :class="{ resizing: isResizing }"
      @pointerdown.capture="onBodyPointerDown"
    >
      <Splitpanes
        v-if="state.tabs.length"
        class="workspace-split"
        @resize="isResizing = true"
        @resized="onResized"
      >
        <Pane
          v-for="index in paneIndexes"
          :key="index"
          :size="paneSizeOf(index)"
          :min-size="20"
        >
          <div
            class="pane-slot"
            @pointerdown="workspace.focusPane(index)"
          >
            <div :ref="el => setSlotRef(index, el)" class="pane-area">
              <div v-if="!paneTab(index)" class="workspace-empty">
                <NodeSwitcher picker @select="node => openTabInPane(index, node)">
                  <AButton>
                    <template #icon>
                      <PlusOutlined />
                    </template>
                    {{ $gettext('New tab') }}
                  </AButton>
                </NodeSwitcher>
              </div>
            </div>
          </div>
        </Pane>
      </Splitpanes>

      <div class="frame-layer">
        <WorkspacePane
          v-for="frame in liveFrames"
          :key="frame.tab.id"
          :tab="frame.tab"
          :rect="frame.rect"
          :visible="frame.visible"
          :focused="frame.focused"
          :is-resizing="isResizing"
        />
      </div>
    </div>
  </div>
</template>

<style scoped lang="less">
@bar-height: 40px;

.workspace {
  height: 100vh;
  display: flex;
  flex-direction: column;
  background: #f0f2f5;
}

.workspace-bar {
  flex: none;
  height: @bar-height;
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 0 8px;
  user-select: none;
}

.bar-tabs {
  flex: 0 1 auto;
  align-self: stretch;
}

.bar-button {
  flex: none;
  width: 28px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: rgba(0, 0, 0, 0.65);
  cursor: pointer;
  transition: background-color 0.15s, color 0.15s;

  &:hover {
    background: rgba(0, 0, 0, 0.06);
    color: rgba(0, 0, 0, 0.88);
  }

  &:focus-visible {
    outline: 2px solid #1677ff;
    outline-offset: -2px;
  }

  &.active {
    background: #ffffff;
    color: #1677ff;
  }
}

.exit-button {
  margin-right: 4px;
}

.bar-actions {
  flex: none;
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 12px;
  padding-left: 8px;
}

.layout-toggle {
  display: flex;
  gap: 2px;
  padding: 2px;
  border-radius: 8px;
  background: rgba(0, 0, 0, 0.04);

  .bar-button {
    width: 26px;
    height: 24px;
  }
}

.workspace-body {
  position: relative;
  flex: 1;
  min-height: 0;
}

.workspace-split {
  height: 100%;

  :deep(.splitpanes__pane) {
    background: transparent;
  }

  :deep(.splitpanes__splitter) {
    position: relative;
    width: 6px;
    background: transparent;
    cursor: col-resize;

    &::after {
      content: '';
      position: absolute;
      top: 50%;
      left: 2px;
      width: 2px;
      height: 32px;
      border-radius: 1px;
      transform: translateY(-50%);
      background: rgba(0, 0, 0, 0.15);
      transition: background-color 0.15s;
    }

    &:hover::after {
      background: #1677ff;
    }
  }
}

.pane-slot {
  height: 100%;
  display: flex;
  flex-direction: column;
  background: #ffffff;
}

.pane-area {
  position: relative;
  flex: 1;
  min-height: 0;
}

.workspace-empty {
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  color: rgba(0, 0, 0, 0.45);
}

.frame-layer {
  position: absolute;
  inset: 0;
  pointer-events: none;

  > * {
    pointer-events: auto;
  }
}

.dark {
  .workspace {
    background: #000000;
  }

  .bar-button {
    color: rgba(255, 255, 255, 0.65);

    &:hover {
      background: rgba(255, 255, 255, 0.08);
      color: rgba(255, 255, 255, 0.88);
    }

    &.active {
      background: #141414;
      color: #4096ff;
    }
  }

  .layout-toggle {
    background: rgba(255, 255, 255, 0.06);
  }

  .workspace-split :deep(.splitpanes__splitter)::after {
    background: rgba(255, 255, 255, 0.2);
  }

  .workspace-split :deep(.splitpanes__splitter:hover)::after {
    background: #4096ff;
  }

  .pane-slot {
    background: #141414;
  }

  .workspace-empty {
    color: rgba(255, 255, 255, 0.45);
  }
}
</style>
