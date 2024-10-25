<script setup lang="ts">
import type Curd from '@/api/curd'
import type { Column } from '@/components/StdDesign/types'
import StdTable from '@/components/StdDesign/StdDataDisplay/StdTable.vue'
import { watchOnce } from '@vueuse/core'
import _ from 'lodash'

const props = defineProps<{
  placeholder?: string
  label?: string
  selectionType: 'radio' | 'checkbox'
  recordValueIndex: string // to index the value of the record
  // eslint-disable-next-line ts/no-explicit-any
  api: Curd<any>
  columns: Column[]
  disableSearch?: boolean
  // eslint-disable-next-line ts/no-explicit-any
  getParams?: any
  description?: string
  errorMessages?: string
  itemKey?: string // default: id
  // eslint-disable-next-line ts/no-explicit-any
  value?: any | any[]
  disabled?: boolean
  // eslint-disable-next-line ts/no-explicit-any
  valueApi?: Curd<any>
  // eslint-disable-next-line ts/no-explicit-any
  getCheckboxProps?: (record: any) => any
  hideInputContainer?: boolean
}>()

const selectedKey = defineModel<number | number[] | undefined | null | string | string[]>('selectedKey')

onMounted(() => {
  if (!selectedKey.value)
    watchOnce(selectedKey, _init)
  else
    _init()
})

const getParams = computed(() => {
  return props.getParams
})

const visible = ref(false)
// eslint-disable-next-line ts/no-explicit-any
const M_values = ref([]) as Ref<any[]>

const ComputedMValue = computed(() => {
  return M_values.value.filter(v => v && Object.keys(v).length > 0)
})

// eslint-disable-next-line ts/no-explicit-any
const records = defineModel<any[]>('selectedRecords', {
  default: () => [],
})

watch(() => props.value, () => {
  if (props.selectionType === 'radio')
    M_values.value = [props.value]
  else if (typeof selectedKey.value === 'object')
    M_values.value = props.value || []
})

async function _init() {
  // valueApi is used to fetch items that are using itemKey as index value
  const api = props.valueApi || props.api

  M_values.value = []

  if (props.selectionType === 'radio') {
    // M_values.value = [props.value]
    // not init value, we need to fetch them from api
    if (!props.value && selectedKey.value && selectedKey.value !== '0') {
      api.get(selectedKey.value, props.getParams).then(r => {
        M_values.value = [r]
        records.value = [r]
      })
    }
  }
  else if (typeof selectedKey.value === 'object') {
    // M_values.value = props.value || []
    // not init value, we need to fetch them from api
    if (!props.value && (selectedKey.value?.length || 0) > 0) {
      api.get_list({
        ...props.getParams,
        id: selectedKey.value,
      }).then(r => {
        M_values.value = r.data
        records.value = r.data
      })
    }
  }
}

function show() {
  if (!props.disabled)
    visible.value = true
}

const selectedKeyBuffer = ref()
// eslint-disable-next-line ts/no-explicit-any
const selectedBuffer: Ref<any[]> = ref([])

watch(selectedKey, () => {
  selectedKeyBuffer.value = _.clone(selectedKey.value)
})

watch(records, v => {
  selectedBuffer.value = [...v]
  M_values.value = [...v]
})

onMounted(() => {
  selectedKeyBuffer.value = _.clone(selectedKey.value)
  selectedBuffer.value = _.clone(records.value)
})

const computedSelectedKeys = computed({
  get() {
    if (props.selectionType === 'radio')
      return [selectedKeyBuffer.value]
    else
      return selectedKeyBuffer.value
  },
  set(v) {
    selectedKeyBuffer.value = v
  },
})

async function ok() {
  visible.value = false
  selectedKey.value = selectedKeyBuffer.value
  records.value = selectedBuffer.value
  await nextTick()
  M_values.value = _.clone(records.value)
}

// function clear() {
//   M_values.value = []
//   emit('update:selectedKey', '')
// }

defineExpose({ show })
</script>

<template>
  <div>
    <div
      v-if="!hideInputContainer"
      class="std-selector-container"
    >
      <div
        class="std-selector"
        @click="show"
      >
        <div class="chips-container">
          <div v-if="props.recordValueIndex">
            <ATag
              v-for="(chipText, index) in ComputedMValue"
              :key="index"
              class="mr-1"
              color="orange"
              :bordered="false"
              @click="show"
            >
              {{ chipText?.[recordValueIndex] }}
            </ATag>
          </div>
          <div
            v-else
            class="text-gray-400"
          >
            {{ placeholder }}
          </div>
        </div>
      </div>
    </div>
    <AModal
      :mask="false"
      :open="visible"
      :cancel-text="$gettext('Cancel')"
      :ok-text="$gettext('Ok')"
      :title="$gettext('Selector')"
      :width="800"
      destroy-on-close
      @cancel="visible = false"
      @ok="ok"
    >
      {{ description }}
      <StdTable
        v-model:selected-row-keys="computedSelectedKeys"
        v-model:selected-rows="selectedBuffer"
        :api
        :columns
        :disable-search
        :row-key="itemKey"
        :get-params
        :selection-type
        :get-checkbox-props
        pithy
        disable-query-params
      />
    </AModal>
  </div>
</template>

<style lang="less" scoped>
.std-selector-container {
  min-height: 39.9px;
  display: flex;
  align-items: self-start;

  .std-selector {
    overflow-y: auto;
    box-sizing: border-box;
    font-variant: tabular-nums;
    list-style: none;
    font-feature-settings: 'tnum';
    min-height: 32px;
    max-height: 100px;
    padding: 4px 11px;
    font-size: 14px;
    line-height: 1.5;
    background-image: none;
    border: 1px solid #d9d9d9;
    border-radius: 6px;
    transition: all 0.3s;
    //margin: 0 10px 0 0;
    cursor: pointer;
    min-width: 180px;
  }
}

.dark {
  .std-selector {
    border: 1px solid #424242;
    background-color: #141414;
  }
}
</style>
