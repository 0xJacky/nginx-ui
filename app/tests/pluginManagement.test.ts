import { describe, expect, test } from 'bun:test'
import { findTrustedOffer } from '../src/views/system/plugins/marketplace/trust'
import { changedSettingKeys, cloneSettings } from '../src/views/system/plugins/settingsForm'
import { fallbackCatalogEntry, findSingleUpdate, findSingleUpdateEntry } from '../src/views/system/plugins/updates'
import { formatUsagePreview, previewUsage } from '../src/views/system/plugins/usage'

// Stand in for vue3-gettext: English text with %{name} placeholders filled.
function fakeGettext(text: string, params: Record<string, string> = {}) {
  return text.replace(/%\{(\w+)\}/g, (_, key) => params[key] ?? '')
}
Object.assign(globalThis, {
  $gettext: fakeGettext,
  $pgettext: (_context: string, text: string) => text,
})

function usage(names: string[], total = names.length) {
  return {
    items: names.map((name, i) => ({
      kind: 'certificate',
      id: String(i + 1),
      name,
    })),
    total,
  }
}

describe('previewUsage', () => {
  test('lists up to three names and counts the rest', () => {
    const preview = previewUsage(usage(['a', 'b', 'c', 'd', 'e']))
    expect(preview).toEqual({
      names: ['a', 'b', 'c'],
      more: 2,
    })
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
    const saved = {
      servers: ['1.1.1.1:53'],
      name: 'a',
      on: false,
    }
    expect(changedSettingKeys(fields, { ...saved }, saved)).toEqual([])
    expect(changedSettingKeys(fields, {
      ...saved,
      servers: ['1.1.1.1:53', ''],
    }, saved)).toEqual(['servers'])
    expect(changedSettingKeys(fields, { ...saved, on: true, name: 'b' }, saved)).toEqual(['name', 'on'])
  })

  test('treats missing, empty text and empty lists alike', () => {
    expect(changedSettingKeys(fields, { servers: [], name: '' }, {})).toEqual([])
  })

  test('the clone does not share lists with the source', () => {
    const saved = {
      servers: ['a'],
    }
    const copy = cloneSettings(saved)
    ;(copy.servers as string[]).push('b')
    expect(saved.servers).toEqual(['a'])
  })
})

describe('plugin update helpers', () => {
  const pluginA = {
    id: 'com.nginxui.dns01',
    name: 'DNS-01 Challenge',
    name_i18n: { en: 'DNS-01 Challenge', zh_CN: 'DNS-01 验证' },
    version: '1.0.2',
    api_version: 1,
    enabled: true,
    status: 'stopped' as const,
  }
  const pluginB = {
    id: 'com.example.logs',
    name: 'Logs',
    version: '1.0.0',
    api_version: 1,
    enabled: true,
    status: 'running' as const,
  }
  const updateA = {
    id: 'com.nginxui.dns01',
    name: 'DNS-01 Challenge',
    installed_version: '1.0.2',
    latest_version: '1.0.3',
    source: 'https://plugins.nginxui.com/index.json',
    trust: 'official' as const,
    installed_yanked: false,
    permissions_changed: false,
    release: { version: '1.0.3', api_version: 1, download_url: 'https://plugins.nginxui.com/a.tar.gz' },
  }
  const updateB = {
    ...updateA,
    id: 'com.example.logs',
    name: 'Logs',
    installed_version: '1.0.0',
    latest_version: '1.1.0',
  }

  test('findSingleUpdate returns the match only when exactly one installed plugin has an update', () => {
    expect(findSingleUpdate([pluginA, pluginB], [])).toBeUndefined()
    expect(findSingleUpdate([pluginA, pluginB], [updateA])).toEqual({ plugin: pluginA, update: updateA })
    expect(findSingleUpdate([pluginA, pluginB], [updateA, updateB])).toBeUndefined()
    // Ignores updates that do not match an installed plugin.
    expect(findSingleUpdate([pluginA], [updateA, updateB])).toEqual({ plugin: pluginA, update: updateA })
  })

  test('fallbackCatalogEntry carries the update release and installed state', () => {
    const entry = fallbackCatalogEntry(pluginA, updateA)
    expect(entry.id).toBe(pluginA.id)
    expect(entry.name).toEqual(pluginA.name_i18n)
    expect(entry.installed_version).toBe('1.0.2')
    expect(entry.installable_release?.version).toBe('1.0.3')
    expect(entry.update_available).toBe(true)
    expect(entry.source).toBe(updateA.source)
  })

  test('findSingleUpdateEntry prefers current entries and falls back to cached catalog', () => {
    const entryInList = fallbackCatalogEntry(pluginA, updateA)
    const entryInCatalog = { ...entryInList, source: 'https://cached.example/index.json' }
    expect(findSingleUpdateEntry([updateA], [entryInList], [entryInCatalog])).toEqual(entryInList)
    expect(findSingleUpdateEntry([updateA], [], [entryInCatalog])).toEqual(entryInCatalog)
    expect(findSingleUpdateEntry([updateA, updateB], [entryInList], [entryInCatalog])).toBeUndefined()
  })
})
