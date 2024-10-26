import type Curd from '@/api/curd'
import type { TableColumnType } from 'ant-design-vue'
import type { Ref } from 'vue'
import type { JSX } from 'vue/jsx'

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
    getParams?: object
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

type FlexType = string | number | boolean

export interface Flex {
  sm?: FlexType
  md?: FlexType
  lg?: FlexType
  xl?: FlexType
  xxl?: FlexType
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
    getParams?: object
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
