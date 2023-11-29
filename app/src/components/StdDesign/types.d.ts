import Curd, {Pagination} from '@/api/curd'
import { Ref } from 'vue'

export interface StdDesignEdit {
  type?: function // component type

  show?: function // show component

  batch?: boolean // batch edit

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

export interface Column {
  title?: string | (() => string);
  dataIndex: string;
  edit?: StdDesignEdit;
  customRender?: function;
  extra?: string | (() => string);
  pithy?: boolean;
  search?: boolean | StdDesignEdit;
  sortable?: boolean;
  hidden?: boolean;
  width?: string | number;
  handle?: boolean;
  hiddenInTrash?: boolean;
  hiddenInCreate?: boolean;
  hiddenInModify?: boolean;
  batch?: boolean;
}


export interface StdTableProvideData {
  displayColumns: Column[];
  pithyColumns: Column[];
  columnsMap: { [key: string]: Column };
  displayKeys: string[];
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  editItem: (id: number, data: any, index: string | number) => void;
  deleteItem: (id: number, index: string | number) => void;
  recoverItem: (id: number) => {};
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  params: any;
  dataSource: any;
  get_list: () => void;
  loading: Ref<boolean>;
}

export interface StdTableResponse {
  data: any[]
  pagination: Pagination
}
