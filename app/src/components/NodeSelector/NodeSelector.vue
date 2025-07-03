<script setup lang="ts">
import type { Environment } from '@/api/environment'
import ws from '@/lib/websocket'

const props = defineProps<{
  hiddenLocal?: boolean
}>()

const target = defineModel<number[]>('target')
const map = defineModel<Record<number, string>>('map')

const data = ref<Environment[]>([])
const data_map = ref<Record<number, Environment>>({})

// WebSocket connection for environment monitoring
const socket = ws('/api/environments/enabled', true)

socket.onmessage = event => {
  try {
    const message = JSON.parse(event.data)

    if (message.event === 'message') {
      const environments: Environment[] = message.data
      data.value = environments
      nextTick(() => {
        data_map.value = data.value.reduce((acc, node) => {
          acc[node.id] = node
          return acc
        }, {} as Record<number, Environment>)
      })
    }
  }
  catch (error) {
    console.error('Error parsing WebSocket message:', error)
  }
}

socket.onerror = error => {
  console.warn('Failed to connect to environments WebSocket endpoint', error)
}

// Cleanup on unmount
onUnmounted(() => {
  socket.close()
})

const value = computed({
  get() {
    return target.value
  },
  set(v: number[]) {
    if (typeof map.value === 'object') {
      const _map = {}

      v?.filter(id => id !== 0).forEach(id => {
        _map[id] = data_map.value[id].name
      })

      map.value = _map
    }
    target.value = v.filter(id => id !== 0)
  },
})

const noData = computed(() => {
  return props.hiddenLocal && !data?.value?.length
})
</script>

<template>
  <ACheckboxGroup
    v-model:value="value"
    class="w-full"
    :class="{
      'justify-center': noData,
    }"
  >
    <ARow
      v-if="!noData"
      :gutter="[16, 16]"
    >
      <ACol v-if="!hiddenLocal">
        <ACheckbox :value="0">
          {{ $gettext('Local') }}
        </ACheckbox>
        <ATag color="green">
          {{ $gettext('Online') }}
        </ATag>
      </ACol>
      <ACol
        v-for="(node, index) in data"
        :key="index"
      >
        <ACheckbox :value="node.id">
          {{ node.name }}
        </ACheckbox>
        <ATag
          v-if="node.status"
          color="green"
        >
          {{ $gettext('Online') }}
        </ATag>
        <ATag
          v-else
          color="error"
        >
          {{ $gettext('Offline') }}
        </ATag>
      </ACol>
    </ARow>
    <AEmpty v-else />
  </ACheckboxGroup>
</template>

<style scoped lang="less">

</style>
