import type { Ref } from 'vue'
import { ref } from 'vue'
import { getErrorMessage } from '@/lib/http'

export interface LatestRequestOptions {
  /** Shares one error message between several request tracks. */
  error?: Ref<string>
}

export interface LatestRequestHandlers<T> {
  /** Receives the response, unless a newer run started in the meantime. */
  onSuccess?: (result: T) => void
  /** Replaces the default of storing getErrorMessage(error) in `error`. */
  onError?: (error: unknown) => void
}

/**
 * Loading and error state for a request that can be re-issued while an
 * earlier call is still in flight. The wizard steps stay alive inside
 * KeepAlive, so a slow response could otherwise land after the operator
 * changed the target or left the step. Only the latest run may write results
 * or clear the loading flag.
 */
export function useLatestRequest(options: LatestRequestOptions = {}) {
  const isLoading = ref(false)
  const error = options.error ?? ref('')
  let requestID = 0

  /** Drops every in-flight response and stops the spinner. Keeps `error`. */
  function invalidate() {
    requestID++
    isLoading.value = false
  }

  /** invalidate() plus a cleared error, for starting over. */
  function reset() {
    invalidate()
    error.value = ''
  }

  async function run<T>(request: () => Promise<T>, handlers: LatestRequestHandlers<T> = {}) {
    const id = ++requestID
    isLoading.value = true
    error.value = ''
    try {
      const result = await request()
      if (id !== requestID)
        return
      handlers.onSuccess?.(result)
    }
    catch (err) {
      if (id !== requestID)
        return
      if (handlers.onError)
        handlers.onError(err)
      else
        error.value = getErrorMessage(err)
    }
    finally {
      if (id === requestID)
        isLoading.value = false
    }
  }

  return { error, invalidate, isLoading, reset, run }
}
