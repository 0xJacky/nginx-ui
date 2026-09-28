<script setup lang="ts">
import type { WorkspaceTab } from '@/lib/workspace/state'
import { postToPane } from '@/lib/workspace/bridge'
import { paneWindowName, writePaneNode } from '@/lib/workspace/paneStorage'

export interface PaneRect {
  left: number
  top: number
  width: number
  height: number
}

const props = defineProps<{
  tab: WorkspaceTab
  /** Where the iframe sits inside the workspace body. */
  rect: PaneRect | null
  visible: boolean
  /** Whether tab clicks go to this pane; the others grey their header. */
  focused: boolean
  /** Lets the pane under the cursor ignore the pointer while the splitter moves. */
  isResizing: boolean
}>()

// The pane window reads its node from `LOCAL_<window name>` at boot, so the
// entry has to exist before the iframe does. Setup runs before the first render.
writePaneNode(localStorage, props.tab.id, props.tab.node)

// Captured once: the pane navigates by itself afterwards, and changing `src`
// would navigate it again. A recreated pane starts at the tab's last route.
const src = `${location.pathname}#${props.tab.route}`
const name = paneWindowName(props.tab.id)

const frameRef = useTemplateRef<HTMLIFrameElement>('frameRef')

// Changes are pushed from here; the initial state is sent when the pane says
// it is ready (see WorkSpace.vue), since the iframe's load event can come
// before the pane is listening.
function sendFocusState() {
  const pane = frameRef.value?.contentWindow
  if (pane)
    postToPane(pane, { type: 'focus-state', focused: props.focused })
}

watch(() => props.focused, sendFocusState)

const frameStyle = computed(() => {
  const rect = props.rect
  if (!rect)
    return { visibility: 'hidden' as const, left: '0px', top: '0px', width: '0px', height: '0px' }

  return {
    left: `${rect.left}px`,
    top: `${rect.top}px`,
    width: `${rect.width}px`,
    height: `${rect.height}px`,
    visibility: props.visible ? 'visible' as const : 'hidden' as const,
    pointerEvents: props.visible && !props.isResizing ? 'auto' as const : 'none' as const,
  }
})
</script>

<template>
  <iframe
    ref="frameRef"
    class="workspace-frame"
    :name
    :src
    :title="tab.node.name"
    :aria-hidden="!visible"
    :tabindex="visible ? 0 : -1"
    :style="frameStyle"
  />
</template>

<style scoped>
.workspace-frame {
  position: absolute;
  border: 0;
  outline: none;
  display: block;
  background: transparent;
}
</style>
