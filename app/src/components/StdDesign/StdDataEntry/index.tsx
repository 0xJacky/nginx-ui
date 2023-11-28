import { h } from 'vue'
import { Input, InputNumber, Switch, Textarea } from 'ant-design-vue'
import _ from 'lodash'
import StdDataEntry from './StdDataEntry'
import StdSelector from './components/StdSelector.vue'
import StdSelect from './components/StdSelect.vue'
import StdPassword from './components/StdPassword.vue'
import type { StdDesignEdit } from '@/components/StdDesign/types'

const fn = _.get
// eslint-disable-next-line @typescript-eslint/no-explicit-any
function readonly(edit: StdDesignEdit, dataSource: any, dataIndex: any) {
  return h('p', fn(dataSource, dataIndex))
}

function placeholder_helper(edit: StdDesignEdit) {
  return typeof edit.config?.placeholder === 'function' ? edit.config?.placeholder() : edit.config?.placeholder
}
// eslint-disable-next-line @typescript-eslint/no-explicit-any
function input(edit: StdDesignEdit, dataSource: any, dataIndex: any) {
  return h(Input, {
    'placeholder': placeholder_helper(edit),
    'value': dataSource?.[dataIndex],
    'onUpdate:value': value => {
      dataSource[dataIndex] = value
    },
  })
}
// eslint-disable-next-line @typescript-eslint/no-explicit-any
function inputNumber(edit: StdDesignEdit, dataSource: any, dataIndex: any) {
  return h(InputNumber, {
    'placeholder': placeholder_helper(edit),
    'min': edit.config?.min,
    'max': edit.config?.max,
    'value': dataSource?.[dataIndex],
    'onUpdate:value': value => {
      dataSource[dataIndex] = value
    },
  })
}
// eslint-disable-next-line @typescript-eslint/no-explicit-any
function textarea(edit: StdDesignEdit, dataSource: any, dataIndex: any) {
  return h(Textarea, {
    'placeholder': placeholder_helper(edit),
    'value': dataSource?.[dataIndex],
    'onUpdate:value': value => {
      dataSource[dataIndex] = value
    },
  })
}
// eslint-disable-next-line @typescript-eslint/no-explicit-any
function password(edit: StdDesignEdit, dataSource: any, dataIndex: any) {
  return <StdPassword
    v-model:value={dataSource[dataIndex]}
    generate={edit.config?.generate}
    placeholder={placeholder_helper(edit)}
  />
}
// eslint-disable-next-line @typescript-eslint/no-explicit-any
function select(edit: StdDesignEdit, dataSource: any, dataIndex: any) {
  return <StdSelect
    v-model:value={dataSource[dataIndex]}
    mask={edit.mask}
  />
}
// eslint-disable-next-line @typescript-eslint/no-explicit-any
function selector(edit: StdDesignEdit, dataSource: any, dataIndex: any) {
  return <StdSelector
    v-model:selectedKey={dataSource[dataIndex]}
    recordValueIndex={edit.selector?.recordValueIndex}
    selectionType={edit.selector?.selectionType}
    api={edit.selector?.api}
    columns={edit.selector?.columns}
    disableSearch={edit.selector?.disable_search}
    getParams={edit.selector?.get_params}
    description={edit.selector?.description}
  />
}
// eslint-disable-next-line @typescript-eslint/no-explicit-any
function switcher(edit: StdDesignEdit, dataSource: any, dataIndex: any) {
  return h(Switch, {
    'checked': dataSource?.[dataIndex],
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    'onUpdate:checked': (value: any) => {
      dataSource[dataIndex] = value
    },
  })
}

export {
  readonly,
  input,
  textarea,
  select,
  selector,
  password,
  inputNumber,
  switcher,
}

export default StdDataEntry
