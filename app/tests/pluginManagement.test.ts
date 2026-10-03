import { describe, expect, test } from 'bun:test'
import { findTrustedOffer } from '../src/views/system/plugins/marketplace/trust'
import { changedSettingKeys, cloneSettings } from '../src/views/system/plugins/settingsForm'
import { formatUsagePreview, previewUsage } from '../src/views/system/plugins/usage'

// Stand in for vue3-gettext: English text with %{name} placeholders filled.
function fakeGettext(text: string, params: Record<string, string> = {}) {
  return text.replace(/%\{(\w+)\}/g, (_, key) => params[key] ?? '')
}
Object.assign(globalThis, { $gettext: fakeGettext, $pgettext: (_context: string, text: string) => text })

function usage(names: string[], total = names.length) {
  return { items: names.map((name, i) => ({ kind: 'certificate', id: String(i + 1), name })), total }
}

describe('previewUsage', () => {
  test('lists up to three names and counts the rest', () => {
    const preview = previewUsage(usage(['a', 'b', 'c', 'd', 'e']))
    expect(preview).toEqual({ names: ['a', 'b', 'c'], more: 2 })
    expect(formatUsagePreview(preview)).toBe('a, b, c and 2 more')
  })

  test('counts items the backend left out of the list', () => {
    const preview = previewUsage(usage(['a'], 150))
    expect(preview.more).toBe(149)
  })

  test('has nothing more when every name is listed', () => {
    expect(formatUsagePreview(previewUsage(usage(['a', 'b'])))).toBe('a, b')
  })
})

describe('findTrustedOffer', () => {
  const entry = {
    id: 'com.nginxui.dns01',
    trust: 'official' as const,
    releases: [],
    source: 'https://plugins.example/index.json',
    installable_release: { version: '1.0.0', api_version: 1, download_url: 'https://plugins.example/p.tar.gz' },
    update_available: false,
  }

  test('offers the official package for an unsigned or community install', () => {
    expect(findTrustedOffer('unsigned', entry)).toEqual({ entry, trust: 'official', version: '1.0.0' })
    expect(findTrustedOffer('community', { ...entry, trust: 'verified' })?.trust).toBe('verified')
  })

  test('offers nothing when the install is already trusted or the entry is not', () => {
    expect(findTrustedOffer('official', entry)).toBeUndefined()
    expect(findTrustedOffer('verified', entry)).toBeUndefined()
    expect(findTrustedOffer(undefined, entry)).toBeUndefined()
    expect(findTrustedOffer('unsigned', { ...entry, trust: 'community' })).toBeUndefined()
    expect(findTrustedOffer('unsigned', { ...entry, installable_release: undefined })).toBeUndefined()
    expect(findTrustedOffer('unsigned', undefined)).toBeUndefined()
  })
})

describe('changedSettingKeys', () => {
  const fields = [
    { key: 'servers', type: 'list' as const, display_name: 'Servers' },
    { key: 'name', type: 'text' as const, display_name: 'Name' },
    { key: 'on', type: 'bool' as const, display_name: 'On' },
  ]

  test('reports only the fields that differ', () => {
    const saved = { servers: ['1.1.1.1:53'], name: 'a', on: false }
    expect(changedSettingKeys(fields, { ...saved }, saved)).toEqual([])
    expect(changedSettingKeys(fields, { ...saved, servers: ['1.1.1.1:53', ''] }, saved)).toEqual(['servers'])
    expect(changedSettingKeys(fields, { ...saved, on: true, name: 'b' }, saved)).toEqual(['name', 'on'])
  })

  test('treats missing, empty text and empty lists alike', () => {
    expect(changedSettingKeys(fields, { servers: [], name: '' }, {})).toEqual([])
  })

  test('the clone does not share lists with the source', () => {
    const saved = { servers: ['a'] }
    const copy = cloneSettings(saved)
    ;(copy.servers as string[]).push('b')
    expect(saved.servers).toEqual(['a'])
  })
})
