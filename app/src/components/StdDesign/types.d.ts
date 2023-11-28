import Curd from '@/api/curd'
import {IKeyEvt} from '@/components/StdDesign/StdDataDisplay/types'
import {Ref} from 'vue'

export interface StdDesignEdit {
  type?: function // component type

  mask?: {
    [key: string]: () => string
  } // use for select-option

  rules?: [] // validator rules

  selector?: {
    get_params?: {}
    recordValueIndex: any // relative to api return
    selectionType: any
    api: Curd,
    valueApi?: Curd,
    columns: any
    disable_search?: boolean
    description?: string
    bind?: any
    itemKey?: any // default is id
    dataSourceValueIndex?: any // relative to dataSource
  } // StdSelector Config

  config?: {
    label?: string | (() => string) // label for form item
    size?: string // class size of Std image upload
    placeholder?: string | (() => string) // placeholder for input
    generate?: boolean // generate btn for StdPassword
    min?: number // min value for input number
    max?: number // max value for input number
    error_messages?: Ref
    hint?: string | (() => string) // hint form item
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
