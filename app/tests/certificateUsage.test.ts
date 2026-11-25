import type { VNode } from 'vue'
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

const { default: certColumns } = await import('../src/views/certificate/CertificateList/certColumns')

function loadUsage() {
  return import('../src/views/certificate/certUsage')
}

describe('certificate usage', () => {
  test('a site and a stream link to their editors', async () => {
    const { toConfigUsages } = await loadUsage()
    expect(toConfigUsages([
      { kind: 'site', name: 'example.com', status: ConfigStatus.Enabled },
      { kind: 'stream', name: 'mail', status: ConfigStatus.Maintenance },
    ])).toEqual([
      {
        key: 'site:example.com',
        label: 'example.com',
        kind: 'site',
        to: '/sites/example.com',
        status: ConfigStatus.Enabled,
      },
      {
        key: 'stream:mail',
        label: 'mail',
        kind: 'stream',
        to: '/streams/mail',
        status: ConfigStatus.Maintenance,
      },
    ])
  })

  test('a name is escaped in the link and kept as is in the label', async () => {
    const { toConfigUsages } = await loadUsage()
    const [usage] = toConfigUsages([
      { kind: 'site', name: 'a b/c?d', status: ConfigStatus.Enabled },
    ])
    expect(usage.to).toBe('/sites/a%20b%2Fc%3Fd')
    expect(usage.label).toBe('a b/c?d')
  })

  test('a site and a stream of the same name get different keys', async () => {
    const { toConfigUsages } = await loadUsage()
    const keys = toConfigUsages([
      { kind: 'site', name: 'shared', status: ConfigStatus.Enabled },
      { kind: 'stream', name: 'shared', status: ConfigStatus.Enabled },
    ]).map(usage => usage.key)
    expect(keys).toEqual(['site:shared', 'stream:shared'])
  })

  test('a certificate nothing loads has no usages', async () => {
    const { renderCertUsage, toConfigUsages } = await loadUsage()
    expect(toConfigUsages()).toEqual([])
    expect(toConfigUsages([])).toEqual([])
    const cell = renderCertUsage(undefined) as VNode
    expect(cell.type).toBe(ConfigUsage)
    expect(cell.props?.usages).toEqual([])
  })

  test('the summary counts the configurations', async () => {
    const { renderCertUsage } = await loadUsage()
    const one = renderCertUsage([
      { kind: 'site', name: 'a', status: ConfigStatus.Enabled },
    ]) as VNode
    expect(one.props?.summary).toBe('Used by 1 configuration')
    const two = renderCertUsage([
      { kind: 'site', name: 'a', status: ConfigStatus.Enabled },
      { kind: 'stream', name: 'b', status: ConfigStatus.Enabled },
    ]) as VNode
    expect(two.props?.summary).toBe('Used by 2 configurations')
  })
})

// The certificate picker of the site editor shows the pure columns.
describe('certificate picker columns', () => {
  const pure = certColumns.filter(column => column.pure)
  const usedBy = pure.find(column => column.dataIndex === 'used_by')

  test('the usage column is one of them', () => {
    expect(usedBy).toBeDefined()
    expect((usedBy!.title as () => string)()).toBe('Used By')
  })

  test('it renders the configurations of a row', () => {
    const render = usedBy!.customRender as (args: { record: unknown }) => VNode
    const cell = render({
      record: {
        used_by: [
          { kind: 'site', name: 'example.com', status: ConfigStatus.Disabled },
        ],
      },
    })
    expect(cell.type).toBe(ConfigUsage)
    expect(cell.props?.usages).toEqual([
      {
        key: 'site:example.com',
        label: 'example.com',
        kind: 'site',
        to: '/sites/example.com',
        status: ConfigStatus.Disabled,
      },
    ])
    expect(cell.props?.summary).toBe('Used by 1 configuration')
  })
})
