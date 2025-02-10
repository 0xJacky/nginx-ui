import type { StdDesignEdit } from '@/components/StdDesign/types'
import type { Dayjs } from 'dayjs'
import { DATE_FORMAT } from '@/constants'
import {
  DatePicker,
  Input,
  InputNumber,
  RangePicker,
  Switch,
} from 'ant-design-vue'
import dayjs from 'dayjs'
import { h } from 'vue'
import StdPassword from './components/StdPassword.vue'
import StdSelect from './components/StdSelect.vue'
import StdSelector from './components/StdSelector.vue'
import StdDataEntry from './StdDataEntry.vue'

// eslint-disable-next-line ts/no-explicit-any
export function readonly(edit: StdDesignEdit, dataSource: any, dataIndex: any) {
  return h('p', dataSource?.[dataIndex] ?? edit?.config?.defaultValue)
}

export function labelRender(title?: string | (() => string)) {
  if (typeof title === 'function')
    return title()

  return title
}

export function placeholderHelper(edit: StdDesignEdit) {
  return typeof edit.config?.placeholder === 'function' ? edit.config?.placeholder() : edit.config?.placeholder
}

// eslint-disable-next-line ts/no-explicit-any
export function input(edit: StdDesignEdit, dataSource: any, dataIndex: any) {
  return h(Input, {
    'autocomplete': 'off',
    'placeholder': placeholderHelper(edit),
    'value': dataSource?.[dataIndex] ?? edit?.config?.defaultValue,
    'onUpdate:value': value => {
      dataSource[dataIndex] = value
    },
  })
}

// eslint-disable-next-line ts/no-explicit-any
export function inputNumber(edit: StdDesignEdit, dataSource: any, dataIndex: any) {
  if (edit.config?.defaultValue !== undefined)
    dataSource[dataIndex] = edit.config.defaultValue

  return h(InputNumber, {
    'placeholder': placeholderHelper(edit),
    'min': edit.config?.min,
    'max': edit.config?.max,
    'value': dataSource?.[dataIndex] ?? edit?.config?.defaultValue,
    'onUpdate:value': value => {
      dataSource[dataIndex] = value
    },
    'addon-before': edit.config?.addonBefore,
    'addon-after': edit.config?.addonAfter,
    'prefix': edit.config?.prefix,
    'suffix': edit.config?.suffix,
  })
}

// eslint-disable-next-line ts/no-explicit-any
export function textarea(edit: StdDesignEdit, dataSource: any, dataIndex: any) {
  if (!dataSource[dataIndex])
    dataSource[dataIndex] = edit.config?.defaultValue

  return (
    <Input
      v-model:value={dataSource[dataIndex]}
      placeholder={placeholderHelper(edit)}
    />
  )
}

// eslint-disable-next-line ts/no-explicit-any
export function password(edit: StdDesignEdit, dataSource: any, dataIndex: any) {
  return (
    <StdPassword
      v-model:value={dataSource[dataIndex]}
      value={dataSource[dataIndex] ?? edit?.config?.defaultValue}
      generate={edit.config?.generate}
      placeholder={placeholderHelper(edit)}
    />
  )
}

// eslint-disable-next-line ts/no-explicit-any
export function select(edit: StdDesignEdit, dataSource: any, dataIndex: any) {
  const actualDataIndex = edit?.actualDataIndex ?? dataIndex

  return (
    <StdSelect
      v-model:value={dataSource[actualDataIndex]}
      mask={edit.mask}
      placeholder={placeholderHelper(edit)}
      multiple={edit.select?.multiple}
      defaultValue={edit.config?.defaultValue}
    />
  )
}

// eslint-disable-next-line ts/no-explicit-any
export function selector(edit: StdDesignEdit, dataSource: any, dataIndex: any) {
  return (
    <StdSelector
      v-model:selectedKey={dataSource[dataIndex]}
      selectedKey={dataSource[dataIndex] || edit?.config?.defaultValue}
      recordValueIndex={edit.selector?.recordValueIndex}
      selectionType={edit.selector?.selectionType ?? 'radio'}
      api={edit.selector?.api}
      columns={edit.selector?.columns}
      disableSearch={edit.selector?.disableSearch}
      getParams={edit.selector?.getParams}
      description={edit.selector?.description}
      getCheckboxProps={edit.selector?.getCheckboxProps}
      expandAll={edit.selector?.expandAll}
    />
  )
}

// eslint-disable-next-line ts/no-explicit-any
export function switcher(edit: StdDesignEdit, dataSource: any, dataIndex: any) {
  return h(Switch, {
    'checked': dataSource?.[dataIndex] ?? edit?.config?.defaultValue,
    // eslint-disable-next-line ts/no-explicit-any
    'onUpdate:checked': (value: any) => {
      dataSource[dataIndex] = value
    },
  })
}

// eslint-disable-next-line ts/no-explicit-any
export function datePicker(edit: StdDesignEdit, dataSource: any, dataIndex: any) {
  const date: Dayjs | undefined = dataSource?.[dataIndex] ? dayjs.unix(dataSource?.[dataIndex]) : undefined

  return (
    <DatePicker
      allowClear
      format={edit?.datePicker?.format ?? DATE_FORMAT}
      picker={edit?.datePicker?.picker}
      value={date}
      onChange={(_, dataString) => dataSource[dataIndex] = dayjs(dataString).unix() ?? undefined}
    />
  )
}

// eslint-disable-next-line ts/no-explicit-any
export function dateRangePicker(edit: StdDesignEdit, dataSource: any, dataIndex: any) {
  const dates: [Dayjs, Dayjs] = dataSource
    ?.[dataIndex]
    ?.filter((item: string) => !!item)
    ?.map((item: string) => dayjs(item))

  return (
    <RangePicker
      allowClear
      format={edit?.datePicker?.format ?? DATE_FORMAT}
      picker={edit?.datePicker?.picker}
      value={dates}
      onChange={(_, dateStrings: [string, string]) => dataSource[dataIndex] = dateStrings}
    />
  )
}

export default StdDataEntry
