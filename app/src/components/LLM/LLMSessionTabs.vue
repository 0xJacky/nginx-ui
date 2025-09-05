<script setup lang="ts">
import {
  CloseOutlined,
  CopyOutlined,
  DeleteOutlined,
  EditOutlined,
  HistoryOutlined,
  MoreOutlined,
  PlusOutlined,
} from '@ant-design/icons-vue'
import { storeToRefs } from 'pinia'
import { useLLMStore } from './llm'
import { useLLMSessionStore } from './sessionStore'

const props = defineProps<{
  path?: string
  type?: 'terminal' | 'nginx'
}>()

const emit = defineEmits<{
  newSessionCreated: []
  sessionCleared: []
}>()

const sessionStore = useLLMSessionStore()
const llmStore = useLLMStore()
const { sortedSessions, activeSessionId } = storeToRefs(sessionStore)
const { loading: llmLoading } = storeToRefs(llmStore)

const editingSessionId = ref<string | null>(null)
const editingTitle = ref('')
const historyDrawerVisible = ref(false)
const sessionsDropdownVisible = ref(false)
const searchText = ref('')

// Only show first 3 sessions in tabs, rest in history
const visibleSessions = computed(() => sortedSessions.value.slice(0, 3))

// Filtered sessions for dropdown search
const filteredSessions = computed(() => {
  if (!searchText.value.trim()) {
    return sortedSessions.value
  }
  return sortedSessions.value.filter(session =>
    session.title.toLowerCase().includes(searchText.value.toLowerCase()),
  )
})

async function createNewSession() {
  if (llmLoading.value) {
    return // Don't create new session while LLM is generating output
  }

  const title = props.type === 'terminal' ? 'Terminal Assistant' : 'New Chat'
  try {
    const session = await sessionStore.createSession(title, props.path, props.type)
    await llmStore.switchSession(session.session_id)
    emit('newSessionCreated')
  }
  catch (error) {
    console.error('Failed to create session:', error)
  }
}

async function selectSession(sessionId: string) {
  if (sessionId === activeSessionId.value)
    return

  if (llmLoading.value) {
    return // Don't switch sessions while LLM is generating output
  }

  await llmStore.switchSession(sessionId)
  sessionStore.setActiveSession(sessionId)
  historyDrawerVisible.value = false
  sessionsDropdownVisible.value = false
}

async function closeSession(sessionId: string, event: Event) {
  event.stopPropagation()

  if (llmLoading.value) {
    return // Don't delete sessions while LLM is generating output
  }

  // Don't allow deleting the only session
  if (sortedSessions.value.length <= 1) {
    return
  }

  const sessionIndex = sortedSessions.value.findIndex(s => s.session_id === sessionId)

  try {
    await sessionStore.deleteSession(sessionId)

    // If deleted the active session, switch to another
    if (sessionId === activeSessionId.value && sortedSessions.value.length > 0) {
      // Try to select the next tab, or the previous one if it was the last
      const newIndex = Math.min(sessionIndex, sortedSessions.value.length - 1)
      if (newIndex >= 0) {
        await selectSession(sortedSessions.value[newIndex].session_id)
      }
      else {
        sessionStore.setActiveSession(null)
        llmStore.clearMessages()
        if (llmStore.currentSessionId) {
          llmStore.currentSessionId = null
        }
        emit('sessionCleared')
      }
    }
  }
  catch (error) {
    console.error('Failed to delete session:', error)
  }
}

async function duplicateSession(sessionId: string, event: Event) {
  event.stopPropagation()

  if (llmLoading.value) {
    return // Don't duplicate sessions while LLM is generating output
  }

  try {
    const newSession = await sessionStore.duplicateSession(sessionId)
    await selectSession(newSession.session_id)
  }
  catch (error) {
    console.error('Failed to duplicate session:', error)
  }
}

function startEditingTitle(sessionId: string, currentTitle: string, event: Event) {
  event.stopPropagation()
  editingSessionId.value = sessionId
  editingTitle.value = currentTitle

  nextTick(() => {
    const input = document.querySelector('.tab-title-input input') as HTMLInputElement
    if (input) {
      input.focus()
      input.select()
    }
  })
}

async function saveTitle() {
  if (!editingSessionId.value || !editingTitle.value.trim())
    return

  try {
    await sessionStore.updateSession(editingSessionId.value, {
      title: editingTitle.value.trim(),
    })
    editingSessionId.value = null
    editingTitle.value = ''
  }
  catch (error) {
    console.error('Failed to update session title:', error)
  }
}

function cancelEditing(event?: Event) {
  if (event) {
    event.stopPropagation()
  }
  editingSessionId.value = null
  editingTitle.value = ''
}

function handleKeyDown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    cancelEditing()
  }
}

function formatDate(dateStr: string) {
  const date = new Date(dateStr)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24))

  if (diffDays === 0) {
    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  }
  else if (diffDays === 1) {
    return 'Yesterday'
  }
  else if (diffDays < 7) {
    return `${diffDays} days ago`
  }
  else {
    return date.toLocaleDateString()
  }
}
</script>

<template>
  <div class="llm-session-tabs">
    <div class="tabs-container">
      <div class="tabs-scroll">
        <!-- Visible session tabs -->
        <div
          v-for="session in visibleSessions"
          :key="session.session_id"
          class="tab-item" :class="[
            {
              active: session.session_id === activeSessionId,
              disabled: llmLoading,
            },
          ]"
          @click="selectSession(session.session_id)"
        >
          <div class="tab-content">
            <div v-if="editingSessionId === session.session_id" class="tab-title-input">
              <AInput
                v-model:value="editingTitle"
                size="small"
                :bordered="false"
                @press-enter="saveTitle"
                @blur="saveTitle"
                @keydown="handleKeyDown"
                @click.stop
              />
            </div>
            <span v-else class="tab-title" @dblclick="startEditingTitle(session.session_id, session.title, $event)">
              {{ session.title }}
            </span>

            <div class="tab-actions">
              <ADropdown :trigger="['click']" placement="bottomRight">
                <AButton
                  type="text"
                  size="small"
                  class="tab-action-btn"
                  @click.stop
                >
                  <MoreOutlined />
                </AButton>
                <template #overlay>
                  <AMenu>
                    <AMenuItem @click="startEditingTitle(session.session_id, session.title, $event)">
                      <EditOutlined />
                      {{ $gettext('Rename') }}
                    </AMenuItem>
                    <AMenuItem @click="duplicateSession(session.session_id, $event)">
                      <CopyOutlined />
                      {{ $gettext('Duplicate') }}
                    </AMenuItem>
                    <template v-if="sortedSessions.length > 1">
                      <AMenuDivider />
                      <AMenuItem danger @click="closeSession(session.session_id, $event)">
                        <DeleteOutlined />
                        {{ $gettext('Delete') }}
                      </AMenuItem>
                    </template>
                  </AMenu>
                </template>
              </ADropdown>

              <AButton
                v-if="sortedSessions.length > 1"
                type="text"
                size="small"
                class="tab-close-btn"
                @click="closeSession(session.session_id, $event)"
              >
                <CloseOutlined />
              </AButton>
            </div>
          </div>
        </div>
      </div>

      <!-- Actions -->
      <div class="tab-actions-group">
        <!-- Sessions list button -->
        <APopover
          v-model:open="sessionsDropdownVisible"
          :trigger="['click']"
          placement="bottomRight"
          overlay-class-name="sessions-popover"
          @open-change="(open) => { if (!open) searchText = '' }"
        >
          <AButton
            type="text"
            size="small"
            class="sessions-btn"
          >
            <HistoryOutlined />
          </AButton>
          <template #content>
            <div class="sessions-dropdown">
              <div class="sessions-search">
                <AInput
                  v-model:value="searchText"
                  placeholder="Search sessions..."
                  size="small"
                  allow-clear
                  @click.stop
                />
              </div>
              <div class="sessions-list">
                <div
                  v-for="session in filteredSessions"
                  :key="session.session_id"
                  class="session-item"
                  :class="{ active: session.session_id === activeSessionId }"
                  @click="selectSession(session.session_id)"
                >
                  <div class="session-info">
                    <div class="session-title">
                      {{ session.title }}
                    </div>
                    <div class="session-meta">
                      <span class="session-date">{{ formatDate(session.updated_at) }}</span>
                      <span v-if="session.message_count > 0" class="session-count">
                        {{ session.message_count }} messages
                      </span>
                    </div>
                  </div>
                  <div class="session-actions">
                    <ADropdown :trigger="['click']" placement="bottomLeft">
                      <AButton
                        type="text"
                        size="small"
                        @click.stop
                      >
                        <MoreOutlined />
                      </AButton>
                      <template #overlay>
                        <AMenu>
                          <AMenuItem @click.stop="startEditingTitle(session.session_id, session.title, $event)">
                            <EditOutlined />
                            {{ $gettext('Rename') }}
                          </AMenuItem>
                          <AMenuItem @click.stop="duplicateSession(session.session_id, $event)">
                            <CopyOutlined />
                            {{ $gettext('Duplicate') }}
                          </AMenuItem>
                          <template v-if="sortedSessions.length > 1">
                            <AMenuDivider />
                            <AMenuItem danger @click.stop="closeSession(session.session_id, $event)">
                              <DeleteOutlined />
                              {{ $gettext('Delete') }}
                            </AMenuItem>
                          </template>
                        </AMenu>
                      </template>
                    </ADropdown>
                  </div>
                </div>
              </div>
            </div>
          </template>
        </APopover>

        <!-- Add new session button -->
        <AButton
          type="text"
          size="small"
          class="add-btn"
          :disabled="llmLoading"
          @click="createNewSession"
        >
          <PlusOutlined />
        </AButton>
      </div>
    </div>
  </div>
</template>

<style lang="less" scoped>
.llm-session-tabs {
  background: rgba(255, 255, 255, 0.8);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
  width: 100%;
  position: sticky;
  top: 0;
  z-index: 2;
  box-shadow: 0 0 10px rgba(0, 0, 0, 0.04);

  .tabs-container {
    display: flex;
    align-items: flex-end;
    height: 48px;
    padding: 6px 12px 0;
    width: 100%;
    box-sizing: border-box;
  }

  .tabs-scroll {
    flex: 1;
    display: flex;
    overflow-x: auto;
    overflow-y: hidden;
    gap: 0;
    min-width: 0;
    border: 1px solid var(--color-border);
    border-bottom: none;
    border-radius: 8px 8px 0 0;
    background: transparent;
    position: relative;

    &::-webkit-scrollbar {
      height: 0;
    }
  }

  .tab-item {
    flex-shrink: 0;
    display: flex;
    align-items: center;
    padding: 8px 8px;
    cursor: pointer;
    transition: all 0.15s ease;
    background: transparent;
    border-right: 1px solid var(--color-border);
    max-width: 120px;
    min-width: 80px;
    position: relative;
    height: 34px;
    box-sizing: border-box;

    &:first-child {
      border-top-left-radius: 7px;
    }

    &:last-child {
      border-right: none;
      border-top-right-radius: 7px;
    }

    &:hover:not(.disabled):not(.active) {
      background: var(--color-fill-2);

      .tab-title {
        color: var(--color-text-1);
      }

      .tab-actions {
        opacity: 1;
        visibility: visible;
        transform: translateY(-50%) translateX(0);
      }
    }

    &.active {
      background: var(--color-primary-light-9);
      color: var(--color-text-1);
      margin-bottom: -1px;
      z-index: 2;
      position: relative;
      border-radius: 6px;
      border: 1px solid var(--color-primary-light-7);

      .tab-title {
        font-weight: 500;
        color: var(--color-text-1);
      }

      &:hover {
        .tab-actions {
          opacity: 1;
          visibility: visible;
          transform: translateY(-50%) translateX(0);
        }
      }
    }

    &.disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }
  }

  .tab-content {
    display: flex;
    align-items: center;
    width: 100%;
  }

  .tab-title {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 13px;
    color: var(--color-text-2);
    transition: color 0.15s ease;
  }

  .tab-title-input {
    flex: 1;

    :deep(.ant-input) {
      padding: 4px 0;
      font-size: 13px;
      background: transparent;
      border: none;
      color: var(--color-text-1);

      &:focus {
        box-shadow: none;
        border: none;
        background: var(--color-fill-1);
        border-radius: 4px;
        padding: 4px 8px;
      }
    }
  }

  .tab-actions {
    position: absolute;
    right: 0;
    top: 50%;
    transform: translateY(-50%) translateX(8px);
    display: flex;
    align-items: center;
    gap: 1px;
    opacity: 0;
    visibility: hidden;
    transition: all 0.2s ease;
    background: linear-gradient(to right, rgba(255, 255, 255, 0) 0%, #ffffff 20%);
    border: 1px solid var(--color-border);
    border-radius: 4px;
    padding: 2px 3px 2px 6px;
    z-index: 10;
  }

  .tab-action-btn,
  .tab-close-btn {
    width: 22px;
    height: 22px;
    padding: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 4px;
    border: none;
    background: transparent;
    color: var(--color-text-3);
    transition: all 0.15s ease;

    &:hover {
      background: var(--color-fill-3);
      color: var(--color-text-1);
    }

    :deep(.anticon) {
      font-size: 12px;
    }
  }

  .tab-close-btn:hover {
    background: var(--color-danger-light-1);
    color: var(--color-danger);
  }

  .tab-actions-group {
    flex-shrink: 0;
    display: flex;
    align-items: center;
    height: 34px;
    padding: 0 8px;
    background: transparent;
    border: 1px solid var(--color-border);
    border-left: none;
    border-bottom: none;
    border-radius: 0 8px 0 0;
    margin-left: 8px;
    position: relative;

    .sessions-btn,
    .history-btn,
    .add-btn {
      width: 24px;
      height: 24px;
      padding: 0;
      display: flex;
      align-items: center;
      justify-content: center;
      border-radius: 4px;
      border: none;
      background: transparent;
      color: var(--color-text-3);
      transition: all 0.15s ease;
      margin: 0 2px;

      &:hover:not(:disabled) {
        background: var(--color-fill-2);
        color: var(--color-text-1);
      }

      &:disabled {
        opacity: 0.5;
        cursor: not-allowed;
      }

      :deep(.anticon) {
        font-size: 12px;
      }
    }
  }
}

.history-list {
  padding: 8px;

  .history-item {
    padding: 14px 16px;
    cursor: pointer;
    border-radius: 8px;
    margin-bottom: 6px;
    transition: all 0.15s ease;

    &:hover:not(.disabled) {
      background: var(--color-fill-1);
    }

    &.active {
      background: var(--color-primary-light-1);
      border-color: var(--color-primary-border);

      .history-title {
        color: var(--color-primary);
        font-weight: 500;
      }
    }

    &.disabled {
      opacity: 0.5;
      cursor: not-allowed;

      &:hover {
        transform: none;
        box-shadow: none;
      }
    }

    &:last-child {
      margin-bottom: 0;
    }
  }

  .history-content {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 12px;
  }

  .history-main {
    flex: 1;
    min-width: 0;
  }

  .history-title {
    font-size: 14px;
    font-weight: 450;
    margin-bottom: 6px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--color-text-1);
    transition: color 0.15s ease;
  }

  .history-meta {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 12px;
    color: var(--color-text-3);

    .history-date {
      font-weight: 400;
    }

    .history-count {
      padding: 2px 6px;
      background: var(--color-fill-2);
      border-radius: 10px;
      font-size: 11px;
      color: var(--color-text-2);
    }
  }

  .history-actions {
    flex-shrink: 0;
    opacity: 0;
    transition: opacity 0.15s ease;

    .history-item:hover & {
      opacity: 1;
    }

    .ant-btn {
      border: none;
      background: transparent;
      color: var(--color-text-3);
      border-radius: 4px;

      &:hover {
        background: var(--color-fill-2);
        color: var(--color-text-1);
      }
    }
  }
}

.sessions-dropdown {
  width: 360px;

  .sessions-search {
    padding: 8px 10px;
    border-bottom: 1px solid var(--color-border);

    :deep(.ant-input) {
      border-radius: 6px;
      font-size: 13px;
    }
  }

  .sessions-list {
    max-height: 380px;
    overflow-y: auto;
    padding: 4px 0;

    &::-webkit-scrollbar {
      width: 6px;
    }

    &::-webkit-scrollbar-thumb {
      background: var(--color-fill-3);
      border-radius: 3px;
    }
  }

  .session-item {
    padding: 6px 10px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    cursor: pointer;
    transition: all 0.15s ease;

    &:hover {
      background: var(--color-fill-1);

      .session-actions {
        opacity: 1;
      }
    }

    &.active {
      background: var(--color-primary-light-9);

      .session-title {
        color: var(--color-primary);
        font-weight: 500;
      }
    }

    .session-info {
      flex: 1;
      min-width: 0;
    }

    .session-title {
      font-size: 13px;
      font-weight: 450;
      margin-bottom: 2px;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      color: var(--color-text-1);
    }

    .session-meta {
      display: flex;
      align-items: center;
      gap: 6px;
      font-size: 11px;
      color: var(--color-text-3);

      .session-date {
        font-weight: 400;
      }

      .session-count {
        padding: 0 4px;
        background: var(--color-fill-2);
        border-radius: 8px;
        font-size: 10px;
        line-height: 16px;
        color: var(--color-text-2);
      }
    }

    .session-actions {
      opacity: 0;
      transition: opacity 0.15s ease;

      .ant-btn {
        width: 20px;
        height: 20px;
        padding: 0;
        display: flex;
        align-items: center;
        justify-content: center;

        :deep(.anticon) {
          font-size: 11px;
        }
      }
    }
  }
}

:deep(.sessions-popover) {
  .ant-popover-inner {
    padding: 0;
  }
}

.dark {
  .llm-session-tabs {
    background: rgba(30, 30, 30, 0.8);

    .tabs-scroll {
      background: transparent;
      border-color: rgba(255, 255, 255, 0.1);
    }

    .tab-item {
      border-right-color: rgba(255, 255, 255, 0.08);

      &:hover:not(.disabled):not(.active) {
        background: rgba(255, 255, 255, 0.08);
      }

      &.active {
        background: rgba(var(--primary-6), 0.15);
        color: #ffffff;
        border: 1px solid var(--color-primary);
        border-radius: 6px;
      }
    }

    .tab-title {
      color: rgba(255, 255, 255, 0.7);
    }

    .tab-item:hover:not(.disabled):not(.active) .tab-title {
      color: rgba(255, 255, 255, 0.9);
    }

    .tab-item.active .tab-title {
      color: #ffffff;
      font-weight: 500;
    }

    .tab-title-input :deep(.ant-input) {
      color: #ffffff;
      font-size: 13px;

      &:focus {
        background: rgba(255, 255, 255, 0.1);
      }
    }

    .tab-action-btn,
    .tab-close-btn {
      color: rgba(255, 255, 255, 0.6);

      &:hover {
        background: rgba(255, 255, 255, 0.1);
        color: rgba(255, 255, 255, 0.9);
      }
    }

    .tab-close-btn:hover {
      background: rgba(239, 68, 68, 0.2);
      color: #ef4444;
    }

    .tab-actions-group {
      background: transparent;
      border-color: rgba(255, 255, 255, 0.1);

      .sessions-btn,
      .history-btn,
      .add-btn {
        color: rgba(255, 255, 255, 0.6);

        &:hover:not(:disabled) {
          background: rgba(255, 255, 255, 0.1);
          color: rgba(255, 255, 255, 0.9);
        }
      }
    }
  }

  .history-list {
    .history-item {
      &:hover:not(.disabled) {
        background: #2a2a2a;
      }

      &.active {
        background: rgba(var(--primary-6), 0.15);
        border-color: var(--color-primary);

        .history-title {
          color: var(--color-primary);
        }
      }
    }

    .history-title {
      color: #e8e8e8;
    }

    .history-meta {
      color: #888888;

      .history-count {
        background: #333333;
        color: #aaaaaa;
      }
    }

    .history-actions .ant-btn {
      color: #888888;

      &:hover {
        background: #404040;
        color: #e8e8e8;
      }
    }
  }

  .tab-actions {
    background: linear-gradient(to right, rgba(26, 26, 26, 0) 0%, #1a1a1a 20%);
    border-color: rgba(255, 255, 255, 0.1);
  }

  .sessions-dropdown {
    .sessions-search {
      border-bottom-color: rgba(255, 255, 255, 0.1);
    }

    .session-item {
      &:hover {
        background: #2a2a2a;
      }

      &.active {
        background: rgba(var(--primary-6), 0.15);

        .session-title {
          color: var(--color-primary);
        }
      }
    }

    .session-title {
      color: #e8e8e8;
    }

    .session-meta {
      color: #888888;

      .session-count {
        background: #333333;
        color: #aaaaaa;
      }
    }
  }

  :deep(.sessions-popover) {
    .ant-popover-inner {
      background: #1a1a1a;
      box-shadow: 0 4px 12px rgba(0, 0, 0, 0.4);
    }
  }
}
</style>
