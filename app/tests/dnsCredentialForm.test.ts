import type { DNSProviderForm } from '@/api/auto_cert'
import { describe, expect, test } from 'bun:test'
import {
  applyMethod,
  changedSettingsCount,
  configurationTarget,
  formMethods,
  initialMethod,
  settingFields,
  visibleCredentialFields,
} from '@/views/dns/components/credentialForm'

function cloudflareForm(): DNSProviderForm {
  return {
    fields: [
      { key: 'CF_DNS_API_TOKEN', label: 'API token', group: 'credential', secret: true },
      { key: 'CF_ZONE_API_TOKEN', label: 'Zone read token', group: 'credential', optional: true },
      { key: 'CF_API_EMAIL', label: 'Account email', group: 'credential' },
      { key: 'CF_API_KEY', label: 'Global API key', group: 'credential', secret: true },
      { key: 'CF_ACCOUNT_ID', label: 'Account ID', group: 'credential', optional: true },
      { key: 'CF_TTL', label: 'TXT record TTL', group: 'setting', default: '120', unit: 'seconds' },
      { key: 'CF_BASE_URL', label: 'API base URL', group: 'setting', default: 'https://api.example.com' },
    ],
    methods: [
      { name: 'API token', recommended: true, fields: ['CF_DNS_API_TOKEN', 'CF_ZONE_API_TOKEN'] },
      { name: 'Global API key', fields: ['CF_API_EMAIL', 'CF_API_KEY'] },
    ],
  }
}

describe('formMethods', () => {
  test('a single method is not offered', () => {
    expect(formMethods({ methods: [{ name: 'Only', fields: ['A'] }] })).toEqual([])
    expect(formMethods(undefined)).toEqual([])
    expect(formMethods(cloudflareForm()).map(m => m.name)).toEqual(['API token', 'Global API key'])
  })
})

describe('visibleCredentialFields', () => {
  test('shows the method fields plus fields no method lists', () => {
    const keys = visibleCredentialFields(cloudflareForm(), 'Global API key').map(f => f.key)
    expect(keys).toEqual(['CF_API_EMAIL', 'CF_API_KEY', 'CF_ACCOUNT_ID'])
  })

  test('shows every credential field without a method', () => {
    const keys = visibleCredentialFields(cloudflareForm(), undefined).map(f => f.key)
    expect(keys).toEqual(['CF_DNS_API_TOKEN', 'CF_ZONE_API_TOKEN', 'CF_API_EMAIL', 'CF_API_KEY', 'CF_ACCOUNT_ID'])
  })

  test('fields without a group count as credentials', () => {
    const keys = visibleCredentialFields({ fields: [{ key: 'A' }, { key: 'B', group: 'setting' }] }, undefined).map(f => f.key)
    expect(keys).toEqual(['A'])
    expect(settingFields({ fields: [{ key: 'A' }, { key: 'B', group: 'setting' }] }).map(f => f.key)).toEqual(['B'])
  })
})

describe('initialMethod', () => {
  test('defaults to the recommended method', () => {
    expect(initialMethod(cloudflareForm(), {})).toBe('API token')
  })

  test('falls back to the first method when none is recommended', () => {
    const form = cloudflareForm()
    form.methods![0].recommended = false
    expect(initialMethod(form, {})).toBe('API token')
  })

  test('picks the method whose keys hold values when editing', () => {
    expect(initialMethod(cloudflareForm(), { CF_API_EMAIL: 'a@b.c', CF_API_KEY: 'k' })).toBe('Global API key')
  })

  test('prefers the method with more values', () => {
    expect(initialMethod(cloudflareForm(), { CF_DNS_API_TOKEN: 't', CF_API_EMAIL: 'a@b.c', CF_API_KEY: 'k' })).toBe('Global API key')
  })

  test('blank values do not count', () => {
    expect(initialMethod(cloudflareForm(), { CF_API_EMAIL: '  ' })).toBe('API token')
  })

  test('is undefined without methods', () => {
    expect(initialMethod({ fields: [{ key: 'A' }] }, { A: 'x' })).toBeUndefined()
  })
})

describe('applyMethod', () => {
  test('moves keys of other methods out and restores them on switch back', () => {
    const form = cloudflareForm()
    const credentials: Record<string, string> = {
      CF_DNS_API_TOKEN: 'token',
      CF_API_EMAIL: 'a@b.c',
      CF_API_KEY: 'key',
      CF_ACCOUNT_ID: 'acc',
    }
    const stash = {}

    applyMethod(credentials, form, 'API token', stash)
    expect(credentials).toEqual({ CF_DNS_API_TOKEN: 'token', CF_ACCOUNT_ID: 'acc' })

    applyMethod(credentials, form, 'Global API key', stash)
    expect(credentials).toEqual({ CF_API_EMAIL: 'a@b.c', CF_API_KEY: 'key', CF_ACCOUNT_ID: 'acc' })

    applyMethod(credentials, form, 'API token', stash)
    expect(credentials).toEqual({ CF_DNS_API_TOKEN: 'token', CF_ACCOUNT_ID: 'acc' })
  })

  test('keeps keys shared by the selected method', () => {
    const form: DNSProviderForm = {
      fields: [{ key: 'TENANT' }, { key: 'SECRET' }, { key: 'CERT' }],
      methods: [
        { name: 'Secret', fields: ['TENANT', 'SECRET'] },
        { name: 'Certificate', fields: ['TENANT', 'CERT'] },
      ],
    }
    const credentials: Record<string, string> = { TENANT: 't', SECRET: 's', CERT: '/c.pem' }
    applyMethod(credentials, form, 'Certificate', {})
    expect(credentials).toEqual({ TENANT: 't', CERT: '/c.pem' })
  })

  test('a value typed after switching wins over the stash', () => {
    const form = cloudflareForm()
    const credentials: Record<string, string> = { CF_DNS_API_TOKEN: 'old' }
    const stash = {}
    applyMethod(credentials, form, 'Global API key', stash)
    credentials.CF_API_KEY = 'k'
    applyMethod(credentials, form, 'API token', stash)
    credentials.CF_DNS_API_TOKEN = 'new'
    applyMethod(credentials, form, 'Global API key', stash)
    applyMethod(credentials, form, 'API token', stash)
    expect(credentials.CF_DNS_API_TOKEN).toBe('new')
  })

  test('does nothing without a matching method', () => {
    const credentials = { CF_API_KEY: 'k' }
    applyMethod(credentials, cloudflareForm(), undefined, {})
    applyMethod(credentials, { fields: [] }, 'API token', {})
    expect(credentials).toEqual({ CF_API_KEY: 'k' })
  })
})

describe('configurationTarget', () => {
  test('settings go to additional, everything else to credentials', () => {
    expect(configurationTarget({ key: 'A', group: 'setting' })).toBe('additional')
    expect(configurationTarget({ key: 'B', group: 'credential' })).toBe('credentials')
    expect(configurationTarget({ key: 'C' })).toBe('credentials')
  })
})

describe('changedSettingsCount', () => {
  test('counts values other than empty or the default', () => {
    const fields = settingFields(cloudflareForm())
    const values: Record<string, string> = { CF_TTL: '120' }
    expect(changedSettingsCount(fields, f => values[f.key])).toBe(0)
    values.CF_TTL = '300'
    values.CF_BASE_URL = ' '
    expect(changedSettingsCount(fields, f => values[f.key])).toBe(1)
  })
})
