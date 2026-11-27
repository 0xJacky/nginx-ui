import { describe, expect, test } from 'bun:test'
import { coreHttpFor } from '../src/plugin/coreHttp'

const client = {
  get: async () => 'get',
  post: async () => 'post',
  put: async () => 'put',
  delete: async () => 'delete',
  patch: async () => 'patch',
} as Parameters<typeof coreHttpFor>[2]

describe('coreHttpFor', () => {
  test('hands the host client to a plugin with core_api', async () => {
    const coreHttp = coreHttpFor('com.example.demo', ['kv', 'core_api'], client)
    expect(coreHttp).toBe(client)
    expect(await coreHttp.get('/settings')).toBe('get')
  })

  test('rejects every call of a plugin without core_api', async () => {
    const warn = console.warn
    const warnings: unknown[] = []
    console.warn = (...args: unknown[]) => warnings.push(args[0])

    try {
      for (const permissions of [
        ['kv', 'network'],
        [],
        undefined,
      ]) {
        const coreHttp = coreHttpFor('com.example.demo', permissions, client)
        expect(coreHttp).not.toBe(client)
        await expect(coreHttp.get('/dns_credentials')).rejects.toThrow('core_api')
        await expect(coreHttp.post('/settings', {})).rejects.toThrow('core_api')
        await expect(coreHttp.put('/settings', {})).rejects.toThrow('core_api')
        await expect(coreHttp.delete('/settings')).rejects.toThrow('core_api')
        await expect(coreHttp.patch('/settings', {})).rejects.toThrow('core_api')
      }
    }
    finally {
      console.warn = warn
    }

    // One warning per plugin registry, however often the bundle calls.
    expect(warnings).toHaveLength(3)
    expect(String(warnings[0])).toContain('com.example.demo')
  })
})
