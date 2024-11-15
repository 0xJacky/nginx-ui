/* eslint-disable ts/no-explicit-any */

import type { Pagination } from '@/api/curd'
import type Curd from '@/api/curd'

import type { TableColumnType } from 'ant-design-vue'
import type { RuleObject } from 'ant-design-vue/es/form'
import type { JSX } from 'vue/jsx'

export type JSXElements = JSX.Element[]

// use for select-option
export type StdDesignMask =
  Record<string | number, string | (() => string)>
  | (() => Promise<Record<string | number, string>>)

export interface StdDesignEdit {

  type?: (edit: StdDesignEdit, dataSource: any, dataIndex: any) => JSX.Element // component type

  show?: (dataSource: any) => boolean // show component or not

  batch?: boolean // batch edit

  mask?: StdDesignMask

  rules?: RuleObject[] // validator rules

  hint?: string | (() => string) // hint form item

  actualDataIndex?: string

  datePicker?: {
    picker?: 'date' | 'week' | 'month' | 'year' | 'quarter'
    format?: string
  }

  cascader?: {
    api: () => Promise<any>
    fieldNames: Record<string, string>
  }

  select?: {
    multiple?: boolean
  }

  selector?: {
    getParams?: Record<string | number, any>
    selectionType?: 'radio' | 'checkbox'
    api: Curd
    valueApi?: Curd
    columns: any
    disableSearch?: boolean
    description?: string
    bind?: any
    itemKey?: any // default is id
    dataSourceValueIndex?: any // relative to dataSource
    recordValueIndex?: any // relative to dataSource
    getCheckboxProps?: (record: any) => any
    expandAll?: boolean
  } // StdSelector Config

  upload?: {
    limit?: number // upload file limitation
    action: string // upload url
  }

  config?: {
    label?: string | (() => string) // label for form item
    recordValueIndex?: any // relative to api return
    placeholder?: string | (() => string) // placeholder for input
    generate?: boolean // generate btn for StdPassword
    selectionType?: any
    api?: Curd
    valueApi?: Curd
    columns?: any
    disableSearch?: boolean
    description?: string
    bind?: any
    itemKey?: any // default is id
    dataSourceValueIndex?: any // relative to dataSource
    defaultValue?: any
    required?: boolean
    noValidate?: boolean
    min?: number // min value for input number
    max?: number // max value for input number
    addonBefore?: string // for inputNumber
    addonAfter?: string // for inputNumber
    prefix?: string // for inputNumber
    suffix?: string // for inputNumber
    size?: string // class size of Std image upload
    error_messages?: Ref
  }

  flex?: Flex
}

export interface Flex {
  // eslint-disable-next-line sonarjs/use-type-alias
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
  radio?: boolean
  mask?: StdDesignMask
  customRender?: function
  selector?: {
    getParams?: Record<string | number, any>
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
    getCheckboxProps?: (record: any) => any
  }
}

export interface StdTableResponse {
  data: any[]
  pagination: Pagination
}

export interface BulkActionOptions {
  text: () => string
  action: (rows: (number | string)[] | undefined) => Promise<boolean>
}

export type BulkActions = Record<string, BulkActionOptions | boolean> & {
  delete?: boolean | BulkActionOptions
  recover?: boolean | BulkActionOptions
}
