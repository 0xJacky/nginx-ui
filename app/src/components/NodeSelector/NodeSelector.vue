<script setup lang="ts">
import type { Ref } from 'vue'
import type { Environment } from '@/api/environment'
import environment from '@/api/environment'

const props = defineProps<{
  hiddenLocal?: boolean
}>()

const target = defineModel<number[]>('target')
const map = defineModel<Record<number, string>>('map')

const data = ref([]) as Ref<Environment[]>
const data_map = ref({}) as Ref<Record<number, Environment>>

onMounted(async () => {
  let hasMore = true
  let page = 1
  while (hasMore) {
    await environment.get_list({ page, enabled: true }).then(r => {
      data.value.push(...r.data)
      r.data?.forEach(node => {
        data_map.value[node.id] = node
      })
      hasMore = r.data.length === r.pagination.per_page
      page++
    }).catch(() => {
      hasMore = false
    })
  }
})

const value = computed({
  get() {
    return target.value
  },
  set(v: number[]) {
    console.log(v)
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
        <ATag color="blue">
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
          color="blue"
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
