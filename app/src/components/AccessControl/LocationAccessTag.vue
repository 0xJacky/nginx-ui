<script setup lang="ts">
import { useAccessControlStore } from './store'

const props = defineProps<{
  serverIdx: number
  locationIdx: number
}>()

const store = useAccessControlStore()

const state = computed(() => store.locationState(props.serverIdx, props.locationIdx))

// Only locations that differ from the server get a tag; inheriting is the
// normal case and would only add noise to the header.
const tag = computed(() => {
  switch (state.value?.mode) {
    case 'list':
      return { color: 'green', text: store.listName(state.value.slug) }
    case 'public':
      return { color: 'orange', text: $gettext('Public') }
    case 'acme':
      return { color: 'default', text: $gettext('ACME') }
    case 'manual':
      return { color: 'default', text: $gettext('Custom rules') }
    default:
      return undefined
  }
})
</script>

<template>
  <ATag v-if="tag" :color="tag.color" class="ml-2" :bordered="false">
    {{ tag.text }}
  </ATag>
</template>
