import type { DnsCredentialPage } from '@/components/DnsCredentialEditor/dnsCredentialSummaries'
import { describe, expect, test } from 'bun:test'
import { collectDnsCredentialSummaries } from '@/components/DnsCredentialEditor/dnsCredentialSummaries'

function credential(id: number) {
  return {
    id,
    name: `cred-${id}`,
    code: 'cloudflare',
    provider: 'Cloudflare',
    provider_code: 'cloudflare',
    config: {
      configuration: {
        credentials: { CF_DNS_API_TOKEN: 'secret' },
      },
    },
    configuration: {
      credentials: { CF_DNS_API_TOKEN: 'secret' },
    },
    created_at: '2026-01-01T00:00:00Z',
  }
}

describe('collectDnsCredentialSummaries', () => {
  test('reads every page and stops at a short one', async () => {
    const pages: Record<number, DnsCredentialPage> = {
      1: {
        data: [
          credential(1),
          credential(2),
        ],
        pagination: { per_page: 2 },
      },
      2: {
        data: [
          credential(3),
        ],
        pagination: { per_page: 2 },
      },
    }
    const requested: number[] = []

    const summaries = await collectDnsCredentialSummaries(async page => {
      requested.push(page)
      return pages[page]
    })

    expect(requested).toEqual([1, 2])
    expect(summaries.map(item => item.id)).toEqual([1, 2, 3])
  })

  test('drops the configuration and every other field', async () => {
    const [summary] = await collectDnsCredentialSummaries(async () => ({
      data: [
        credential(7),
      ],
    }))

    expect(summary).toEqual({
      id: 7,
      name: 'cred-7',
      code: 'cloudflare',
      provider: 'Cloudflare',
      provider_code: 'cloudflare',
    })
    expect(JSON.stringify(summary)).not.toContain('secret')
  })

  test('stops when the response has no pagination', async () => {
    let calls = 0
    const summaries = await collectDnsCredentialSummaries(async () => {
      calls++
      return {
        data: [
          credential(1),
        ],
      }
    })

    expect(calls).toBe(1)
    expect(summaries).toHaveLength(1)
  })

  test('rejects instead of returning a partial list', async () => {
    const fetchPage = async (page: number): Promise<DnsCredentialPage> => {
      if (page === 2)
        throw new Error('network down')
      return {
        data: [
          credential(1),
        ],
        pagination: { per_page: 1 },
      }
    }

    await expect(collectDnsCredentialSummaries(fetchPage)).rejects.toThrow('network down')
  })
})
