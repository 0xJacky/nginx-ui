import { describe, expect, test } from 'bun:test'
import { createRetryableIssue } from '../src/views/certificate/components/retryableIssue'

function flush() {
  return new Promise(resolve => setTimeout(resolve, 0))
}

// Each call to attempt() returns the next scripted outcome.
function scripted(outcomes: Array<{ ok: string } | { err: string }>) {
  let calls = 0
  const attempt = () => {
    const outcome = outcomes[calls++]
    if (!outcome)
      return Promise.reject(new Error('unexpected attempt'))
    return 'ok' in outcome
      ? Promise.resolve(outcome.ok)
      : Promise.reject(new Error(outcome.err))
  }
  return {
    attempt,
    calls: () => calls,
  }
}

describe('retryable certificate issuance', () => {
  test('a retry after a failure re-runs the attempt and resolves start()', async () => {
    const { attempt, calls } = scripted([
      { err: 'dns timeout' },
      { ok: 'cert' },
    ])
    const issue = createRetryableIssue(attempt)

    const result = issue.start()
    let settled = false
    result.then(() => settled = true, () => settled = true)
    await flush()
    expect(calls()).toBe(1)
    expect(settled).toBe(false)

    issue.retry()
    expect(calls()).toBe(2)
    expect(await result).toBe('cert')
  })

  test('cancelling a failed run rejects with the last error', async () => {
    const { attempt } = scripted([
      { err: 'first' },
      { err: 'second' },
    ])
    const issue = createRetryableIssue(attempt)

    const result = issue.start()
    await flush()
    issue.retry()
    await flush()
    issue.cancel()

    await expect(result).rejects.toThrow('second')
  })

  test('retry is ignored while an attempt is in flight and after success', async () => {
    let calls = 0
    let finish: (value: string) => void = () => {}
    const issue = createRetryableIssue(() => {
      calls++
      return new Promise<string>(resolve => finish = resolve)
    })

    const result = issue.start()
    issue.retry()
    expect(calls).toBe(1)

    finish('cert')
    expect(await result).toBe('cert')
    issue.retry()
    issue.cancel()
    expect(calls).toBe(1)
  })
})
