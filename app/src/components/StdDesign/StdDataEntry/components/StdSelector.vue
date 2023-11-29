<script setup lang="ts">
import StdTable from '@/components/StdDesign/StdDataDisplay/StdTable.vue'
import gettext from '@/gettext'
import type Curd from '@/api/curd'
import type { Column } from '@/components/StdDesign/types'

const props = defineProps<{
  selectedKey: string | number
  value?: string | number
  recordValueIndex: string
  selectionType: 'radio' | 'checkbox'
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  api: Curd<any>
  columns: Column[]
  disableSearch?: boolean
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  getParams: any
  description?: string
}>()

const emit = defineEmits(['update:selectedKey', 'changeSelect'])
const { $gettext } = gettext
const visible = ref(false)
const M_value = ref('')

onMounted(() => {
  init()
})

const selected = ref([])
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const record: any = reactive({})

function init() {
  if (props.selectedKey && !props.value && props.selectionType === 'radio') {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    props.api.get(props.selectedKey).then((r: any) => {
      Object.assign(record, r)
      M_value.value = r[props.recordValueIndex]
    })
  }
}

function show() {
  visible.value = true
}
// eslint-disable-next-line @typescript-eslint/no-explicit-any
function onSelect(_selected: any) {
  selected.value = _selected
}
// eslint-disable-next-line @typescript-eslint/no-explicit-any
function onSelectedRecord(r: any) {
  Object.assign(record, r)
}

function ok() {
  visible.value = false
  if (props.selectionType === 'radio')
    emit('update:selectedKey', selected.value[0])
  else
    emit('update:selectedKey', selected.value)

  M_value.value = record[props.recordValueIndex]
  emit('changeSelect', record)
}

watch(props, () => {
  if (!props?.selectedKey)
    M_value.value = ''
  else if (props.value)
    M_value.value = props.value as string
  else
    init()
})

const _selectedKey = computed({
  get() {
    return props.selectedKey
  },
  set(v) {
    emit('update:selectedKey', v)
  },
})
</script>

<template>
  <div class="std-selector-container">
    <div
      class="std-selector"
      @click="show"
    >
      <AInput
        v-model="_selectedKey"
        disabled
        hidden
      />
      <div class="value">
        {{ M_value }}
      </div>
      <AModal
        :mask="false"
        :open="visible"
        :cancel-text="$gettext('Cancel')"
        :ok-text="$gettext('OK')"
        :title="$gettext('Selector')"
        :width="800"
        destroy-on-close
        @cancel="visible = false"
        @ok="ok"
      >
        {{ description }}
        <StdTable
          :api="api"
          :columns="columns"
          :disable-search="disableSearch"
          pithy
          :get-params="getParams"
          :selection-type="selectionType"
          disable-query-params
          @on-selected="onSelect"
          @on-selected-record="onSelectedRecord"
        />
      </AModal>
    </div>
  </div>
</template>

<style lang="less" scoped>
.std-selector-container {
  height: 39.9px;
  display: flex;
  align-items: flex-start;

  .std-selector {
    box-sizing: border-box;
    font-variant: tabular-nums;
    list-style: none;
    font-feature-settings: 'tnum';
    height: 32px;
    padding: 4px 11px;
    color: rgba(0, 0, 0, 0.85);
    font-size: 14px;
    line-height: 1.5;
    background-color: #fff;
    background-image: none;
    border: 1px solid #d9d9d9;
    border-radius: 4px;
    transition: all 0.3s;
    margin: 0 10px 0 0;
    cursor: pointer;
    min-width: 180px;
  }
}
</style>
