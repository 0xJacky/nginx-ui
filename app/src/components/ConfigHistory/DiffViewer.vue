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

// Define modal visibility using defineModel with boolean type
const visible = defineModel<boolean>('visible')
// Define currentContent using defineModel
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

// Check if there is content to display
function hasContent() {
  return originalText.value && modifiedText.value
}

// Set editor content based on selected records
function setContent() {
  if (!props.records || props.records.length === 0) {
    errorMessage.value = $gettext('No records selected')
    return false
  }

  try {
    // Set content based on number of selected records
    if (props.records.length === 1) {
      // Single record - compare with current content
      originalText.value = props.records[0]?.content || ''
      modifiedText.value = currentContent.value || ''

      // Ensure both sides have content for comparison
      if (!originalText.value || !modifiedText.value) {
        errorMessage.value = $gettext('Cannot compare: Missing content')
        return false
      }

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

      // Ensure both sides have content for comparison
      if (!originalText.value || !modifiedText.value) {
        errorMessage.value = $gettext('Cannot compare: Missing content')
        return false
      }

      originalTitle.value = `${sorted[0]?.name || ''} (${formatDateTime(sorted[0]?.created_at || '')})`
      modifiedTitle.value = `${sorted[1]?.name || ''} (${formatDateTime(sorted[1]?.created_at || '')})`
    }

    errorMessage.value = ''
    return hasContent()
  }
  catch (error) {
    console.error('Error setting content:', error)
    errorMessage.value = $gettext('Error processing content')
    return false
  }
}

// Create editors
function createEditors() {
  if (!diffEditorRef.value)
    return false

  try {
    // Clear editor area
    diffEditorRef.value.innerHTML = ''

    // Create left and right editor containers
    const leftContainer = document.createElement('div')
    leftContainer.style.width = '50%'
    leftContainer.style.height = '100%'
    leftContainer.style.float = 'left'
    leftContainer.style.position = 'relative'

    const rightContainer = document.createElement('div')
    rightContainer.style.width = '50%'
    rightContainer.style.height = '100%'
    rightContainer.style.float = 'right'
    rightContainer.style.position = 'relative'

    // Add to DOM
    diffEditorRef.value.appendChild(leftContainer)
    diffEditorRef.value.appendChild(rightContainer)

    // Create editors
    editors.left = ace.edit(leftContainer)
    editors.left.setTheme('ace/theme/monokai')
    editors.left.getSession().setMode('ace/mode/nginx')
    editors.left.setReadOnly(true)
    editors.left.setOption('showPrintMargin', false)

    editors.right = ace.edit(rightContainer)
    editors.right.setTheme('ace/theme/monokai')
    editors.right.getSession().setMode('ace/mode/nginx')
    editors.right.setReadOnly(true)
    editors.right.setOption('showPrintMargin', false)

    return true
  }
  catch (error) {
    console.error('Error creating editors:', error)
    errorMessage.value = $gettext('Error initializing diff viewer')
    return false
  }
}

// Update editor content
function updateEditors() {
  if (!editors.left || !editors.right) {
    console.error('Editors not available')
    return false
  }

  try {
    // Check if content is empty
    if (!originalText.value || !modifiedText.value) {
      console.error('Empty content detected', {
        originalLength: originalText.value?.length,
        modifiedLength: modifiedText.value?.length,
      })
      return false
    }

    // Set content
    editors.left.setValue(originalText.value, -1)
    editors.right.setValue(modifiedText.value, -1)

    // Scroll to top
    editors.left.scrollToLine(0, false, false)
    editors.right.scrollToLine(0, false, false)

    // Highlight differences
    highlightDiffs()

    // Setup sync scroll
    setupSyncScroll()

    return true
  }
  catch (error) {
    console.error('Error updating editors:', error)
    return false
  }
}

// Highlight differences
function highlightDiffs() {
  if (!editors.left || !editors.right)
    return

  try {
    const leftSession = editors.left.getSession()
    const rightSession = editors.right.getSession()

    // Clear previous all marks
    leftSession.clearBreakpoints()
    rightSession.clearBreakpoints()

    // Add CSS styles
    addHighlightStyles()

    // Compare lines
    const leftLines = originalText.value.split('\n')
    const rightLines = modifiedText.value.split('\n')

    // Use difference comparison algorithm
    compareAndHighlightLines(leftSession, rightSession, leftLines, rightLines)
  }
  catch (error) {
    console.error('Error highlighting diffs:', error)
  }
}

// Add highlight styles
function addHighlightStyles() {
  const styleId = 'diff-highlight-style'
  if (!document.getElementById(styleId)) {
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
      .diff-line-changed {
        position: absolute;
        background: rgba(255, 255, 100, 0.3);
        z-index: 5;
        width: 100% !important;
      }
    `
    document.head.appendChild(style)
  }
}

// Compare and highlight lines
function compareAndHighlightLines(leftSession: Ace.EditSession, rightSession: Ace.EditSession, leftLines: string[], rightLines: string[]) {
  // Create a mapping table to track which lines have been matched
  const matchedLeftLines = new Set<number>()
  const matchedRightLines = new Set<number>()

  // 1. First mark completely identical lines
  for (let i = 0; i < leftLines.length; i++) {
    for (let j = 0; j < rightLines.length; j++) {
      if (leftLines[i] === rightLines[j] && !matchedLeftLines.has(i) && !matchedRightLines.has(j)) {
        matchedLeftLines.add(i)
        matchedRightLines.add(j)
        break
      }
    }
  }

  // 2. Mark lines left deleted
  for (let i = 0; i < leftLines.length; i++) {
    if (!matchedLeftLines.has(i)) {
      leftSession.addGutterDecoration(i, 'ace_gutter-active-line')
      leftSession.addMarker(
        new Range(i, 0, i, leftLines[i].length || 1),
        'diff-line-deleted',
        'fullLine',
      )
    }
  }

  // 3. Mark lines right added
  for (let j = 0; j < rightLines.length; j++) {
    if (!matchedRightLines.has(j)) {
      rightSession.addGutterDecoration(j, 'ace_gutter-active-line')
      rightSession.addMarker(
        new Range(j, 0, j, rightLines[j].length || 1),
        'diff-line-added',
        'fullLine',
      )
    }
  }
}

// Setup sync scroll
function setupSyncScroll() {
  if (!editors.left || !editors.right)
    return

  // Sync scroll
  const leftSession = editors.left.getSession()
  const rightSession = editors.right.getSession()

  leftSession.on('changeScrollTop', (scrollTop: number) => {
    rightSession.setScrollTop(scrollTop)
  })

  rightSession.on('changeScrollTop', (scrollTop: number) => {
    leftSession.setScrollTop(scrollTop)
  })
}

// Initialize difference comparator
async function initDiffViewer() {
  if (!diffEditorRef.value)
    return

  // Reset error message
  errorMessage.value = ''

  // Set content
  const hasValidContent = setContent()
  if (!hasValidContent) {
    console.error('No valid content to compare')
    return
  }

  // Create editors
  const editorsCreated = createEditors()
  if (!editorsCreated) {
    console.error('Failed to create editors')
    return
  }

  // Wait for DOM update
  await nextTick()

  // Update editor content
  const editorsUpdated = updateEditors()
  if (!editorsUpdated) {
    console.error('Failed to update editors')
    return
  }

  // Adjust size to ensure full display
  window.setTimeout(() => {
    if (editors.left && editors.right) {
      editors.left.resize()
      editors.right.resize()
    }
  }, 200)
}

// Listen for records change
watch(() => [props.records, visible.value], async () => {
  if (visible.value) {
    // When selected records change, update content
    await nextTick()
    initDiffViewer()
  }
})

// Close dialog handler
function handleClose() {
  visible.value = false
  errorMessage.value = ''
}

// Add restore functionality
function restoreContent() {
  if (originalText.value) {
    // Update current content with history version
    currentContent.value = originalText.value
    // Close dialog
    handleClose()
    emit('restore')
  }
}

// Add restore functionality for modified content
function restoreModifiedContent() {
  if (modifiedText.value && props.records.length === 2) {
    // Update current content with the modified version
    currentContent.value = modifiedText.value
    // Close dialog
    handleClose()
  }
}
</script>

<template>
  <AModal
    v-model:open="visible"
    :title="$gettext('Compare Configurations')"
    width="100%"
    :footer="null"
    @cancel="handleClose"
  >
    <div v-if="errorMessage" class="diff-error">
      <AAlert
        :message="errorMessage"
        type="warning"
        show-icon
      />
    </div>

    <div v-else class="diff-container">
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
    </div>
  </AModal>
</template>

<style lang="less" scoped>
.diff-container {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.diff-error {
  margin-bottom: 16px;
}

.diff-header {
  display: flex;
  justify-content: space-between;
  margin-bottom: 8px;
}

.diff-title-container {
  display: flex;
  align-items: center;
  width: 50%;
  gap: 8px;
}

.diff-title {
  padding: 0 8px;
}

.diff-editor {
  height: 500px;
  width: 100%;
  border: 1px solid #ddd;
  border-radius: 4px;
  overflow: hidden;
}
</style>
