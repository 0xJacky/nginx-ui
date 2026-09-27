import type { MaybeRefOrGetter } from 'vue'
import type { HTTPSOnboardingEvent, HTTPSOnboardingState } from './httpsOnboardingState'
import type { HTTPSCheck, HTTPSCheckRequest, HTTPSRequest } from '@/api/https'
import https, { httpsWebSocketPath } from '@/api/https'
import use2FAModal from '@/components/TwoFA/use2FAModal'
import { useWebSocket } from '@/lib/websocket'
import { createHTTPSOnboardingState, parseHTTPSEvent, reduceHTTPSEvent } from './httpsOnboardingState'

export * from './httpsOnboardingState'

/**
 * Drives the backend-orchestrated HTTPS enablement of one site over
 * `/api/sites/:name/https`. Must be called from a component setup because
 * it opens the 2FA modal and closes the socket on scope dispose.
 */
export function useHTTPSOnboarding(configName: MaybeRefOrGetter<string>) {
  const otpModal = use2FAModal()

  const state = shallowRef<HTTPSOnboardingState>(createHTTPSOnboardingState())
  const lastRequest = shallowRef<HTTPSRequest>()

  const checks = ref<HTTPSCheck[]>([])
  const checking = ref(false)
  const checkError = ref<unknown>()

  let socket: WebSocket | undefined
  // Incremented by every start/reset so handlers of an abandoned socket become no-ops.
  let runId = 0

  const phase = computed(() => state.value.phase)
  const running = computed(() => state.value.phase === 'running')
  const steps = computed(() => state.value.steps)
  const currentStep = computed(() => state.value.currentStep)
  const logs = computed(() => state.value.logs)
  const diagnostics = computed(() => state.value.diagnostics)
  const result = computed(() => state.value.result)
  const error = computed(() => state.value.error)
  const hint = computed(() => state.value.error?.hint)
  const canRetry = computed(() => state.value.phase === 'error' && !!lastRequest.value)

  function dispatch(id: number, event: HTTPSOnboardingEvent) {
    if (id !== runId)
      return

    state.value = reduceHTTPSEvent(state.value, event)
  }

  function closeSocket() {
    const current = socket
    socket = undefined
    if (current && current.readyState !== WebSocket.CLOSED && current.readyState !== WebSocket.CLOSING)
      current.close()
  }

  async function start(request: HTTPSRequest) {
    if (running.value)
      return

    const id = ++runId
    closeSocket()
    lastRequest.value = { ...request, domains: [...request.domains] }
    state.value = createHTTPSOnboardingState('running')

    let secureSessionId: string
    try {
      secureSessionId = await otpModal.open()
    }
    catch {
      // 2FA cancelled: back to the form without recording a failure.
      if (id === runId)
        state.value = createHTTPSOnboardingState()
      return
    }

    if (id !== runId)
      return

    const { ws } = useWebSocket(httpsWebSocketPath(toValue(configName)), false, undefined, {
      'X-Secure-Session-ID': secureSessionId,
    })
    const current = ws.value
    if (!current) {
      dispatch(id, { type: 'socket_closed' })
      return
    }
    socket = current

    current.onopen = () => {
      if (id === runId)
        current.send(JSON.stringify(lastRequest.value))
    }

    current.onmessage = (m: MessageEvent) => {
      const event = parseHTTPSEvent(m.data)
      if (event)
        dispatch(id, event)
    }

    current.onerror = () => dispatch(id, { type: 'socket_closed' })
    current.onclose = () => dispatch(id, { type: 'socket_closed' })
  }

  async function retry() {
    if (lastRequest.value)
      await start(lastRequest.value)
  }

  function reset() {
    runId++
    closeSocket()
    state.value = createHTTPSOnboardingState()
  }

  async function check(request: HTTPSCheckRequest) {
    checking.value = true
    checkError.value = undefined
    try {
      const r = await https.check(toValue(configName), request)
      checks.value = r?.checks ?? []
    }
    catch (e) {
      checks.value = []
      checkError.value = e
    }
    finally {
      checking.value = false
    }
  }

  function clearChecks() {
    checks.value = []
    checkError.value = undefined
  }

  onScopeDispose(() => {
    runId++
    closeSocket()
  })

  return {
    state: readonly(state),
    phase,
    running,
    steps,
    currentStep,
    logs,
    diagnostics,
    result,
    error,
    hint,
    canRetry,
    lastRequest: readonly(lastRequest),
    checks: readonly(checks),
    checking: readonly(checking),
    checkError: readonly(checkError),
    start,
    retry,
    reset,
    check,
    clearChecks,
  }
}

export type HTTPSOnboarding = ReturnType<typeof useHTTPSOnboarding>
