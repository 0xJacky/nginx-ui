import type { SlotRegistration } from '../src/plugin/types'
import { describe, expect, test } from 'bun:test'
import {
  collectSlotsByPrefix,
  NGINX_LOG_COLUMN_SLOT_PREFIX,
  NGINX_LOG_VIEW_SLOT_PREFIX,
  registrationApplies,
  uniqueByKey,
} from '../src/plugin/slots'

function registration(pluginId: string, order = 0, extra: Partial<SlotRegistration> = {}): SlotRegistration {
  return { pluginId, component: {}, order, ...extra }
}

describe('collectSlotsByPrefix', () => {
  const slots = {
    'nginx_log.view:search': [registration('a', 5, { label: 'Search' })],
    'nginx_log.view:dashboard': [registration('a', 1, { label: 'Dashboard' }), registration('b', 1)],
    'nginx_log.view:': [registration('c')],
    'nginx_log.list.column:status': [registration('a', 2)],
    'nginx_log.list.toolbar': [registration('a')],
    'sidebar.footer': [registration('a')],
  }

  test('lists the registrations of one family with the key from the slot name', () => {
    const views = collectSlotsByPrefix(slots, NGINX_LOG_VIEW_SLOT_PREFIX)

    expect(views.map(item => [item.key, item.registration.pluginId])).toEqual([
      ['dashboard', 'a'],
      ['dashboard', 'b'],
      ['search', 'a'],
    ])
    expect(views[0].slot).toBe('nginx_log.view:dashboard')
  })

  test('does not mix in other families or a slot without a key', () => {
    const columns = collectSlotsByPrefix(slots, NGINX_LOG_COLUMN_SLOT_PREFIX)

    expect(columns.map(item => item.key)).toEqual(['status'])
    expect(collectSlotsByPrefix(slots, 'nothing:')).toEqual([])
  })

  test('orders by order and keeps registration order for equal values', () => {
    const ordered = collectSlotsByPrefix({
      'p:one': [registration('first', 1), registration('second', 1)],
      'p:two': [registration('third', 0), registration('fourth', 1)],
    }, 'p:')

    expect(ordered.map(item => item.registration.pluginId)).toEqual(['third', 'first', 'second', 'fourth'])
  })

  test('applies when() to the context and treats a throwing when() as false', () => {
    const withWhen = {
      'p:a': [
        registration('yes', 0, { when: ctx => ctx.type === 'access' }),
        registration('no', 0, { when: ctx => ctx.type === 'error' }),
        registration('broken', 0, { when: () => { throw new Error('boom') } }),
      ],
    }
    const error = console.error
    console.error = () => {}
    try {
      expect(collectSlotsByPrefix(withWhen, 'p:', { type: 'access' }).map(item => item.registration.pluginId)).toEqual(['yes'])
    }
    finally {
      console.error = error
    }
  })
})

describe('registrationApplies', () => {
  test('is true without a condition and follows the condition otherwise', () => {
    expect(registrationApplies(registration('a'))).toBe(true)
    expect(registrationApplies(registration('a', 0, { when: () => false }))).toBe(false)
    expect(registrationApplies(registration('a', 0, { when: ctx => ctx.x === 1 }), { x: 1 })).toBe(true)
  })
})

describe('uniqueByKey', () => {
  test('keeps the first registration of each key', () => {
    const items = collectSlotsByPrefix({
      'p:dup': [registration('first'), registration('second')],
      'p:other': [registration('third', 1)],
    }, 'p:')

    expect(uniqueByKey(items).map(item => item.registration.pluginId)).toEqual(['first', 'third'])
  })
})
