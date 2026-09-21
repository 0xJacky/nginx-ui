<script setup lang="ts">
import { describePermission } from './permissions'

const props = defineProps<{
  permissions: string[]
}>()
</script>

<template>
  <ul v-if="props.permissions.length > 0" class="permission-list">
    <li v-for="permission in props.permissions" :key="permission">
      <ATag color="warning" class="permission-tag">
        {{ permission }}
      </ATag>
      <span class="permission-description">{{ describePermission(permission) }}</span>
    </li>
  </ul>
  <p v-else class="mb-0 text-gray-500">
    {{ $gettext('This plugin does not request any host permission.') }}
  </p>
</template>

<style lang="less" scoped>
.permission-list {
  margin: 0;
  padding: 0;
  list-style: none;

  li {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 4px;
    padding: 6px 0;

    & + li {
      border-top: 1px solid var(--ant-color-split);
    }
  }
}

.permission-tag {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
}

.permission-description {
  color: var(--ant-color-text-secondary);
}
</style>
