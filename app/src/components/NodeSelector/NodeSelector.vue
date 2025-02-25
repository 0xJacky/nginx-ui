<script setup lang="ts">
import type { Environment } from '@/api/environment'
import type { SSEvent } from 'sse.js'
import { useUserStore } from '@/pinia'
import { SSE } from 'sse.js'

const props = defineProps<{
  hiddenLocal?: boolean
}>()

const target = defineModel<number[]>('target')
const map = defineModel<Record<number, string>>('map')
const { token } = storeToRefs(useUserStore())

const data = ref<Environment[]>([])
const data_map = ref<Record<number, Environment>>({})

const sse = shallowRef(newSSE())

function reconnect() {
  setTimeout(() => {
    sse.value = newSSE()
  }, 5000)
}

function newSSE() {
  const s = new SSE('api/environments/enabled', {
    headers: {
      Authorization: token.value,
    },
  })

  s.onmessage = (e: SSEvent) => {
    data.value = JSON.parse(e.data)
    nextTick(() => {
      data_map.value = data.value.reduce((acc, node) => {
        acc[node.id] = node
        return acc
      }, {} as Record<number, Environment>)
    })
  }

  // reconnect
  s.onerror = reconnect

  return s
}

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
