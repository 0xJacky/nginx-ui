import { describe, expect, test } from 'bun:test'

// Fake catalog: translates known msgids to a marked form and interpolates
// %{name} placeholders like vue3-gettext does.
const catalog: Record<string, string> = {
  'HTTP-01 challenge route check failed for {0}: {1}': '[zh] HTTP-01 route check failed for {0}: {1}',
  'Certificate issuance failed for %{domain}; check the log.': '[zh] Issuance failed for %{domain}',
}
function fakeGettext(message: string, args?: Record<string, unknown>) {
  const translated = catalog[message] ?? message
  return translated.replace(/%\{(\w+)\}/g, (_, key: string) => String(args?.[key] ?? `%{${key}}`))
}
Object.assign(globalThis, { $gettext: fakeGettext })

const { issueFailureDetail, issueHintTitle } = await import('../src/views/site/site_edit/components/Cert/issueFailure')

describe('certificate issuance failure text', () => {
  test('translates the structured cosy error by its template and params', () => {
    const detail = issueFailureDetail({
      message: 'HTTP-01 challenge route check failed for example.com: unexpected status 404',
      error: {
        scope: 'cert',
        code: 50058,
        message: 'HTTP-01 challenge route check failed for {0}: {1}',
        params: ['example.com', 'unexpected status 404'],
      },
    })
    expect(detail).toBe('[zh] HTTP-01 route check failed for example.com: unexpected status 404')
  })

  test('falls back to the raw message without a structured error', () => {
    expect(issueFailureDetail({ message: 'obtain cert error: boom' })).toBe('obtain cert error: boom')
  })

  test('returns an empty detail when the message is empty', () => {
    expect(issueFailureDetail({})).toBe('')
  })

  test('translates the hint title with its params', () => {
    expect(issueHintTitle({
      code: 'challenge_not_served',
      message: 'Certificate issuance failed for %{domain}; check the log.',
      params: { domain: 'example.com' },
    })).toBe('[zh] Issuance failed for example.com')
  })

  test('has no hint title without a hint', () => {
    expect(issueHintTitle()).toBe('')
    expect(issueHintTitle({ code: 'x', message: '' })).toBe('')
  })
})
