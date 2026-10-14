import { describe, expect, test } from 'bun:test'
import { getDefaultDNSRecordLine } from '../src/utils/dnsRecordLine'

describe('DNS record resolution lines', () => {
  test('provides the correct provider-specific default', () => {
    expect(getDefaultDNSRecordLine('alidns')).toBe('default')
    expect(getDefaultDNSRecordLine('huaweicloud')).toBe('default_view')
    expect(getDefaultDNSRecordLine('tencentcloud')).toBe('默认')
  })

  test('normalizes provider codes', () => {
    expect(getDefaultDNSRecordLine(' TencentCloud ')).toBe('默认')
    expect(getDefaultDNSRecordLine(' ALIDNS ')).toBe('default')
    expect(getDefaultDNSRecordLine(' HUAWEICLOUD ')).toBe('default_view')
  })

  test('does not enable line selection for unsupported providers', () => {
    for (const provider of [undefined, '', 'cloudflare', 'azuredns', 'unknown'])
      expect(getDefaultDNSRecordLine(provider)).toBeUndefined()
  })
})
