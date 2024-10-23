import Curd, {Pagination} from '@/api/curd'
import {Ref} from 'vue'
import type {JSX} from 'vue/jsx'
import {TableColumnType} from "ant-design-vue"

export type JSXElements = JSX.Element[]

export interface StdDesignEdit {
  type?: (edit: StdDesignEdit, dataSource: any, dataIndex: any) => JSX.Element // component type

  show?: (dataSource: any) => boolean // show component or not

  batch?: boolean // batch edit

  mask?: Record<string | number, string | (() => string)> | (() => Promise<Record<string | number, string>>) // use for select-option

  rules?: [] // validator rules

  hint?: string | (() => string) // hint form item

  actualDataIndex?: string

  select?: {
    multiple?: boolean
  }

  selector?: {
    getParams?: {}
    recordValueIndex: any // relative to api return
    selectionType: any
    api: Curd
    valueApi?: Curd
    columns: any
    disableSearch?: boolean
    description?: string
    bind?: any
    itemKey?: any // default is id
    dataSourceValueIndex?: any // relative to dataSource
  } // StdSelector Config

  upload?: {
    limit?: number // upload file limitation
    action: string // upload url
  }

  config?: {
    label?: string | (() => string) // label for form item
    size?: string // class size of Std image upload
    placeholder?: string | (() => string) // placeholder for input
    generate?: boolean // generate btn for StdPassword
    min?: number // min value for input number
    max?: number // max value for input number
    error_messages?: Ref
    required?: boolean
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    defaultValue?: any
    addonBefore?: string // for inputNumber
    addonAfter?: string // for inputNumber
    prefix?: string // for inputNumber
    suffix?: string // for inputNumber
  }

  flex?: Flex
}

export interface Flex {
  sm?: string | number | boolean
  md?: string | number | boolean
  lg?: string | number | boolean
  xl?: string | number | boolean
  xxl?: string | number | boolean
}

export interface Column extends TableColumnType {
  title?: string | (() => string)
  edit?: StdDesignEdit
  extra?: string | (() => string)
  pithy?: boolean
  search?: boolean | StdDesignEdit
  handle?: boolean
  hiddenInTable?: boolean
  hiddenInTrash?: boolean
  hiddenInCreate?: boolean
  hiddenInModify?: boolean
  hiddenInDetail?: boolean
  hiddenInExport?: boolean
  import?: boolean
  batch?: boolean
  customRender?: function
  selector?: {
    getParams?: {}
    recordValueIndex: any // relative to api return
    selectionType: any
    api: Curd
    valueApi?: Curd
    columns: any
    disableSearch?: boolean
    description?: string
    bind?: any
    itemKey?: any // default is id
    dataSourceValueIndex?: any // relative to dataSource
  }
}

export interface StdTableResponse {
  data: any[]
  pagination: Pagination
}
