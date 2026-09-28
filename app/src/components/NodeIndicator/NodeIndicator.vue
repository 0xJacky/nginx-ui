<script setup lang="ts">
import { DatabaseOutlined, DownOutlined } from '@antdv-next/icons'
import { storeToRefs } from 'pinia'
import NodeSwitcher from '@/components/NodeSwitcher'
import { useSettingsStore } from '@/pinia'

const settingsStore = useSettingsStore()
const { node, server_name } = storeToRefs(settingsStore)

const open = ref(false)

const isLocal = computed(() => node.value.id === 0)
const currentName = computed(() => isLocal.value ? (server_name.value || $gettext('Local')) : node.value.name)
</script>

<template>
  <div class="indicator">
    <NodeSwitcher v-model:open="open">
      <button
        type="button"
        class="container"
        :class="{ remote: !isLocal }"
        :aria-expanded="open"
      >
        <DatabaseOutlined />
        <span class="node-name">{{ currentName }}</span>
        <DownOutlined class="chevron" :class="{ open }" />
      </button>
    </NodeSwitcher>
  </div>
</template>

<style scoped lang="less">
// Keep in sync with the alignment variables in NodeSwitcher.
@pill-inset: 15px;
@pill-gap: 8px;
@icon-size: 14px;

// Nested as deep as the `> .anticon` rule below so it still wins over it.
.ant-layout-sider-collapsed .indicator .container {
  justify-content: center;

  .node-name, > .chevron {
    display: none;
  }
}

.indicator {
  padding: 20px 20px 16px 20px;

  .container {
    width: 100%;
    border-radius: 16px;
    border: 1px solid #91d5ff;
    background: #e6f7ff;
    height: 32px;
    // Symmetric insets and equal-width icons on both ends put the name in the
    // middle of the pill, not just between the two icons.
    padding: 0 @pill-inset;
    color: #096dd9;
    font: inherit;
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: @pill-gap;
    transition: border-color 0.2s;

    &:hover {
      border-color: #1677ff;
    }

    &:focus-visible {
      outline: 2px solid #1677ff;
      outline-offset: 1px;
    }

    &.remote {
      border-color: #ffd591;
      background: #fff7e6;
      color: #d46b08;

      &:hover {
        border-color: #fa8c16;
      }
    }

    // Icons are inline-flex by default and sit on the text baseline; as flex
    // items they center on the box instead.
    > .anticon {
      display: flex;
    }

    .node-name {
      flex: 1;
      min-width: 0;
      text-align: center;
      text-overflow: ellipsis;
      white-space: nowrap;
      line-height: 20px;
      overflow: hidden;
    }

    .chevron {
      width: @icon-size;
      justify-content: center;
      font-size: 10px;
      opacity: 0.7;
      transition: transform 0.2s;

      &.open {
        transform: rotate(180deg);
      }
    }
  }
}

.dark {
  .indicator .container {
    border: 1px solid #545454;
    background: transparent;
    color: #bebebe;

    &:hover {
      border-color: #8c8c8c;
    }

    &.remote {
      border-color: rgba(216, 150, 20, 0.5);
      color: #d89614;

      &:hover {
        border-color: #d89614;
      }
    }
  }
}
</style>
