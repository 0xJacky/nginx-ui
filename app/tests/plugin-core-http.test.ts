import { afterEach, beforeEach, describe, expect, test } from 'bun:test'
import { coreHttpFor, deprecatedSharedHttp } from '../src/plugin/coreHttp'

type CoreHttp = Parameters<typeof coreHttpFor>[2]

const calls: string[] = []

const client = {
  get: async (url: string) => {
    calls.push(`get ${url}`)
    return 'get'
  },
  post: async (url: string) => {
    calls.push(`post ${url}`)
    return 'post'
  },
  put: async (url: string) => {
    calls.push(`put ${url}`)
    return 'put'
  },
  delete: async (url: string) => {
    calls.push(`delete ${url}`)
    return 'delete'
  },
  patch: async (url: string) => {
    calls.push(`patch ${url}`)
    return 'patch'
  },
} as CoreHttp

const warn = console.warn
let warnings: unknown[] = []

beforeEach(() => {
  calls.length = 0
  warnings = []
  console.warn = (...args: unknown[]) => warnings.push(args[0])
})

afterEach(() => {
  console.warn = warn
})

async function callEveryMethod(coreHttp: CoreHttp) {
  return [
    await coreHttp.get('/a'),
    await coreHttp.post('/b', {}),
    await coreHttp.put('/c', {}),
    await coreHttp.delete('/d'),
    await coreHttp.patch('/e', {}),
  ]
}

describe('coreHttpFor', () => {
  test('hands the host client to a plugin with core_api in either mode', () => {
    expect(coreHttpFor('com.example.demo', ['kv', 'core_api'], client, false)).toBe(client)
    expect(coreHttpFor('com.example.demo', ['kv', 'core_api'], client, true)).toBe(client)
    expect(warnings).toHaveLength(0)
  })

  test('warns once but still calls the host API while not enforced', async () => {
    const coreHttp = coreHttpFor('com.example.demo', ['network'], client, false)
    expect(coreHttp).not.toBe(client)

    expect(await callEveryMethod(coreHttp)).toEqual([
      'get',
      'post',
      'put',
      'delete',
      'patch',
    ])
    expect(calls).toEqual([
      'get /a',
      'post /b',
      'put /c',
      'delete /d',
      'patch /e',
    ])
    expect(warnings).toHaveLength(1)
    expect(String(warnings[0])).toContain('com.example.demo')
    expect(String(warnings[0])).toContain('deprecated')
  })

  test('rejects every call of a plugin without core_api once enforced', async () => {
    for (const permissions of [
      ['kv', 'network'],
      [],
      undefined,
    ]) {
      const coreHttp = coreHttpFor('com.example.demo', permissions, client, true)
      await expect(coreHttp.get('/dns_credentials')).rejects.toThrow('core_api')
      await expect(coreHttp.post('/settings', {})).rejects.toThrow('core_api')
      await expect(coreHttp.put('/settings', {})).rejects.toThrow('core_api')
      await expect(coreHttp.delete('/settings')).rejects.toThrow('core_api')
      await expect(coreHttp.patch('/settings', {})).rejects.toThrow('core_api')
    }

    expect(calls).toHaveLength(0)
    // One warning per plugin registry, however often the bundle calls.
    expect(warnings).toHaveLength(3)
  })
})

describe('deprecatedSharedHttp', () => {
  test('calls the host API and warns once', async () => {
    const shared = deprecatedSharedHttp(client)

    expect(await callEveryMethod(shared)).toEqual([
      'get',
      'post',
      'put',
      'delete',
      'patch',
    ])
    expect(calls).toHaveLength(5)
    expect(warnings).toHaveLength(1)
    expect(String(warnings[0])).toContain('window.NginxUI.shared.http')
  })
})
