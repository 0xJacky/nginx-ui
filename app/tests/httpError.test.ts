import { AxiosHeaders } from 'axios'
import { describe, expect, test } from 'bun:test'
import { normalizeHttpError, shouldLogoutOnAuthFailure } from '../src/lib/http/normalizeError'

Object.assign(globalThis, { $gettext: (message: string) => message })
const { registerError, translateError } = await import('../src/lib/http/error')

describe('HTTP error normalization', () => {
  test('only current-session 403 responses log out, not old requests or 5xx failures', () => {
    const config = { headers: new AxiosHeaders({ Authorization: 'current-token' }) }
    expect(shouldLogoutOnAuthFailure({ config, response: { status: 403 } }, 'current-token')).toBe(true)
    expect(shouldLogoutOnAuthFailure({ config, response: { status: 403 } }, 'new-token')).toBe(false)
    expect(shouldLogoutOnAuthFailure({ config, response: { status: 503 } }, 'current-token')).toBe(false)
    expect(shouldLogoutOnAuthFailure({ response: { status: 403 } }, 'current-token')).toBe(false)
    expect(shouldLogoutOnAuthFailure({ config: { ...config, skipAuthRedirect: true }, response: { status: 403 } }, 'current-token')).toBe(false)
  })
  test('HTML errors show the status instead of the response body', async () => {
    const error = normalizeHttpError({ response: { status: 503, data: '<html>private proxy details</html>' } })
    expect(await translateError(error)).toBe('Server error (HTTP 503)')
    expect(JSON.stringify(error)).not.toContain('private proxy details')
  })

  test('empty and malformed response bodies retain their status', async () => {
    for (const data of [null, '', false, [], { error: 'failure' }, { message: 123 }]) {
      const error = normalizeHttpError({ response: { status: 502, data } })
      expect(await translateError(error)).toBe('Server error (HTTP 502)')
    }
  })

  test('preserves structured error codes, parameters and translation', async () => {
    registerError('test-scope', { 12: () => 'Failure for {0}' })
    const error = normalizeHttpError({ response: { status: 500, data: { scope: 'test-scope', code: 12, message: 'fallback', params: ['config'] } } })
    expect(error.code).toBe(12)
    expect(error.httpStatus).toBe(500)
    expect(await translateError(error)).toBe('Failure for config')
  })

  test('preserves numeric login codes, validation details and numeric parameters', async () => {
    const validation = { name: 'required' }
    const error = normalizeHttpError({ response: { status: 500, data: { code: 4043, errors: validation, params: [123] } } })
    expect(error.code).toBe(4043)
    expect(error).toMatchObject({ errors: validation, params: ['123'] })
  })

  test('preserves explicit messages and deliberate empty messages', async () => {
    for (const message of ['Database unavailable', '']) {
      const error = normalizeHttpError({ response: { status: 500, data: { message } } })
      expect(await translateError(error)).toBe(message)
    }
  })

  test('preserves transport errors and discards untrusted metadata', async () => {
    const error = normalizeHttpError({ code: 'ERR_NETWORK', message: 'Network Error', config: { headers: { Authorization: 'secret' } } })
    expect(await translateError(error)).toBe('Network Error')
    expect(JSON.stringify(error)).not.toContain('secret')
    expect(normalizeHttpError(null).message).toBe('Network error')
  })
})
