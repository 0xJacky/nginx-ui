import { describe, expect, test } from 'bun:test'
import { toLogFileBaseName } from '@/components/NgxConfigEditor/logFileName'

describe('site log file base name', () => {
  test('keeps ASCII site names unchanged', () => {
    expect(toLogFileBaseName('example.com')).toBe('example.com')
    expect(toLogFileBaseName('my-site_1')).toBe('my-site_1')
  })

  test('keeps non-ASCII letters so distinct names stay distinct', () => {
    expect(toLogFileBaseName('测试站点')).toBe('测试站点')
    expect(toLogFileBaseName('官网')).not.toBe(toLogFileBaseName('博客'))
    expect(toLogFileBaseName('例子.中国')).toBe('例子.中国')
    expect(toLogFileBaseName('café')).toBe('café')
  })

  test('replaces path separators, whitespace and shell characters', () => {
    expect(toLogFileBaseName('a/b')).toBe('a_b')
    expect(toLogFileBaseName('我的 站点')).toBe('我的_站点')
    expect(toLogFileBaseName('a;b$c')).toBe('a_b_c')
  })

  test('strips leading dots and falls back when nothing is left', () => {
    expect(toLogFileBaseName('..hidden')).toBe('hidden')
    expect(toLogFileBaseName('...')).toBe('site')
    expect(toLogFileBaseName('')).toBe('site')
  })
})
