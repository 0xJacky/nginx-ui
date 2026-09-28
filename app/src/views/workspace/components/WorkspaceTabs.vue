<script setup lang="ts">
import type { PaneIndex, WorkspaceLayout, WorkspaceTab } from '@/lib/workspace/state'
import { CloseOutlined } from '@antdv-next/icons'
import { nodeColorFor } from '@/lib/node/color'

const props = defineProps<{
  tabs: WorkspaceTab[]
  panes: [number | null, number | null]
  layout: WorkspaceLayout
  focus: PaneIndex
  /** Display name of the local node (its server name). */
  localName: string
  isDark: boolean
}>()

const emit = defineEmits<{
  show: [tabId: number]
  close: [tabId: number]
}>()

const PANE_BADGES = ['L', 'R'] as const

function paneOf(tabId: number): PaneIndex | null {
  if (props.panes[0] === tabId)
    return 0
  if (props.layout === 'split' && props.panes[1] === tabId)
    return 1
  return null
}

const items = computed(() => props.tabs.map(tab => {
  const pane = paneOf(tab.id)
  const nodeName = tab.node.id === 0 ? props.localName : tab.node.name
  return {
    tab,
    nodeName,
    label: tab.title ? `${nodeName} · ${tab.title}` : nodeName,
    color: nodeColorFor(tab.node.id, props.isDark),
    isShown: pane !== null,
    isFocused: pane !== null && (props.layout !== 'split' || pane === props.focus),
    badge: props.layout === 'split' && pane !== null ? PANE_BADGES[pane] : null,
  }
}))

const stripRef = useTemplateRef<HTMLElement>('stripRef')

// Keep the tab of the focused pane in view when the strip overflows.
const focusedTabId = computed(() => items.value.find(item => item.isFocused)?.tab.id ?? null)
watch(focusedTabId, async tabId => {
  if (tabId == null)
    return

  await nextTick()
  stripRef.value?.querySelector<HTMLElement>(`[data-tab-id="${tabId}"]`)
    ?.scrollIntoView({ block: 'nearest', inline: 'nearest' })
}, { immediate: true })

// A vertical wheel scrolls the strip sideways, like browser tab strips.
function onWheel(event: WheelEvent) {
  const strip = stripRef.value
  if (!strip || Math.abs(event.deltaX) > Math.abs(event.deltaY))
    return

  strip.scrollLeft += event.deltaY
}

function onAuxClick(event: MouseEvent, tabId: number) {
  // Middle click closes, as in browsers.
  if (event.button === 1)
    emit('close', tabId)
}
</script>

<template>
  <div ref="stripRef" class="workspace-tabs" role="tablist" @wheel.passive="onWheel">
    <div
      v-for="item in items"
      :key="item.tab.id"
      :data-tab-id="item.tab.id"
      class="workspace-tab"
      :class="{ shown: item.isShown, focused: item.isFocused }"
      role="tab"
      tabindex="0"
      :aria-selected="item.isShown"
      :title="item.label"
      @click="emit('show', item.tab.id)"
      @keydown.enter.prevent="emit('show', item.tab.id)"
      @keydown.space.prevent="emit('show', item.tab.id)"
      @auxclick="onAuxClick($event, item.tab.id)"
    >
      <span class="tab-dot" :style="{ background: item.color }" />
      <span class="tab-label">
        <span class="tab-node">{{ item.nodeName }}</span>
        <template v-if="item.tab.title">
          <span class="tab-separator"> · </span>
          <span class="tab-title">{{ item.tab.title }}</span>
        </template>
      </span>
      <span v-if="item.badge" class="tab-badge">{{ item.badge }}</span>
      <button
        type="button"
        class="tab-close"
        :aria-label="$gettext('Close')"
        @click.stop="emit('close', item.tab.id)"
      >
        <CloseOutlined />
      </button>
    </div>
  </div>
</template>

<style scoped lang="less">
.workspace-tabs {
  display: flex;
  // Tabs fill the whole bar height so their text lines up with the buttons
  // beside the strip, which are centred in the same bar.
  align-items: stretch;
  gap: 2px;
  min-width: 0;
  height: 100%;
  overflow-x: auto;
  overflow-y: hidden;
  scrollbar-width: none;

  &::-webkit-scrollbar {
    display: none;
  }
}

.workspace-tab {
  // A real width, not just a flex-basis: the strip sizes itself from its
  // content, and a basis alone would let each tab collapse to its label.
  // Tabs start wide and only shrink, all together, once the strip is full.
  flex: 0 1 auto;
  width: 240px;
  min-width: 120px;
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 0 4px 0 10px;
  color: rgba(0, 0, 0, 0.65);
  font-size: 13px;
  cursor: pointer;
  user-select: none;
  transition: background-color 0.15s, color 0.15s;

  &:hover {
    background: rgba(0, 0, 0, 0.04);
  }

  &:focus-visible {
    outline: 2px solid #1677ff;
    outline-offset: -2px;
  }

  // On screen in a pane. The tab of the pane that tab clicks go to is the
  // solid one; the other visible tab stays half filled. This is the focus cue:
  // a colored line over the pane read as a page-loading bar.
  &.shown {
    background: rgba(255, 255, 255, 0.55);
    color: rgba(0, 0, 0, 0.88);
  }

  &.focused {
    background: #ffffff;
  }
}

.tab-dot {
  flex: none;
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.tab-label {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tab-node {
  font-weight: 500;
}

.tab-separator, .tab-title {
  opacity: 0.75;
}

.tab-badge {
  flex: none;
  min-width: 16px;
  height: 16px;
  padding: 0 3px;
  border-radius: 4px;
  background: rgba(0, 0, 0, 0.06);
  font-size: 11px;
  font-weight: 600;
  line-height: 16px;
  text-align: center;
}

.tab-close {
  flex: none;
  width: 20px;
  height: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: inherit;
  font-size: 10px;
  opacity: 0.55;
  cursor: pointer;

  &:hover {
    opacity: 1;
    background: rgba(0, 0, 0, 0.08);
  }

  &:focus-visible {
    opacity: 1;
    outline: 2px solid #1677ff;
    outline-offset: -2px;
  }
}

.dark {
  .workspace-tab {
    color: rgba(255, 255, 255, 0.65);

    &:hover {
      background: rgba(255, 255, 255, 0.06);
    }

    &.shown {
      background: rgba(20, 20, 20, 0.55);
      color: rgba(255, 255, 255, 0.88);
    }

    &.focused {
      background: #141414;
    }
  }

  .tab-badge {
    background: rgba(255, 255, 255, 0.12);
  }

  .tab-close:hover {
    background: rgba(255, 255, 255, 0.12);
  }
}
</style>
