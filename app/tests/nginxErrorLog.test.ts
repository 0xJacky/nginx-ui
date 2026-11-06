import { describe, expect, test } from 'bun:test'
import {
  ERROR_LOG_LEVELS,
  errorLevelColor,
  errorLevelParam,
  errorLogContext,
  errorLogRequest,
} from '../src/views/nginx_log/structured/errorLog'

describe('errorLevelParam', () => {
  test('joins the selected severities for the search request', () => {
    expect(errorLevelParam(['error', 'crit'])).toBe('error,crit')
  })

  test('drops duplicates and values nginx does not write', () => {
    expect(errorLevelParam(['Error', 'error', 'fatal', ' warn '])).toBe('error,warn')
  })

  test('sends nothing when no severity is selected', () => {
    expect(errorLevelParam([])).toBeUndefined()
    expect(errorLevelParam(['fatal'])).toBeUndefined()
  })
})

describe('errorLevelColor', () => {
  test('gives every severity its own color', () => {
    const colors = ERROR_LOG_LEVELS.map(level => errorLevelColor(level))
    expect(new Set(colors).size).toBe(ERROR_LOG_LEVELS.length)
  })

  test('falls back to the neutral color', () => {
    expect(errorLevelColor('CRIT')).toBe(errorLevelColor('crit'))
    expect(errorLevelColor(undefined)).toBe('default')
    expect(errorLevelColor('fatal')).toBe('default')
  })
})

describe('errorLogRequest', () => {
  test('shows the method and path of the request', () => {
    expect(errorLogRequest({ method: 'GET', path: '/api/x', request: 'GET /api/x HTTP/1.1' })).toBe('GET /api/x')
  })

  test('falls back to the request line, then to nothing', () => {
    expect(errorLogRequest({ request: 'PRI * HTTP/2.0' })).toBe('PRI * HTTP/2.0')
    expect(errorLogRequest({})).toBe('')
  })
})

describe('errorLogContext', () => {
  test('lists only the parts nginx logged, in a fixed order', () => {
    expect(errorLogContext({ host: 'example.com', server: 'example.com', upstream: '' })).toEqual([
      { key: 'server', value: 'example.com' },
      { key: 'host', value: 'example.com' },
    ])
  })
})
