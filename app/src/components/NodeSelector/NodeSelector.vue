<script setup lang="ts">
import { useGettext } from 'vue3-gettext'
import type { Ref } from 'vue'
import type { Environment } from '@/api/environment'
import environment from '@/api/environment'

const props = defineProps<{
  target: number[]
  map?: Record<number, string>
  hiddenLocal?: boolean
}>()

const emit = defineEmits(['update:target', 'update:map'])

const { $gettext } = useGettext()

const data = ref([]) as Ref<Environment[]>
const data_map = ref({}) as Ref<Record<number, Environment>>

environment.get_list().then(r => {
  data.value = r.data
  r.data?.forEach(node => {
    data_map.value[node.id] = node
  })
})

const value = computed({
  get() {
    return props.target
  },
  set(v) {
    if (typeof props.map === 'object') {
      v.forEach(id => {
        if (id !== 0)
          emit('update:map', { ...props.map, [id]: data_map.value[id].name })
      })
    }
    emit('update:target', v)
  },
})
</script>

<template>
  <ACheckboxGroup
    v-model:value="value"
    style="width: 100%"
  >
    <ARow :gutter="[16, 16]">
      <ACol
        v-if="!hiddenLocal"
        :span="8"
      >
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
        :span="8"
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
    <AEmpty v-if="hiddenLocal && data?.length === 0" />
  </ACheckboxGroup>
</template>

<style scoped lang="less">

</style>
