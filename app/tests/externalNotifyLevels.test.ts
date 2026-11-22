import { describe, expect, test } from 'bun:test'

Object.assign(globalThis, {
  $gettext: (text: string) => text,
})

const {
  notificationLevelOptions,
  notificationLevels,
  receivedLevels,
} = await import('../src/views/preference/components/ExternalNotify/levels')

describe('receivedLevels', () => {
  test('a notifier without a selection receives every level', () => {
    expect(receivedLevels(null)).toEqual(['info', 'success', 'warning', 'error'])
    expect(receivedLevels(undefined)).toEqual(['info', 'success', 'warning', 'error'])
  })

  test('an explicit selection is kept as is', () => {
    expect(receivedLevels(['error'])).toEqual(['error'])
    expect(receivedLevels(['warning', 'error'])).toEqual(['warning', 'error'])
  })

  test('an empty selection stays empty instead of falling back to every level', () => {
    expect(receivedLevels([])).toEqual([])
  })
})

describe('notificationLevelOptions', () => {
  test('offers one option per level the server accepts', () => {
    expect(notificationLevelOptions().map(option => option.value)).toEqual(notificationLevels)
  })
})
