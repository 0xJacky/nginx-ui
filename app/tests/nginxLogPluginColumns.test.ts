import type { NginxLogRow } from '../src/plugin/types'
import type { PluginColumnRules } from '../src/views/nginx_log/pluginColumns'
import { describe, expect, test } from 'bun:test'
import {
  applyPluginColumns,
  compareSortValues,
  filterRowsByColumns,
  pluginColumnKey,
  sortRowsByColumn,
  stripPluginParams,
} from '../src/views/nginx_log/pluginColumns'

function row(name: string, extra: Record<string, unknown> = {}): NginxLogRow {
  return { path: `/var/log/nginx/${name}.log`, type: 'access', name: `${name}.log`, config_file: '', ...extra }
}

const rows = [
  row('a', { count: 30, status: 'indexed' }),
  row('b', { status: 'error' }),
  row('c', { count: 5, status: 'indexed' }),
  row('d', { count: 100, status: 'queued' }),
  row('e', { count: null, status: 'indexed' }),
]

const countColumn: PluginColumnRules = {
  columnKey: pluginColumnKey('p:count'),
  sortValue: r => r.count as number | null | undefined,
}

const names = (list: NginxLogRow[]) => list.map(item => item.name.replace('.log', ''))

describe('compareSortValues', () => {
  test('numbers by magnitude and strings by locale order', () => {
    expect(compareSortValues(2, 10)).toBeLessThan(0)
    expect(compareSortValues('2', '10')).toBeGreaterThan(0)
    expect(compareSortValues('a', 'b')).toBeLessThan(0)
  })
})

describe('sortRowsByColumn', () => {
  test('sorts ascending and descending with missing values last in both', () => {
    expect(names(sortRowsByColumn(rows, countColumn, 'asc'))).toEqual(['c', 'a', 'd', 'b', 'e'])
    expect(names(sortRowsByColumn(rows, countColumn, 'desc'))).toEqual(['d', 'a', 'c', 'b', 'e'])
  })

  test('is stable for equal values and does not touch the input', () => {
    const same = [row('x', { v: 1 }), row('y', { v: 1 }), row('z', { v: 0 })]
    const column = { columnKey: 'k', sortValue: (r: NginxLogRow) => r.v as number }

    expect(names(sortRowsByColumn(same, column, 'asc'))).toEqual(['z', 'x', 'y'])
    expect(names(sortRowsByColumn(same, column, 'desc'))).toEqual(['x', 'y', 'z'])
    expect(names(same)).toEqual(['x', 'y', 'z'])
  })

  test('sorts strings by locale order and treats NaN as missing', () => {
    const list = [row('m', { s: 'b' }), row('n', { s: 'a' }), row('o', { s: Number.NaN })]
    const column = { columnKey: 'k', sortValue: (r: NginxLogRow) => r.s as string | number }

    expect(names(sortRowsByColumn(list, column, 'asc'))).toEqual(['n', 'm', 'o'])
  })

  test('a throwing sortValue counts as missing', () => {
    const column = {
      columnKey: 'k',
      sortValue: (r: NginxLogRow) => {
        if (r.name === 'a.log')
          throw new Error('x')
        return 1
      },
    }
    const error = console.error
    console.error = () => {}
    try {
      expect(names(sortRowsByColumn(rows.slice(0, 2), column, 'asc'))).toEqual(['b', 'a'])
    }
    finally {
      console.error = error
    }
  })

  test('returns the rows as they are for a column without sortValue', () => {
    expect(sortRowsByColumn(rows, { columnKey: 'k' }, 'asc')).toBe(rows)
  })
})

describe('filterRowsByColumns', () => {
  const statusColumn: PluginColumnRules = {
    columnKey: pluginColumnKey('p:status'),
    filters: [
      { label: 'Indexed', value: 'indexed', match: r => r.status === 'indexed' },
      { label: 'Error', value: 'error', match: r => r.status === 'error' },
      { label: 'Queued', value: 'queued', match: r => r.status === 'queued' },
    ],
  }
  const largeColumn: PluginColumnRules = {
    columnKey: pluginColumnKey('p:large'),
    filters: [{ label: 'Large', value: 'large', match: r => Number(r.count) >= 30 }],
  }

  test('keeps every row without a selection', () => {
    expect(filterRowsByColumns(rows, [statusColumn], {})).toBe(rows)
    expect(filterRowsByColumns(rows, [statusColumn], { [statusColumn.columnKey]: [] })).toBe(rows)
    expect(filterRowsByColumns(rows, [statusColumn], { [statusColumn.columnKey]: null })).toBe(rows)
  })

  test('combines several choices of one column with OR', () => {
    const result = filterRowsByColumns(rows, [statusColumn], { [statusColumn.columnKey]: ['error', 'queued'] })

    expect(names(result)).toEqual(['b', 'd'])
  })

  test('combines columns with AND', () => {
    const result = filterRowsByColumns(rows, [statusColumn, largeColumn], {
      [statusColumn.columnKey]: ['indexed', 'queued'],
      [largeColumn.columnKey]: ['large'],
    })

    expect(names(result)).toEqual(['a', 'd'])
  })

  test('ignores unknown choices and columns without filters', () => {
    expect(filterRowsByColumns(rows, [statusColumn], { [statusColumn.columnKey]: ['nope'] })).toBe(rows)
    expect(filterRowsByColumns(rows, [countColumn], { [countColumn.columnKey]: ['x'] })).toBe(rows)
  })

  test('a throwing match keeps the row out', () => {
    const broken: PluginColumnRules = {
      columnKey: 'k',
      filters: [{
        label: 'x',
        value: 'x',
        match: () => {
          throw new Error('boom')
        },
      }],
    }
    const error = console.error
    console.error = () => {}
    try {
      expect(filterRowsByColumns(rows, [broken], { k: ['x'] })).toEqual([])
    }
    finally {
      console.error = error
    }
  })
})

describe('list request parameters', () => {
  const statusKey = pluginColumnKey('p:status')
  const statusColumn: PluginColumnRules = {
    columnKey: statusKey,
    sortValue: r => r.status as string,
    filters: [{ label: 'Indexed', value: 'indexed', match: r => r.status === 'indexed' }],
  }

  test('stripPluginParams removes the plugin sort and selections only', () => {
    expect(stripPluginParams({ sort_by: statusKey, order: 'desc', [statusKey]: ['indexed'], type: 'access' }))
      .toEqual({ type: 'access' })
    expect(stripPluginParams({ sort_by: 'name', order: 'asc', type: 'access' }))
      .toEqual({ sort_by: 'name', order: 'asc', type: 'access' })
  })

  test('stripPluginParams also removes leftovers of columns the list no longer shows', () => {
    const hiddenKey = pluginColumnKey('p:hidden')
    expect(stripPluginParams({ sort_by: hiddenKey, order: 'asc', [hiddenKey]: ['x'], name: 'a' }))
      .toEqual({ name: 'a' })
  })

  test('applyPluginColumns filters and then sorts by the named column', () => {
    const result = applyPluginColumns(rows, { sort_by: statusKey, order: 'desc', [statusKey]: ['indexed'] }, [statusColumn])

    expect(names(result)).toEqual(['a', 'c', 'e'])
    expect(applyPluginColumns(rows, { sort_by: 'name', order: 'asc' }, [statusColumn])).toEqual(rows)
    expect(names(applyPluginColumns(rows, { sort_by: statusKey }, [statusColumn]))).toEqual(['b', 'a', 'c', 'e', 'd'])
  })
})
