<script setup lang="ts">
import { describePermission, permissionLabel } from './permissions'

const props = defineProps<{
  permissions: string[]
  /** The reason the plugin gives for a permission, by permission. */
  reasons?: Record<string, string>
}>()
</script>

<template>
  <ul v-if="props.permissions.length > 0" class="permission-list">
    <li v-for="permission in props.permissions" :key="permission">
      <ATag color="warning" class="permission-tag">
        {{ permissionLabel(permission) }}
      </ATag>
      <div class="permission-body">
        <div class="permission-description">
          {{ describePermission(permission) }}
        </div>
        <div v-if="props.reasons?.[permission]" class="permission-reason">
          <span class="permission-reason-label">{{ $gettext('Note from the author') }}</span>
          {{ props.reasons[permission] }}
        </div>
      </div>
    </li>
  </ul>
  <p v-else class="mb-0 text-gray-500">
    {{ $gettext('This plugin needs no access beyond its own features.') }}
  </p>
</template>

<style lang="less" scoped>
.permission-list {
  margin: 0;
  padding: 0;
  list-style: none;

  // Each row keeps its tag next to the text, the note starts under the text.
  li {
    display: grid;
    grid-template-columns: max-content minmax(0, 1fr);
    align-items: baseline;
    column-gap: 6px;
    padding: 6px 0;

    & + li {
      border-top: 1px solid var(--ant-color-split);
    }
  }
}

.permission-tag {
  justify-self: start;
  margin: 0;
  font-size: 12px;
}

.permission-description {
  color: var(--ant-color-text-secondary);
}

.permission-reason {
  margin-top: 6px;
  padding-left: 10px;
  border-left: 2px solid var(--ant-color-border);
  font-size: 13px;
}

.permission-reason-label {
  margin-right: 6px;
  color: var(--ant-color-text-tertiary);
}
</style>
