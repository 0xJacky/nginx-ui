import type { VNode } from 'vue'
import type { Cert } from '../src/api/cert'
import { describe, expect, mock, test } from 'bun:test'
import { h } from 'vue'
import * as vueJsx from 'vue/jsx-runtime'
import { ConfigStatus } from '../src/constants'

function interpolate(message: string, args?: Record<string, unknown>) {
  return message.replace(/%\{(\w+)\}/g, (_, key: string) => String(args?.[key] ?? `%{${key}}`))
}
Object.assign(globalThis, {
  h,
  $gettext: interpolate,
  $ngettext: (singular: string, plural: string, count: number, args?: Record<string, unknown>) =>
    interpolate(count === 1
      ? singular
      : plural, args),
})

// Bun compiles JSX against React's runtime, the app against Vue's.
mock.module('react/jsx-dev-runtime', () => vueJsx)
mock.module('react/jsx-runtime', () => vueJsx)
// Bun cannot load a single-file component, so a stand-in receives the props.
const ConfigUsage = {
  name: 'ConfigUsage',
  props: ['usages', 'summary'],
  render: () => null,
}
mock.module('../src/components/ConfigUsage', () => ({ default: ConfigUsage }))

const { certListColumns, certListNarrowColumns }
  = await import('../src/views/certificate/CertificateList/listColumns')

interface Column {
  title?: string
  key?: string
  dataIndex?: string | string[]
  width?: number
  fixed?: string
  render?: (value: unknown, record: Cert, index: number) => unknown
}

function desktopColumns() {
  return certListColumns(() => undefined) as Column[]
}

function columnId(column: Column) {
  return column.key ?? [column.dataIndex].flat().join('.')
}

function usageCell(record: Partial<Cert>) {
  const column = desktopColumns().find(item => item.dataIndex === 'used_by')
  return column?.render?.(record.used_by, record as Cert, 0) as VNode
}

describe('certificate list columns', () => {
  test('the desktop table shows which configurations use a certificate', () => {
    const columns = desktopColumns()
    expect(columns.map(columnId)).toEqual([
      'name',
      'domains',
      'state',
      'renewal_method',
      'certificate_info.not_after',
      'used_by',
      'actions',
    ])
    expect(columns.find(item => item.dataIndex === 'used_by')?.title).toBe('Used By')
  })

  test('a certificate lists the sites and streams that load it', () => {
    const cell = usageCell({
      id: 1,
      name: 'example.com',
      used_by: [
        { kind: 'site', name: 'example.com', status: ConfigStatus.Enabled },
        { kind: 'stream', name: 'mail relay', status: ConfigStatus.Disabled },
      ],
    })
    expect(cell.type).toBe(ConfigUsage)
    expect(cell.props?.usages).toEqual([
      {
        key: 'site:example.com',
        label: 'example.com',
        kind: 'site',
        to: '/sites/example.com',
        status: ConfigStatus.Enabled,
      },
      {
        key: 'stream:mail relay',
        label: 'mail relay',
        kind: 'stream',
        to: '/streams/mail%20relay',
        status: ConfigStatus.Disabled,
      },
    ])
    expect(cell.props?.summary).toBe('Used by 2 configurations')
  })

  // The API omits used_by for a certificate nothing loads; ConfigUsage then
  // shows its "Not used" text.
  test('an unused certificate renders an empty usage list', () => {
    const cell = usageCell({ id: 2, name: 'spare' })
    expect(cell.type).toBe(ConfigUsage)
    expect(cell.props?.usages).toEqual([])
  })

  test('every desktop column has a width, and only the actions stay fixed', () => {
    const columns = desktopColumns()
    expect(columns.every(item => typeof item.width === 'number')).toBe(true)
    expect(columns.filter(item => item.fixed).map(columnId)).toEqual(['actions'])
  })

  test('the phone layout keeps the name and the actions only', () => {
    const columns = certListNarrowColumns(() => undefined) as Column[]
    expect(columns.map(columnId)).toEqual(['name', 'actions'])
  })

  test('the name opens the certificate', () => {
    const opened: number[] = []
    const columns = certListColumns(record => opened.push(record.id)) as Column[]
    const link = columns[0].render?.(undefined, { id: 7, name: 'example.com' } as Cert, 0) as VNode
    ;(link.props?.onClick as () => void)()
    expect(opened).toEqual([7])
  })
})
