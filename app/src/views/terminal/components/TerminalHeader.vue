<script setup lang="ts">
import type { TerminalTab } from '@/pinia/moudule/terminal'
import { CloseOutlined, PlusOutlined } from '@ant-design/icons-vue'

interface Props {
  tabs: TerminalTab[]
  activeTabId: string | null
  llmPanelVisible: boolean
}

defineProps<Props>()

const emit = defineEmits<{
  switchTab: [tabId: string]
  closeTab: [tabId: string]
  createNewTerminal: []
  toggleRightPanel: []
}>()

function handleSwitchTab(tabId: string) {
  emit('switchTab', tabId)
}

function handleCloseTab(tabId: string) {
  emit('closeTab', tabId)
}

function handleCreateNewTerminal() {
  emit('createNewTerminal')
}

function handleToggleRightPanel() {
  emit('toggleRightPanel')
}
</script>

<template>
  <div class="terminal-header">
    <div class="terminal-tabs">
      <div class="tabs-scroll">
        <div
          v-for="tab in tabs"
          :key="tab.id"
          class="terminal-tab"
          :class="{ active: tab.id === activeTabId }"
          @click="handleSwitchTab(tab.id)"
        >
          <span class="tab-name">{{ tab.name }}</span>
          <CloseOutlined
            v-if="tabs.length > 1"
            class="tab-close"
            @click.stop="handleCloseTab(tab.id)"
          />
        </div>
      </div>

      <div class="tab-actions-group">
        <AButton
          type="text"
          size="small"
          class="add-tab-btn"
          @click="handleCreateNewTerminal"
        >
          <template #icon>
            <PlusOutlined />
          </template>
        </AButton>
      </div>
    </div>
    <div class="header-actions">
      <AButton
        type="text"
        size="small"
        @click="handleToggleRightPanel"
      >
        {{ llmPanelVisible ? $gettext('Hide Assistant') : $gettext('Show Assistant') }}
      </AButton>
    </div>
  </div>
</template>

<style lang="less" scoped>
.terminal-header {
  background: rgba(30, 30, 30, 0.8);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
  border-bottom: 1px solid #333;
  box-shadow: 0 0 10px rgba(0, 0, 0, 0.04);
  display: flex;
  justify-content: space-between;

  .terminal-tabs {
    flex: 1;
    display: flex;
    align-items: center;
    height: 47px;
    padding: 0 12px;
    width: 100%;
    box-sizing: border-box;

    .tabs-scroll {
      flex: 1;
      display: flex;
      overflow-x: auto;
      overflow-y: hidden;
      gap: 0;
      min-width: 0;
      background: transparent;
      position: relative;

      &::-webkit-scrollbar {
        height: 0;
      }
    }

    .terminal-tab {
      flex-shrink: 0;
      display: flex;
      align-items: center;
      padding: 8px 8px;
      cursor: pointer;
      transition: all 0.15s ease;
      background: transparent;
      max-width: 120px;
      min-width: 80px;
      position: relative;
      box-sizing: border-box;
      border-radius: 6px;

      &:hover:not(.active) {
        .tab-name {
          color: rgba(255, 255, 255, 0.9);
        }

        .tab-close {
          opacity: 1;
          transform: scale(1);
        }
      }

      &.active {
        color: #ffffff;
        z-index: 2;
        position: relative;

        .tab-name {
          font-weight: 500;
          color: #ffffff;
        }

        .tab-close {
          opacity: 1;
          transform: scale(1);
        }
      }

      .tab-name {
        flex: 1;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        font-size: 13px;
        color: rgba(255, 255, 255, 0.7);
        transition: color 0.15s ease;
      }

      .tab-close {
        width: 22px;
        height: 22px;
        padding: 0;
        display: flex;
        align-items: center;
        justify-content: center;
        border-radius: 4px;
        border: none;
        background: transparent;
        color: rgba(255, 255, 255, 0.6);
        margin-left: 4px;
        opacity: 0;
        transform: scale(0.8);
        transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);

        &:hover {
          background: rgba(239, 68, 68, 0.2);
          color: #ef4444;
          transform: scale(1);
        }

        :deep(.anticon) {
          font-size: 12px;
        }
      }
    }
  }

  .tab-actions-group {
    flex-shrink: 0;
    display: flex;
    align-items: center;

    .add-tab-btn {
      width: 24px;
      height: 24px;
      padding: 0;
      display: flex;
      align-items: center;
      justify-content: center;
      border: none;
      background: transparent;
      color: rgba(255, 255, 255, 0.6);
      transition: color 0.15s ease;

      &:hover:not(:disabled) {
        color: rgba(255, 255, 255, 0.9);
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

  .header-actions {
    display: flex;
    gap: 8px;
    align-items: center;
    padding: 0 12px 0 0;

    .icon {
      font-size: 16px;
    }

    :deep(.ant-btn) {
      color: #e0e0e0;
      border: 1px solid #444;
      background: transparent;

      &:hover {
        color: #4a9eff;
        border-color: #4a9eff;
        background: rgba(74, 158, 255, 0.1);
      }
    }
  }
}

@media (max-width: 768px) {
  .terminal-header {
    padding: 6px 8px;
    min-height: 44px;

    :deep(.ant-btn) {
      font-size: 12px;
      padding: 4px 8px;
    }
  }
}
</style>
