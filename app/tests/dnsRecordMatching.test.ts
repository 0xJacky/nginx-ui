import type { DNSRecord } from '../src/api/dns'
import { describe, expect, test } from 'bun:test'
import { findMatchingDNSRecordIds } from '../src/utils/dnsRecordMatching'

function record(id: string, name: string, type = 'A'): DNSRecord {
  return { id, name, type, content: '192.0.2.1', ttl: 600 }
}

describe('DNS record matching', () => {
  test('selects every record whose full name matches server_name', () => {
    const records = [
      record('root', '@'),
      record('www-a', 'www'),
      record('www-aaaa', 'www', 'AAAA'),
      record('api', 'api'),
    ]

    expect(findMatchingDNSRecordIds('www.example.com', 'example.com', records))
      .toEqual(['www-a', 'www-aaaa'])
  })

  test('matches root, wildcard, and provider-returned fully qualified names', () => {
    const records = [
      record('root', '@'),
      record('wildcard', '*'),
      record('fqdn', 'WWW.Example.COM.'),
    ]

    expect(findMatchingDNSRecordIds('example.com.', 'Example.COM', records)).toEqual(['root'])
    expect(findMatchingDNSRecordIds('*.example.com', 'example.com.', records)).toEqual(['wildcard'])
    expect(findMatchingDNSRecordIds('www.example.com', 'example.com', records)).toEqual(['fqdn'])
  })

  test('does not select suffix or unrelated records', () => {
    const records = [
      record('partial', 'notwww'),
      record('foreign', 'www.other.example'),
    ]

    expect(findMatchingDNSRecordIds('www.example.com', 'example.com', records)).toEqual([])
  })
})
