<script setup lang="ts">
import type { UsageKind } from '../usage'
import { SearchOutlined } from '@antdv-next/icons'
import { toUsages, usageColor } from '../usage'

const props = withDefaults(defineProps<{
  usedBy: string[]
  // Tags shown before the rest folds into the list behind "+N".
  limit?: number
}>(), { limit: 3 })

const router = useRouter()

const usages = computed(() => toUsages(props.usedBy))
const shown = computed(() => usages.value.slice(0, props.limit))
const hiddenCount = computed(() => usages.value.length - shown.value.length)

const isOpen = ref(false)
const filterText = ref('')

const filtered = computed(() => {
  const text = filterText.value.trim().toLowerCase()
  return text ? usages.value.filter(u => u.label.toLowerCase().includes(text)) : usages.value
})

watch(isOpen, open => {
  if (!open)
    filterText.value = ''
})

function kindLabel(kind: UsageKind) {
  if (kind === 'site')
    return $gettext('Site')
  if (kind === 'stream')
    return $gettext('Stream')
  return $gettext('Configuration')
}

function go(to: string) {
  isOpen.value = false
  router.push(to)
}
</script>

<template>
  <div
    v-if="usages.length > 0"
    class="flex flex-wrap gap-1"
  >
    <ATag
      v-for="usage in shown"
      :key="usage.path"
      :color="usageColor[usage.kind]"
      :bordered="false"
      class="m-0 max-w-full cursor-pointer truncate"
      @click="go(usage.to)"
    >
      {{ usage.label }}
    </ATag>
    <APopover
      v-if="hiddenCount > 0"
      v-model:open="isOpen"
      trigger="click"
      placement="bottomLeft"
      :styles="{ container: { width: '320px', padding: '12px' } }"
    >
      <ATag
        :bordered="false"
        class="m-0 cursor-pointer"
      >
        {{ $gettext('+%{count} more', { count: String(hiddenCount) }) }}
      </ATag>
      <template #content>
        <div class="mb-2 font-medium">
          {{ $ngettext('Included by %{count} file', 'Included by %{count} files', usages.length, { count: String(usages.length) }) }}
        </div>
        <AInput
          v-if="usages.length > 8"
          v-model:value="filterText"
          size="small"
          allow-clear
          class="mb-2"
          :placeholder="$gettext('Search')"
        >
          <template #prefix>
            <SearchOutlined />
          </template>
        </AInput>
        <div class="usage-list max-h-72 overflow-y-auto">
          <button
            v-for="usage in filtered"
            :key="usage.path"
            type="button"
            class="usage-item"
            @click="go(usage.to)"
          >
            <span class="truncate">{{ usage.label }}</span>
            <span class="shrink-0 text-xs text-gray-500 dark:text-gray-400">{{ kindLabel(usage.kind) }}</span>
          </button>
          <div
            v-if="filtered.length === 0"
            class="py-2 text-center text-gray-500 dark:text-gray-400"
          >
            {{ $gettext('No file matches the search') }}
          </div>
        </div>
      </template>
    </APopover>
  </div>
  <span
    v-else
    class="text-gray-500 dark:text-gray-400"
  >
    {{ $gettext('Not used') }}
  </span>
</template>

<style scoped>
.usage-item {
  display: flex;
  width: 100%;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 6px 8px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--ant-color-text);
  text-align: left;
  cursor: pointer;
}

.usage-item:hover,
.usage-item:focus-visible {
  background: var(--ant-color-fill-tertiary);
  outline: none;
}
</style>
