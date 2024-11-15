import type { ImportConfig } from '@/components/StdDesign/StdDataImport/types'

export interface StdCurdProps<T> extends StdTableProps<T> {
  cardTitleKey?: string
  modalMaxWidth?: string | number
  modalMask?: boolean
  exportExcel?: boolean
  importExcel?: boolean

  disableAdd?: boolean
  onClickAdd?: () => void

  onClickEdit?: (id: number | string, record: T, index: number) => void
  // eslint-disable-next-line ts/no-explicit-any
  beforeSave?: (data: any) => Promise<void>
  importConfig?: ImportConfig
}

// eslint-disable-next-line ts/no-explicit-any
export interface StdTableProps<T = any> {
  title?: string
  mode?: string
  rowKey?: string

  api: Curd<T>
  columns: Column[]
  // eslint-disable-next-line ts/no-explicit-any
  getParams?: Record<string, any>
  size?: string
  disableQueryParams?: boolean
  disableSearch?: boolean
  pithy?: boolean
  exportExcel?: boolean
  exportMaterial?: boolean
  // eslint-disable-next-line ts/no-explicit-any
  overwriteParams?: Record<string, any>
  disableView?: boolean
  disableModify?: boolean
  selectionType?: string
  sortable?: boolean
  disableDelete?: boolean
  disablePagination?: boolean
  sortableMoveHook?: (oldRow: number[], newRow: number[]) => boolean
  scrollX?: string | number
  // eslint-disable-next-line ts/no-explicit-any
  getCheckboxProps?: (record: any) => any
  bulkActions?: BulkActions
  inTrash?: boolean
  expandAll?: boolean
}
