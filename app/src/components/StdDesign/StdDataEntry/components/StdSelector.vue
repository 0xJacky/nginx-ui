<script setup lang="ts">
import _ from 'lodash'
import type { Ref } from 'vue'
import StdTable from '@/components/StdDesign/StdDataDisplay/StdTable.vue'
import type Curd from '@/api/curd'
import type { Column } from '@/components/StdDesign/types'

const props = defineProps<{
  label?: string
  selectedKey: number | number[] | undefined | null
  selectionType: 'radio' | 'checkbox'
  recordValueIndex: string // to index the value of the record
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  api: Curd<any>
  columns: Column[]
  disableSearch?: boolean
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  getParams?: any
  description?: string
  errorMessages?: string
  itemKey?: string // default: id
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  value?: any | any[]
  disabled?: boolean
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  valueApi?: Curd<any>
}>()

const emit = defineEmits(['update:selectedKey'])

const getParams = computed(() => {
  return props.getParams
})

const visible = ref(false)
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const M_values = ref([]) as any

const init = _.debounce(_init, 500, {
  leading: true,
  trailing: false,
})

onMounted(() => {
  init()
})

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const records = ref([]) as Ref<any[]>

async function _init() {
  // valueApi is used to fetch items that are using itemKey as index value
  const api = props.valueApi || props.api

  M_values.value = []

  if (props.selectionType === 'radio') {
    // M_values.value = [props.value] // not init value, we need to fetch them from api
    if (!props.value && props.selectedKey) {
      api.get(props.selectedKey, props.getParams).then(r => {
        M_values.value = [r]
        records.value = [r]
      })
    }
  }
  else if (typeof props.selectedKey === 'object') {
    M_values.value = props.value || []

    // not init value, we need to fetch them from api
    if (!props.value && (props.selectedKey?.length || 0) > 0) {
      api.get_list({
        ...props.getParams,
        id: props.selectedKey,
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

if (props.selectionType === 'radio')
  selectedKeyBuffer.value = [props.selectedKey]
else
  selectedKeyBuffer.value = props.selectedKey

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

onMounted(() => {
  if (props.selectedKey === undefined || props.selectedKey === null) {
    if (props.selectionType === 'radio')
      emit('update:selectedKey', '')
    else
      emit('update:selectedKey', [])
  }
})

async function ok() {
  visible.value = false
  emit('update:selectedKey', selectedKeyBuffer.value)

  M_values.value = _.clone(records.value)
}

watchEffect(() => {
  init()
})

// function clear() {
//   M_values.value = []
//   emit('update:selectedKey', '')
// }
</script>

<template>
  <div class="std-selector-container">
    <div
      class="std-selector"
      @click="show"
    >
      <div class="chips-container">
        <ATag
          v-for="(chipText, index) in M_values"
          :key="index"
          class="mr-1"
          color="orange"
          :bordered="false"
          @click="show"
        >
          {{ chipText?.[recordValueIndex] }}
        </ATag>
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
          v-model:selected-rows="records"
          :api="api"
          :columns="columns"
          :disable-search="disableSearch"
          pithy
          :row-key="itemKey"
          :get-params="getParams"
          :selection-type="selectionType"
          disable-query-params
        />
      </AModal>
    </div>
  </div>
</template>

<style lang="less" scoped>
.std-selector-container {
  min-height: 39.9px;
  display: flex;
  align-items: flex-start;

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
    border-radius: 4px;
    transition: all 0.3s;
    //margin: 0 10px 0 0;
    cursor: pointer;
    min-width: 180px;
  }
}

.chips-container {
  span {
    margin: 2px;
  }
}
</style>
