import { describe, expect, test } from 'bun:test'

function fakeGettext(text: string, params: Record<string, string> = {}) {
  return text.replace(/%\{(\w+)\}/g, (_, key) => params[key] ?? '')
}
Object.assign(globalThis, { $gettext: fakeGettext, $pgettext: (_context: string, text: string) => text })

const {
  conflictingIds,
  conflictNames,
  conflictNote,
  enabledConflicts,
  enableReplacesText,
  installReplacesText,
} = await import('../src/views/system/plugins/conflicts')

const installed = [
  { id: 'a.one', name: 'One', enabled: true, conflicts: [] as string[] },
  { id: 'a.two', name: 'Two', enabled: false, conflicts: ['a.three'] },
  { id: 'a.three', name: 'Three', enabled: true },
  { id: 'a.four', name: 'Four', enabled: true, conflicts: ['a.one'] },
]

describe('conflictingIds', () => {
  test('follows a declaration from either side', () => {
    expect(conflictingIds({ id: 'a.one' }, installed)).toEqual(['a.four'])
    expect(conflictingIds({ id: 'a.three' }, installed)).toEqual(['a.two'])
    expect(conflictingIds({ id: 'a.two', conflicts: ['a.three'] }, installed)).toEqual(['a.three'])
  })

  test('merges both sides without repeats or the plugin itself', () => {
    const subject = { id: 'a.one', conflicts: ['a.four', 'a.one', 'a.zzz'] }
    expect(conflictingIds(subject, installed)).toEqual(['a.four', 'a.zzz'])
  })

  test('is empty without any declaration', () => {
    expect(conflictingIds({ id: 'a.new' }, installed)).toEqual([])
  })
})

describe('enabledConflicts', () => {
  test('keeps only the enabled installed plugins', () => {
    expect(enabledConflicts({ id: 'a.new', conflicts: ['a.two', 'a.three'] }, installed).map(item => item.id))
      .toEqual(['a.three'])
    expect(enabledConflicts({ id: 'a.one' }, installed).map(item => item.id)).toEqual(['a.four'])
    expect(enabledConflicts({ id: 'a.two' }, installed)).toEqual([])
  })
})

describe('conflictNames', () => {
  test('uses the display name of an installed plugin and the id otherwise', () => {
    expect(conflictNames(['a.one', 'b.missing'], installed, item => item.name)).toEqual(['One', 'b.missing'])
  })
})

describe('texts', () => {
  test('name the plugins without technical terms', () => {
    expect(conflictNote(['One', 'Two'])).toBe('Cannot be enabled together with One, Two')
    expect(enableReplacesText('Three', ['One'])).toBe('Enabling Three will disable One, because these plugins cannot be enabled at the same time.')
    expect(installReplacesText(['One'])).toContain('One')
  })
})
