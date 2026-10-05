<script setup lang="ts">
import type { ConfigUsageItem, ConfigUsageKind } from './types'
import { SearchOutlined } from '@antdv-next/icons'
import { configStatusLabel } from '@/components/ConfigStatusTag'
import { ConfigStatus } from '@/constants'
import { usageColor } from './types'

const props = withDefaults(defineProps<{
  usages: ConfigUsageItem[]
  // Heading of the full list, such as "Included by 5 files".
  summary: string
  // Tags shown before the rest folds into the list behind "+N".
  limit?: number
}>(), { limit: 3 })

const router = useRouter()

const shown = computed(() => props.usages.slice(0, props.limit))
const hiddenCount = computed(() => props.usages.length - shown.value.length)

const isOpen = ref(false)
const filterText = ref('')

const filtered = computed(() => {
  const text = filterText.value.trim().toLowerCase()
  return text ? props.usages.filter(u => u.label.toLowerCase().includes(text)) : props.usages
})

watch(isOpen, open => {
  if (!open)
    filterText.value = ''
})

function kindLabel(kind: ConfigUsageKind) {
  if (kind === 'site')
    return $gettext('Site')
  if (kind === 'stream')
    return $gettext('Stream')
  return $gettext('Configuration')
}

// Only a status that keeps the file from serving traffic needs a mention.
function isInactive(usage: ConfigUsageItem) {
  return !!usage.status && usage.status !== ConfigStatus.Enabled
}

function detailLabel(usage: ConfigUsageItem) {
  const kind = kindLabel(usage.kind)
  return isInactive(usage) ? `${kind} · ${configStatusLabel(usage.status!)}` : kind
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
    <ATooltip
      v-for="usage in shown"
      :key="usage.key"
      :title="isInactive(usage) ? configStatusLabel(usage.status!) : undefined"
    >
      <ATag
        :color="isInactive(usage) ? 'default' : usageColor[usage.kind]"
        :bordered="false"
        class="m-0 max-w-full cursor-pointer truncate"
        :class="{ 'opacity-60': isInactive(usage) }"
        @click="go(usage.to)"
      >
        {{ usage.label }}
      </ATag>
    </ATooltip>
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
          {{ summary }}
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
            :key="usage.key"
            type="button"
            class="usage-item"
            @click="go(usage.to)"
          >
            <span
              class="truncate"
              :class="{ 'opacity-60': isInactive(usage) }"
            >{{ usage.label }}</span>
            <span class="shrink-0 text-xs text-gray-500 dark:text-gray-400">{{ detailLabel(usage) }}</span>
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
