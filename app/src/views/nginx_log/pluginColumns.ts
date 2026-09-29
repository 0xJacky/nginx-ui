import type { NginxLogRow, SlotColumnFilter, SlotSortValue } from '@/plugin/types'

/** Prefix of the table column key of a plugin column. */
const COLUMN_KEY_PREFIX = 'plugin_column_'

export function pluginColumnKey(key: string) {
  return `${COLUMN_KEY_PREFIX}${key}`
}

/** A plugin column reduced to what sorting and filtering need. */
export interface PluginColumnRules {
  /** Table column key, see `pluginColumnKey`. */
  columnKey: string
  sortValue?: (row: NginxLogRow) => SlotSortValue
  filters?: SlotColumnFilter[]
}

/** Numbers by magnitude, everything else by locale order. */
export function compareSortValues(a: string | number, b: string | number): number {
  if (typeof a === 'number' && typeof b === 'number')
    return a - b

  return String(a).localeCompare(String(b))
}

function readSortValue(column: PluginColumnRules, row: NginxLogRow): string | number | null {
  let value: SlotSortValue
  try {
    value = column.sortValue?.(row)
  }
  catch (error) {
    console.error(`[plugin] sort value of ${column.columnKey} failed`, error)
    return null
  }

  if (value === null || value === undefined)
    return null

  if (typeof value === 'number')
    return Number.isNaN(value) ? null : value

  return String(value)
}

/**
 * Sorts rows by a column. Rows without a value come last in either direction,
 * and equal values keep their order.
 */
export function sortRowsByColumn<T extends NginxLogRow>(
  rows: T[],
  column: PluginColumnRules,
  order: 'asc' | 'desc',
): T[] {
  if (!column.sortValue)
    return rows

  const direction = order === 'desc' ? -1 : 1
  const keyed = rows.map((row, index) => ({ row, index, value: readSortValue(column, row) }))

  keyed.sort((a, b) => {
    if (a.value === null || b.value === null) {
      if (a.value === b.value)
        return a.index - b.index

      return a.value === null ? 1 : -1
    }

    return direction * compareSortValues(a.value, b.value) || a.index - b.index
  })

  return keyed.map(item => item.row)
}

function matches(filter: SlotColumnFilter, row: NginxLogRow): boolean {
  try {
    return filter.match(row) === true
  }
  catch (error) {
    console.error(`[plugin] filter ${filter.value} failed`, error)
    return false
  }
}

/**
 * Keeps the rows that pass every column with a selection. Several selected
 * choices of one column combine with OR, columns combine with AND.
 */
export function filterRowsByColumns<T extends NginxLogRow>(
  rows: T[],
  columns: PluginColumnRules[],
  selected: Record<string, string[] | null | undefined>,
): T[] {
  const active = columns
    .map(column => {
      const values = selected[column.columnKey]
      if (!values || values.length === 0 || !column.filters)
        return null

      const chosen = column.filters.filter(filter => values.includes(filter.value))
      return chosen.length > 0 ? chosen : null
    })
    .filter((chosen): chosen is SlotColumnFilter[] => chosen !== null)

  if (active.length === 0)
    return rows

  return rows.filter(row => active.every(chosen => chosen.some(filter => matches(filter, row))))
}

/** Query parameters of the list request that belong to plugin columns. */
export interface PluginListParams {
  sort_by?: string
  order?: string
  [key: string]: unknown
}

/**
 * Removes what only the browser can apply from the parameters sent to the
 * server: the sort of a plugin column and the selections of plugin filters.
 */
export function stripPluginParams<T extends PluginListParams>(params: T, columns: PluginColumnRules[]): T {
  const keys = new Set(columns.map(column => column.columnKey))
  const next: PluginListParams = { ...params }

  if (next.sort_by && keys.has(next.sort_by)) {
    delete next.sort_by
    delete next.order
  }
  for (const key of keys)
    delete next[key]

  return next as T
}

/** Applies the filters and the sort of plugin columns named in `params`. */
export function applyPluginColumns<T extends NginxLogRow>(
  rows: T[],
  params: PluginListParams,
  columns: PluginColumnRules[],
): T[] {
  const selected: Record<string, string[] | null | undefined> = {}
  for (const column of columns) {
    const value = params[column.columnKey]
    selected[column.columnKey] = Array.isArray(value) ? value.map(String) : null
  }

  const filtered = filterRowsByColumns(rows, columns, selected)

  const sortColumn = params.sort_by ? columns.find(column => column.columnKey === params.sort_by) : undefined
  if (!sortColumn)
    return filtered

  return sortRowsByColumn(filtered, sortColumn, params.order === 'desc' ? 'desc' : 'asc')
}
