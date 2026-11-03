export interface RetryableIssue<T> {
  // Starts a new issuance. The returned promise resolves on the first
  // successful attempt and rejects only when the run is cancelled.
  start: () => Promise<T>
  // Re-runs the attempt for the pending run; a no-op when nothing is pending
  // or an attempt is still in flight.
  retry: () => void
  // Settles the pending run as failed, with the last attempt's error when
  // there is one. A no-op when nothing is pending.
  cancel: () => void
}

interface PendingRun<T> {
  resolve: (value: T) => void
  reject: (reason: unknown) => void
  lastError?: unknown
  isAttempting: boolean
}

// A failed attempt leaves the run pending so the user can retry inside the
// same dialog; only a success or an explicit cancel (closing the dialog)
// settles the promise handed out by start().
export function createRetryableIssue<T>(attempt: () => Promise<T>): RetryableIssue<T> {
  let pending: PendingRun<T> | undefined

  function run(current: PendingRun<T>) {
    current.isAttempting = true
    attempt()
      .then(value => {
        if (pending !== current)
          return
        pending = undefined
        current.resolve(value)
      })
      .catch((error: unknown) => {
        current.lastError = error
      })
      .finally(() => {
        current.isAttempting = false
      })
  }

  function cancel() {
    const current = pending
    if (!current)
      return
    pending = undefined
    current.reject(current.lastError ?? new Error('Certificate issuance cancelled'))
  }

  function start() {
    cancel()
    return new Promise<T>((resolve, reject) => {
      const current: PendingRun<T> = { resolve, reject, isAttempting: false }
      pending = current
      run(current)
    })
  }

  function retry() {
    if (pending && !pending.isAttempting)
      run(pending)
  }

  return { start, retry, cancel }
}
