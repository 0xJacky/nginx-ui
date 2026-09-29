import { describe, expect, test } from 'bun:test'
import { editorSiteLogContext, siteLogContext } from '../src/plugin/siteLogContext'

describe('siteLogContext', () => {
  test('fills what a site list row lacks with empty values', () => {
    expect(siteLogContext({ siteName: 'a.conf' })).toEqual({
      accessLogPath: '',
      accessLogInherited: false,
      errorLogPath: '',
      errorLogInherited: false,
      siteName: 'a.conf',
    })
  })

  test('keeps paths and flags of a row', () => {
    expect(siteLogContext({
      siteName: 'a.conf',
      accessLogPath: '/var/log/nginx/a.log',
      errorLogPath: '/var/log/nginx/error.log',
      errorLogInherited: true,
    })).toEqual({
      accessLogPath: '/var/log/nginx/a.log',
      accessLogInherited: false,
      errorLogPath: '/var/log/nginx/error.log',
      errorLogInherited: true,
      siteName: 'a.conf',
    })
  })

  test('a flag never stands without a path', () => {
    const context = siteLogContext({ accessLogInherited: true, errorLogInherited: true })
    expect(context.accessLogInherited).toBe(false)
    expect(context.errorLogInherited).toBe(false)
  })
})

describe('editorSiteLogContext', () => {
  const defaults = { access: '/var/log/nginx/access.log', error: '/var/log/nginx/error.log' }

  test('an own directive wins and is not inherited', () => {
    const context = editorSiteLogContext('a.conf', { access: '/var/log/nginx/a.log' }, defaults, true)
    expect(context.accessLogPath).toBe('/var/log/nginx/a.log')
    expect(context.accessLogInherited).toBe(false)
    expect(context.errorLogPath).toBe('/var/log/nginx/error.log')
    expect(context.errorLogInherited).toBe(true)
  })

  test('without directives the defaults are inherited', () => {
    const context = editorSiteLogContext('a.conf', {}, defaults, true)
    expect(context.accessLogPath).toBe(defaults.access)
    expect(context.accessLogInherited).toBe(true)
  })

  test('a missing default leaves the path empty', () => {
    const context = editorSiteLogContext('a.conf', {}, {}, true)
    expect(context.accessLogPath).toBe('')
    expect(context.accessLogInherited).toBe(false)
  })

  test('a context that does not inherit reports own paths only', () => {
    const context = editorSiteLogContext('a.conf', { access: '/var/log/nginx/a.log' }, defaults, false)
    expect(context.errorLogPath).toBe('')
    expect(context.errorLogInherited).toBe(false)
    expect(context.accessLogPath).toBe('/var/log/nginx/a.log')
  })
})
