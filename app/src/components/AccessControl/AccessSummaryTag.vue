<script setup lang="ts">
import type { AccessSummaryMode } from '@/api/access_list'

const props = defineProps<{
  mode?: AccessSummaryMode | ''
  slug?: string
}>()

const tag = computed(() => {
  switch (props.mode) {
    case 'list':
      return { color: 'green', text: props.slug ?? '' }
    case 'mixed':
      return { color: 'orange', text: $gettext('Mixed') }
    case 'manual':
      return { color: 'default', text: $gettext('Custom rules') }
    case 'public':
      return { color: 'default', text: $gettext('Public') }
    default:
      return undefined
  }
})
</script>

<template>
  <ATag v-if="tag" :color="tag.color" :bordered="false" class="font-mono">
    {{ tag.text }}
  </ATag>
</template>
