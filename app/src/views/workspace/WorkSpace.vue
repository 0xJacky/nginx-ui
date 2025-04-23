<script lang="ts" setup>
import { CloseOutlined } from '@ant-design/icons-vue'
import { Pane, Splitpanes } from 'splitpanes'
import { useRouter } from 'vue-router'
import 'splitpanes/dist/splitpanes.css'

const router = useRouter()

const src = computed(() => {
  return location.pathname
})

const paneSize = ref(localStorage.paneSize ?? 50) // Read from persistent localStorage.
function storePaneSize({ prevPane }) {
  localStorage.paneSize = prevPane.size // Store in persistent localStorage.
}

function closeSplitView() {
  router.push('/')
}

const leftFrame = useTemplateRef('leftFrame')
const rightFrame = useTemplateRef('rightFrame')

function handleLoad(iframeRef: HTMLIFrameElement | null) {
  if (!iframeRef) {
    return
  }

  iframeRef.addEventListener('load', () => {
    if (iframeRef.contentWindow) {
      iframeRef.contentWindow.inWorkspace = true
    }
  })
}

onMounted(() => {
  handleLoad(leftFrame.value)
  handleLoad(rightFrame.value)
})
</script>

<template>
  <div class="h-100vh macos-window">
    <div class="macos-titlebar flex items-center p-2 relative">
      <div class="traffic-lights flex ml-2">
        <div class="traffic-light close" @click="closeSplitView">
          <CloseOutlined class="traffic-icon" />
        </div>
      </div>
      <div class="window-title absolute left-0 right-0 text-center">
        {{ $gettext('Workspace') }}
      </div>
    </div>

    <Splitpanes class="default-theme split-container" @resized="storePaneSize">
      <Pane :size="paneSize" :min-size="20">
        <iframe ref="leftFrame" name="split-view-left" :src class="w-full h-full iframe-no-border" />
      </Pane>
      <Pane :size="100 - paneSize" :min-size="20">
        <iframe ref="rightFrame" name="split-view-right" :src class="w-full h-full iframe-no-border" />
      </Pane>
    </Splitpanes>
  </div>
</template>

<style scoped>
.macos-window {
  border-radius: 8px;
  overflow: hidden;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.1);
}

.macos-titlebar {
  background: linear-gradient(to bottom, #f9f9f9, #ececec);
  height: 32px;
  border-bottom: 1px solid #e1e1e1;
  -webkit-app-region: drag;
  user-select: none;
}

.dark .macos-titlebar {
  background: linear-gradient(to bottom, #323232, #282828);
  border-bottom: 1px solid #3a3a3a;
}

.split-container {
  height: calc(100vh - 32px);
}

.traffic-lights {
  -webkit-app-region: no-drag;
  z-index: 10;
}

.traffic-light {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  margin-right: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
}

.traffic-light.close {
  background-color: #ff5f57;
  border: 1px solid #e0443e;
}

.traffic-icon {
  opacity: 0;
  font-size: 9px;
  color: rgba(0, 0, 0, 0.5);
}

.traffic-light:hover .traffic-icon {
  opacity: 1;
}

.window-title {
  font-size: 13px;
  font-weight: 500;
  color: #333;
  pointer-events: none;
}

.dark .window-title {
  color: #e0e0e0;
}

:deep(.splitpanes__splitter) {
  background-color: #ececec !important;
}

.iframe-no-border {
  border: none;
  outline: none;
}
</style>
