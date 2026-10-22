import type { DNSProviderForm } from '@/api/auto_cert'
import { describe, expect, test } from 'bun:test'
import {
  applyMethod,
  changedSettingsCount,
  configurationTarget,
  formMethods,
  initialMethod,
  methodNeedsNoInput,
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

// Route 53 style form: keys, a named profile or the server's own role.
function awsForm(): DNSProviderForm {
  return {
    fields: [
      { key: 'AWS_ACCESS_KEY_ID', label: 'Access key ID', group: 'credential' },
      { key: 'AWS_SECRET_ACCESS_KEY', label: 'Secret access key', group: 'credential', secret: true },
      { key: 'AWS_PROFILE', label: 'Profile', group: 'credential' },
      { key: 'AWS_REGION', label: 'Region', group: 'setting' },
    ],
    methods: [
      { name: 'Access key', recommended: true, fields: ['AWS_ACCESS_KEY_ID', 'AWS_SECRET_ACCESS_KEY'] },
      { name: 'Profile', fields: ['AWS_PROFILE'], values: { AWS_SDK_LOAD_CONFIG: '1' } },
      { name: 'Instance role', values: { AWS_SDK_LOAD_CONFIG: '1', AWS_USE_INSTANCE_ROLE: 'true' } },
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

  test('prefers a method whose fixed values are all stored', () => {
    expect(initialMethod(awsForm(), { AWS_SDK_LOAD_CONFIG: '1', AWS_USE_INSTANCE_ROLE: 'true' })).toBe('Instance role')
    // Matching values beat filled fields.
    expect(initialMethod(awsForm(), { AWS_ACCESS_KEY_ID: 'a', AWS_SECRET_ACCESS_KEY: 's', AWS_SDK_LOAD_CONFIG: '1', AWS_USE_INSTANCE_ROLE: 'true' })).toBe('Instance role')
  })

  test('a partial value match does not count', () => {
    expect(initialMethod(awsForm(), { AWS_USE_INSTANCE_ROLE: 'false', AWS_SDK_LOAD_CONFIG: '1' })).toBe('Profile')
    expect(initialMethod(awsForm(), { AWS_USE_INSTANCE_ROLE: 'true' })).toBe('Access key')
  })

  test('methods without fields or matching values score nothing', () => {
    expect(initialMethod(awsForm(), {})).toBe('Access key')
    const form = awsForm()
    form.methods![0].recommended = false
    form.methods!.reverse()
    expect(initialMethod(form, {})).toBe('Instance role')
    expect(initialMethod(form, { AWS_ACCESS_KEY_ID: 'a' })).toBe('Access key')
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

  test('writes the fixed values of the selected method and drops the others', () => {
    const form = awsForm()
    const credentials: Record<string, string> = { AWS_ACCESS_KEY_ID: 'a', AWS_SECRET_ACCESS_KEY: 's' }
    const stash = {}

    applyMethod(credentials, form, 'Instance role', stash)
    expect(credentials).toEqual({ AWS_SDK_LOAD_CONFIG: '1', AWS_USE_INSTANCE_ROLE: 'true' })

    // A key shared with the selected method stays.
    applyMethod(credentials, form, 'Profile', stash)
    expect(credentials).toEqual({ AWS_SDK_LOAD_CONFIG: '1' })

    applyMethod(credentials, form, 'Access key', stash)
    expect(credentials).toEqual({ AWS_ACCESS_KEY_ID: 'a', AWS_SECRET_ACCESS_KEY: 's' })
    expect(stash).toEqual({})
  })

  test('overwrites a stored fixed value that differs', () => {
    const credentials: Record<string, string> = { AWS_USE_INSTANCE_ROLE: 'false' }
    applyMethod(credentials, awsForm(), 'Instance role', {})
    expect(credentials.AWS_USE_INSTANCE_ROLE).toBe('true')
  })

  test('does nothing without a matching method', () => {
    const credentials = { CF_API_KEY: 'k' }
    applyMethod(credentials, cloudflareForm(), undefined, {})
    applyMethod(credentials, { fields: [] }, 'API token', {})
    expect(credentials).toEqual({ CF_API_KEY: 'k' })
  })
})

// dnsupdate style form: Kerberos methods fix a field the TSIG method lets the user fill.
function dnsUpdateForm(): DNSProviderForm {
  return {
    fields: [
      { key: 'TSIG_KEY', group: 'credential' },
      { key: 'TSIG_ALGORITHM', group: 'credential', optional: true },
      { key: 'KRB5_PRINCIPAL', group: 'credential' },
    ],
    methods: [
      { name: 'TSIG key', recommended: true, fields: ['TSIG_KEY', 'TSIG_ALGORITHM'] },
      { name: 'Kerberos password', fields: ['KRB5_PRINCIPAL'], values: { TSIG_ALGORITHM: 'gss-tsig' } },
      { name: 'Kerberos keytab', values: { TSIG_ALGORITHM: 'gss-tsig' } },
    ],
  }
}

describe('applyMethod with a fixed value on a field', () => {
  test('stashes the typed value and restores it on switch back', () => {
    const form = dnsUpdateForm()
    const credentials: Record<string, string> = { TSIG_KEY: 'k', TSIG_ALGORITHM: 'hmac-sha256' }
    const stash = {}

    applyMethod(credentials, form, 'Kerberos password', stash)
    expect(credentials).toEqual({ TSIG_ALGORITHM: 'gss-tsig' })

    applyMethod(credentials, form, 'Kerberos keytab', stash)
    expect(credentials).toEqual({ TSIG_ALGORITHM: 'gss-tsig' })

    applyMethod(credentials, form, 'TSIG key', stash)
    expect(credentials).toEqual({ TSIG_KEY: 'k', TSIG_ALGORITHM: 'hmac-sha256' })
    expect(stash).toEqual({})
  })

  test('clears a fixed value left by another method', () => {
    const form = dnsUpdateForm()
    const credentials: Record<string, string> = { KRB5_PRINCIPAL: 'p', TSIG_ALGORITHM: 'gss-tsig' }
    const stash = {}
    expect(initialMethod(form, credentials)).toBe('Kerberos password')

    applyMethod(credentials, form, 'Kerberos password', stash)
    expect(credentials).toEqual({ KRB5_PRINCIPAL: 'p', TSIG_ALGORITHM: 'gss-tsig' })

    applyMethod(credentials, form, 'TSIG key', stash)
    expect(credentials).toEqual({})
    expect(stash).toEqual({ KRB5_PRINCIPAL: 'p' })

    applyMethod(credentials, form, 'Kerberos password', stash)
    expect(credentials).toEqual({ KRB5_PRINCIPAL: 'p', TSIG_ALGORITHM: 'gss-tsig' })
  })

  test('a fixed value match still picks the method', () => {
    expect(initialMethod(dnsUpdateForm(), { TSIG_ALGORITHM: 'gss-tsig' })).toBe('Kerberos password')
    expect(initialMethod(dnsUpdateForm(), { TSIG_KEY: 'k', TSIG_ALGORITHM: 'hmac-sha256' })).toBe('TSIG key')
  })
})

describe('methodNeedsNoInput', () => {
  test('is true for a method without fields when nothing else is shown', () => {
    expect(methodNeedsNoInput(awsForm(), 'Instance role')).toBe(true)
    expect(methodNeedsNoInput(awsForm(), 'Profile')).toBe(false)
    expect(methodNeedsNoInput(awsForm(), undefined)).toBe(false)
  })

  test('is false while unlisted credential fields remain', () => {
    const form = awsForm()
    form.fields!.push({ key: 'AWS_HOSTED_ZONE_ID', label: 'Hosted zone ID', group: 'credential', optional: true })
    expect(methodNeedsNoInput(form, 'Instance role')).toBe(false)
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
