<script setup lang="ts">
import type { AnalyticNode } from '@/api/node'
import type { NodeSwitchTarget } from '@/composables/useNodeSwitch'
import type { NodeSwitchBlocker } from '@/lib/node/switch'
import { SettingOutlined } from '@antdv-next/icons'
import { storeToRefs } from 'pinia'
import nodeApi from '@/api/node'
import { useNodeSwitch } from '@/composables/useNodeSwitch'
import { filterSwitchNodes, getNodeSwitchBlocker, sortSwitchNodes } from '@/lib/node/switch'
import { isWorkspacePane } from '@/lib/workspace/env'
import { useSettingsStore } from '@/pinia'
import { version } from '@/version.json'

const props = defineProps<{
  /**
   * Pick a node instead of switching to it: rows emit `select`, nothing is
   * marked current and the management link is hidden. Used by the workspace.
   */
  picker?: boolean
}>()

const emit = defineEmits<{
  select: [node: NodeSwitchTarget]
}>()

// Below this many nodes the whole list fits, and a search box is just noise.
const SEARCH_THRESHOLD = 8

const router = useRouter()
const settingsStore = useSettingsStore()
const { node, server_name } = storeToRefs(settingsStore)
const { switchNode, switchToLocal } = useNodeSwitch()

const open = defineModel<boolean>('open', { default: false })
const keyword = ref('')
const nodes = ref<AnalyticNode[]>([])
const isLoading = ref(false)
const isSwitching = ref(false)

const isLocal = computed(() => node.value.id === 0)
const currentId = computed(() => props.picker ? null : node.value.id)
// Opening a split from inside a workspace pane would nest workspaces.
const canSplit = computed(() => !props.picker && !isWorkspacePane)
const localName = computed(() => server_name.value || $gettext('Local'))
const showSearch = computed(() => nodes.value.length > SEARCH_THRESHOLD)

const options = computed(() => sortSwitchNodes(filterSwitchNodes(nodes.value, keyword.value))
  .map(item => ({ node: item, blocker: getNodeSwitchBlocker(item, version) })))

function blockerLabel(blocker: NodeSwitchBlocker, item: AnalyticNode) {
  switch (blocker) {
    case 'offline':
      return $gettext('Offline')
    case 'version_mismatch':
      return item.version
    case 'version_unknown':
      return $gettext('Unknown')
  }
}

// Fetched from this server on every open: while a remote node is selected, the
// live node store holds the remote node's own list instead of ours.
async function loadNodes() {
  isLoading.value = true
  try {
    nodes.value = await nodeApi.getLocalList()
  }
  catch (error) {
    console.error('Failed to load nodes:', error)
  }
  finally {
    isLoading.value = false
  }
}

watch(open, isOpen => {
  if (!isOpen)
    return

  keyword.value = ''
  loadNodes()
})

async function pick(target: AnalyticNode, blocker: NodeSwitchBlocker | null) {
  if (blocker || isSwitching.value)
    return

  open.value = false
  if (props.picker) {
    emit('select', { id: target.id, name: target.name })
    return
  }

  isSwitching.value = true
  await switchNode({ id: target.id, name: target.name })
}

async function pickLocal() {
  open.value = false
  if (props.picker) {
    emit('select', { id: 0, name: 'Local' })
    return
  }

  if (isLocal.value || isSwitching.value)
    return

  isSwitching.value = true
  await switchToLocal()
}

/** Opens the workspace with the current node on the left and `targetId` on the right. */
function openSplit(targetId: number) {
  open.value = false
  router.push({ path: '/workspace', query: { l: String(node.value.id), r: String(targetId) } })
}

function pickFirstMatch() {
  const match = options.value.find(option => !option.blocker)
  if (match)
    pick(match.node, null)
}
</script>

<template>
  <APopover
    v-model:open="open"
    trigger="click"
    placement="bottomLeft"
    :arrow="false"
    :styles="{ root: { width: '280px' }, container: { padding: '4px', width: '280px' } }"
  >
    <template #content>
      <div class="node-switcher">
        <AInput
          v-if="showSearch"
          v-model:value="keyword"
          class="node-search"
          size="small"
          allow-clear
          :placeholder="$gettext('Search')"
          @press-enter="pickFirstMatch"
        />

        <div class="option-list">
          <div class="node-row" :class="{ splittable: canSplit }">
            <button
              type="button"
              class="node-option"
              :class="{ current: currentId === 0 }"
              @click="pickLocal"
            >
              <span class="status-dot local" />
              <span class="option-text">
                <span class="option-name">{{ localName }}</span>
                <span v-if="server_name" class="node-url">{{ $gettext('Local') }}</span>
              </span>
            </button>
            <button
              v-if="canSplit"
              type="button"
              class="split-action"
              :title="$gettext('Open in split view')"
              :aria-label="$gettext('Open in split view')"
              @click="openSplit(0)"
            >
              <span class="i-tabler-layout-columns text-base" />
            </button>
          </div>

          <div v-if="nodes.length" class="group-label">
            {{ $gettext('Nodes') }}
          </div>

          <ATooltip
            v-for="option in options"
            :key="option.node.id"
            placement="right"
            :title="option.blocker === 'version_mismatch'
              ? $gettext('The remote Nginx UI version is not compatible with the local Nginx UI version. '
                + 'To avoid potential errors, please upgrade the remote Nginx UI to match the local version.')
              : undefined"
          >
            <div class="node-row" :class="{ splittable: canSplit && !option.blocker }">
              <button
                type="button"
                class="node-option"
                :class="{ current: option.node.id === currentId, blocked: option.blocker }"
                :aria-disabled="!!option.blocker"
                @click="pick(option.node, option.blocker)"
              >
                <span class="status-dot" :class="{ offline: !option.node.status }" />
                <span class="option-text">
                  <span class="option-name">{{ option.node.name }}</span>
                  <span class="node-url">{{ option.node.url }}</span>
                </span>
                <ATag
                  v-if="option.blocker"
                  class="blocked-reason"
                  :color="option.blocker === 'offline' ? 'error' : 'warning'"
                  variant="filled"
                >
                  {{ blockerLabel(option.blocker, option.node) }}
                </ATag>
              </button>
              <button
                v-if="canSplit && !option.blocker"
                type="button"
                class="split-action"
                :title="$gettext('Open in split view')"
                :aria-label="$gettext('Open in split view')"
                @click="openSplit(option.node.id)"
              >
                <span class="i-tabler-layout-columns text-base" />
              </button>
            </div>
          </ATooltip>

          <div v-if="isLoading && !nodes.length" class="list-state">
            <ASpin size="small" />
          </div>
          <div v-else-if="keyword && !options.length" class="list-state">
            {{ $gettext('No data') }}
          </div>
        </div>

        <RouterLink
          v-if="isLocal && !picker"
          to="/nodes"
          class="manage-link"
          @click="open = false"
        >
          <SettingOutlined />
          {{ $gettext('Nodes') }}
        </RouterLink>
      </div>
    </template>

    <slot :open="open" />
  </APopover>
</template>

<style scoped lang="less">
// The list lines up with the sidebar pill (NodeIndicator) it opens from: dots
// centered under the pill icon, names starting at the left of the pill's text area. The
// popover sits flush with the pill's left edge, so the offsets below are
// measured from that shared edge; other triggers just get the same layout.
@pill-border: 1px;
@pill-inset: 15px;
@icon-size: 14px;
@pill-gap: 8px;
@panel-padding: 4px; // keep in sync with the popover container padding
@dot-size: 8px;
@text-inset: @pill-border + @pill-inset + @icon-size + @pill-gap;
@option-inset: @pill-border + @pill-inset + ((@icon-size - @dot-size) / 2) - @panel-padding;
@option-gap: @text-inset - @panel-padding - @option-inset - @dot-size;

.node-switcher {
  display: flex;
  flex-direction: column;

  .node-search {
    margin: 4px 4px 6px;
    width: auto;
  }

  .option-list {
    max-height: 320px;
    overflow-y: auto;
  }

  .group-label {
    font-size: 12px;
    opacity: 0.55;
    padding: 8px 8px 2px (@text-inset - @panel-padding);
  }

  // The split action sits over the right end of a row and shows on hover.
  .node-row {
    position: relative;

    // Room for the split action, so it never covers the node name.
    &.splittable .node-option {
      padding-right: 36px;
    }

    &:hover .split-action, .split-action:focus-visible {
      opacity: 1;
    }
  }

  .split-action {
    position: absolute;
    top: 50%;
    right: 6px;
    transform: translateY(-50%);
    width: 24px;
    height: 24px;
    display: flex;
    align-items: center;
    justify-content: center;
    border: 0;
    border-radius: 4px;
    background: transparent;
    color: inherit;
    cursor: pointer;
    opacity: 0;
    transition: opacity 0.15s;

    &:hover {
      background: rgba(0, 0, 0, 0.06);
    }

    &:focus-visible {
      outline: 2px solid #1677ff;
      outline-offset: -2px;
    }
  }

  .node-option {
    width: 100%;
    display: flex;
    align-items: center;
    gap: @option-gap;
    padding: 6px 8px 6px @option-inset;
    border: 0;
    border-radius: 6px;
    background: transparent;
    color: inherit;
    font: inherit;
    text-align: left;
    cursor: pointer;

    &:hover:not(.blocked) {
      background: rgba(0, 0, 0, 0.04);
    }

    &:focus-visible {
      outline: 2px solid #1677ff;
      outline-offset: -2px;
    }

    &.current {
      background: rgba(22, 119, 255, 0.08);
    }

    &.blocked {
      cursor: not-allowed;

      .option-name {
        opacity: 0.5;
      }
    }

    .option-text {
      flex: 1;
      min-width: 0;
      display: flex;
      flex-direction: column;
      line-height: 20px;
    }

    .option-name {
      font-weight: 500;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .node-url {
      font-size: 12px;
      line-height: 18px;
      opacity: 0.55;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .blocked-reason {
      flex: none;
      margin-right: 0;
    }
  }

  .status-dot {
    flex: none;
    width: @dot-size;
    height: @dot-size;
    border-radius: 50%;
    background: #52c41a;

    &.offline {
      background: #ff4d4f;
    }

    &.local {
      background: #1677ff;
    }
  }

  .list-state {
    display: flex;
    justify-content: center;
    padding: 12px;
    opacity: 0.55;
  }

  .manage-link {
    display: flex;
    align-items: center;
    gap: @pill-gap;
    margin-top: 4px;
    padding: 8px 8px 4px (@pill-border + @pill-inset - @panel-padding);
    border-top: 1px solid rgba(5, 5, 5, 0.06);
    color: inherit;
    opacity: 0.75;

    &:hover {
      opacity: 1;
    }
  }
}

.dark .node-switcher {
  .split-action:hover {
    background: rgba(255, 255, 255, 0.12);
  }

  .node-option {
    &:hover:not(.blocked) {
      background: rgba(255, 255, 255, 0.08);
    }

    &.current {
      background: rgba(22, 119, 255, 0.16);
    }
  }

  .manage-link {
    border-top-color: rgba(253, 253, 253, 0.12);
  }
}
</style>
