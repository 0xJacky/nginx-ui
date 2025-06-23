<script setup lang="ts">
import type { Ace } from 'ace-builds'
import type { ConfigBackup } from '@/api/config'
import ace from 'ace-builds'
// Import required modules
import extLanguageToolsUrl from 'ace-builds/src-min-noconflict/ext-language_tools?url'
import { formatDateTime } from '@/lib/helper'
import 'ace-builds/src-noconflict/mode-nginx'
import 'ace-builds/src-noconflict/theme-monokai'

const props = defineProps<{
  records: ConfigBackup[]
}>()

const emit = defineEmits<{
  (e: 'restore'): void
}>()

// Import Range class separately to avoid loading the entire ace package
const Range = ace.Range

// Define modal visibility and current content using defineModel
const visible = defineModel<boolean>('visible')
const currentContent = defineModel<string>('currentContent')

const originalText = ref('')
const modifiedText = ref('')
const diffEditorRef = ref<HTMLElement | null>(null)
const editors: { left?: Ace.Editor, right?: Ace.Editor } = {}
const originalTitle = ref('')
const modifiedTitle = ref('')
const errorMessage = ref('')

// Initialize ace language tools
onMounted(() => {
  try {
    ace.config.setModuleUrl('ace/ext/language_tools', extLanguageToolsUrl)
  }
  catch (error) {
    console.error('Failed to initialize Ace editor language tools:', error)
  }
})

// Set editor content based on selected records
function setContent() {
  if (!props.records?.length) {
    errorMessage.value = $gettext('No records selected')
    return false
  }

  try {
    if (props.records.length === 1) {
      // Compare single record with current content
      originalText.value = props.records[0]?.content || ''
      modifiedText.value = currentContent.value || ''
      originalTitle.value = `${props.records[0]?.name || ''} (${formatDateTime(props.records[0]?.created_at || '')})`
      modifiedTitle.value = $gettext('Current Content')
    }
    else if (props.records.length === 2) {
      // Compare two records - sort by time
      const sorted = [...props.records].sort((a, b) =>
        new Date(a.created_at).getTime() - new Date(b.created_at).getTime(),
      )
      originalText.value = sorted[0]?.content || ''
      modifiedText.value = sorted[1]?.content || ''
      originalTitle.value = `${sorted[0]?.name || ''} (${formatDateTime(sorted[0]?.created_at || '')})`
      modifiedTitle.value = `${sorted[1]?.name || ''} (${formatDateTime(sorted[1]?.created_at || '')})`
    }

    // Ensure both sides have content for comparison
    if (!originalText.value || !modifiedText.value) {
      errorMessage.value = $gettext('Cannot compare: Missing content')
      return false
    }

    errorMessage.value = ''
    return true
  }
  catch (error) {
    console.error('Error setting content:', error)
    errorMessage.value = $gettext('Error processing content')
    return false
  }
}

// Create side-by-side editors
function createEditors() {
  if (!diffEditorRef.value)
    return false

  try {
    diffEditorRef.value.innerHTML = ''

    // Create containers for left and right editors
    const leftContainer = document.createElement('div')
    const rightContainer = document.createElement('div')

    Object.assign(leftContainer.style, {
      width: '50%',
      height: '100%',
      float: 'left',
      position: 'relative',
    })

    Object.assign(rightContainer.style, {
      width: '50%',
      height: '100%',
      float: 'right',
      position: 'relative',
    })

    diffEditorRef.value.appendChild(leftContainer)
    diffEditorRef.value.appendChild(rightContainer)

    // Initialize editors with common settings
    const editorConfig = {
      theme: 'ace/theme/monokai',
      mode: 'ace/mode/nginx',
      readOnly: true,
      showPrintMargin: false,
    }

    editors.left = ace.edit(leftContainer)
    editors.right = ace.edit(rightContainer)

    // Apply settings to both editors
    ;[editors.left, editors.right].forEach(editor => {
      editor.setTheme(editorConfig.theme)
      editor.getSession().setMode(editorConfig.mode)
      editor.setReadOnly(editorConfig.readOnly)
      editor.setOption('showPrintMargin', editorConfig.showPrintMargin)
    })

    return true
  }
  catch (error) {
    console.error('Error creating editors:', error)
    errorMessage.value = $gettext('Error initializing diff viewer')
    return false
  }
}

// Update editor content and setup highlighting
function updateEditors() {
  if (!editors.left || !editors.right) {
    console.error('Editors not available')
    return false
  }

  try {
    // Set content
    editors.left.setValue(originalText.value, -1)
    editors.right.setValue(modifiedText.value, -1)

    // Scroll to top and clear selection
    ;[editors.left, editors.right].forEach(editor => {
      editor.scrollToLine(0, false, false)
      editor.clearSelection()
    })

    // Balance heights and setup features
    balanceEditorHeights()
    highlightDiffs()
    setupSyncScroll()

    return true
  }
  catch (error) {
    console.error('Error updating editors:', error)
    return false
  }
}

// Balance editor heights by padding shorter content
function balanceEditorHeights() {
  if (!editors.left || !editors.right)
    return

  try {
    const leftSession = editors.left.getSession()
    const rightSession = editors.right.getSession()
    const leftLineCount = leftSession.getLength()
    const rightLineCount = rightSession.getLength()

    if (leftLineCount === rightLineCount)
      return

    // Add padding lines to make editors same height
    const maxLines = Math.max(leftLineCount, rightLineCount)
    const leftPadding = maxLines - leftLineCount
    const rightPadding = maxLines - rightLineCount

    if (leftPadding > 0) {
      const content = leftSession.getValue()
      leftSession.setValue(content + '\n'.repeat(leftPadding))
      leftSession.selection.clearSelection()
    }

    if (rightPadding > 0) {
      const content = rightSession.getValue()
      rightSession.setValue(content + '\n'.repeat(rightPadding))
      rightSession.selection.clearSelection()
    }
  }
  catch (error) {
    console.warn('Error balancing editor heights:', error)
  }
}

// Add diff highlighting styles
function addHighlightStyles() {
  const styleId = 'diff-highlight-style'
  if (document.getElementById(styleId))
    return

  const style = document.createElement('style')
  style.id = styleId
  style.textContent = `
    .diff-line-deleted {
      position: absolute;
      background: rgba(255, 100, 100, 0.3);
      z-index: 5;
      width: 100% !important;
    }
    .diff-line-added {
      position: absolute;
      background: rgba(100, 255, 100, 0.3);
      z-index: 5;
      width: 100% !important;
    }
  `
  document.head.appendChild(style)
}

// Highlight differences between editors
function highlightDiffs() {
  if (!editors.left || !editors.right)
    return

  try {
    const leftSession = editors.left.getSession()
    const rightSession = editors.right.getSession()

    // Clear previous highlights
    leftSession.clearBreakpoints()
    rightSession.clearBreakpoints()

    addHighlightStyles()

    // Compare lines and highlight differences
    const leftLines = originalText.value.split('\n')
    const rightLines = modifiedText.value.split('\n')
    const matchedLeft = new Set<number>()
    const matchedRight = new Set<number>()

    // Mark identical lines
    leftLines.forEach((leftLine, i) => {
      const rightIndex = rightLines.findIndex((rightLine, j) =>
        rightLine === leftLine && !matchedRight.has(j),
      )
      if (rightIndex !== -1) {
        matchedLeft.add(i)
        matchedRight.add(rightIndex)
      }
    })

    // Highlight unmatched lines
    leftLines.forEach((line, i) => {
      if (!matchedLeft.has(i)) {
        leftSession.addGutterDecoration(i, 'ace_gutter-active-line')
        leftSession.addMarker(new Range(i, 0, i, line.length || 1), 'diff-line-deleted', 'fullLine')
      }
    })

    rightLines.forEach((line, j) => {
      if (!matchedRight.has(j)) {
        rightSession.addGutterDecoration(j, 'ace_gutter-active-line')
        rightSession.addMarker(new Range(j, 0, j, line.length || 1), 'diff-line-added', 'fullLine')
      }
    })
  }
  catch (error) {
    console.error('Error highlighting diffs:', error)
  }
}

// Setup synchronized scrolling
function setupSyncScroll() {
  if (!editors.left || !editors.right)
    return

  const leftSession = editors.left.getSession()
  const rightSession = editors.right.getSession()
  let isScrolling = false

  const syncScroll = (source: Ace.EditSession, target: Ace.EditSession) => (scrollTop: number) => {
    if (isScrolling)
      return
    isScrolling = true
    target.setScrollTop(scrollTop)
    setTimeout(() => {
      isScrolling = false
    }, 10)
  }

  leftSession.on('changeScrollTop', syncScroll(leftSession, rightSession))
  rightSession.on('changeScrollTop', syncScroll(rightSession, leftSession))
}

// Initialize diff viewer
async function initDiffViewer() {
  if (!diffEditorRef.value)
    return

  errorMessage.value = ''

  if (!setContent() || !createEditors()) {
    console.error('Failed to initialize diff viewer')
    return
  }

  await nextTick()

  if (!updateEditors()) {
    console.error('Failed to update editors')
    return
  }

  // Resize editors after a short delay
  setTimeout(() => {
    if (editors.left && editors.right) {
      editors.left.resize()
      editors.right.resize()
    }
  }, 200)
}

// Watch for changes and reinitialize
watch(() => [props.records, visible.value], async () => {
  if (visible.value) {
    await nextTick()
    initDiffViewer()
  }
})

// Close dialog
function handleClose() {
  visible.value = false
  errorMessage.value = ''
}

// Restore original content
function restoreContent() {
  if (originalText.value) {
    currentContent.value = originalText.value
    handleClose()
    emit('restore')
  }
}

// Restore modified content
function restoreModifiedContent() {
  if (modifiedText.value && props.records.length === 2) {
    currentContent.value = modifiedText.value
    handleClose()
  }
}
</script>

<template>
  <AModal
    v-model:open="visible"
    :title="$gettext('Compare Configurations')"
    width="100vw"
    :style="{ height: '90vh' }"
    :footer="null"
    :destroy-on-close="true"
    centered
    @cancel="handleClose"
  >
    <div class="diff-container">
      <AAlert
        v-if="errorMessage"
        :message="errorMessage"
        type="warning"
        show-icon
        class="diff-error"
      />

      <template v-else>
        <div class="diff-header">
          <div class="diff-title-container">
            <div class="diff-title">
              {{ originalTitle }}
            </div>
            <AButton
              type="link"
              size="small"
              @click="restoreContent"
            >
              {{ $gettext('Restore this version') }}
            </AButton>
          </div>
          <div class="diff-title-container">
            <div class="diff-title">
              {{ modifiedTitle }}
            </div>
            <AButton
              v-if="props.records.length === 2"
              type="link"
              size="small"
              @click="restoreModifiedContent"
            >
              {{ $gettext('Restore this version') }}
            </AButton>
          </div>
        </div>
        <div
          ref="diffEditorRef"
          class="diff-editor"
        />
      </template>
    </div>
  </AModal>
</template>

<style lang="less" scoped>
.diff-container {
  display: flex;
  flex-direction: column;
  height: calc(90vh - 120px);
}

.diff-error {
  margin-bottom: 16px;
}

.diff-header {
  display: flex;
  justify-content: space-between;
  margin-bottom: 8px;
  flex-shrink: 0;
}

.diff-title-container {
  display: flex;
  align-items: center;
  width: 50%;
  gap: 8px;
}

.diff-title {
  padding: 0 8px;
  font-weight: 500;
}

.diff-editor {
  flex: 1;
  width: 100%;
  border: 1px solid #ddd;
  border-radius: 4px;
  overflow: hidden;
  min-height: 0;
}
</style>
