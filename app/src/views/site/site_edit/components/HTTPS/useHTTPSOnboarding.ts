import type { MaybeRefOrGetter } from 'vue'
import type { IssueErrorMessage } from '../Cert/issueFailure'
import type { HTTPSOnboardingEvent, HTTPSOnboardingState } from './httpsOnboardingState'
import type { HTTPSCheck, HTTPSCheckRequest, HTTPSDoneErrorEvent, HTTPSMessageArgs, HTTPSRequest } from '@/api/https'
import https, { httpsWebSocketPath } from '@/api/https'
import use2FAModal from '@/components/TwoFA/use2FAModal'
import { useWebSocket } from '@/lib/websocket'
import { issueFailureDetail } from '../Cert/issueFailure'
import { createHTTPSOnboardingState, parseHTTPSEvent, reduceHTTPSEvent } from './httpsOnboardingState'

export * from './httpsOnboardingState'

/**
 * A DNS-01 certificate the main node issues for the selected node before the
 * run on the node installs it.
 */
export interface HTTPSDelegatedIssue {
  nodeId: number
  // The options of the issuance; the domains come from the request.
  payload: Record<string, unknown>
}

// A message of the issuance stream of the main node.
interface DelegatedIssueMessage extends IssueErrorMessage {
  status?: string
  args?: HTTPSMessageArgs
  remote_certificate_id?: number
}

/**
 * Drives the backend-orchestrated HTTPS enablement of one site over
 * `/api/sites/:name/https`. Must be called from a component setup because
 * it opens the 2FA modal and closes the socket on scope dispose.
 */
export function useHTTPSOnboarding(configName: MaybeRefOrGetter<string>) {
  const otpModal = use2FAModal()

  const state = shallowRef<HTTPSOnboardingState>(createHTTPSOnboardingState())
  const lastRequest = shallowRef<HTTPSRequest>()
  const lastDelegated = shallowRef<HTTPSDelegatedIssue>()

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

  // The certificate the main node issued for the last delegated run, reused
  // by a retry with the same options so a failed run on the node does not
  // issue it again.
  let delegatedIssued: { key: string, certificateId: number } | undefined

  async function start(request: HTTPSRequest, delegated?: HTTPSDelegatedIssue) {
    if (running.value)
      return

    const id = ++runId
    closeSocket()
    lastRequest.value = { ...request, domains: [...request.domains] }
    lastDelegated.value = delegated
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

    let runRequest = lastRequest.value
    if (delegated) {
      const certificateId = await issueOnMainNode(id, delegated, secureSessionId)
      if (!certificateId || id !== runId)
        return
      runRequest = { ...runRequest, certificate_id: certificateId }
    }

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
        current.send(JSON.stringify(runRequest))
    }

    current.onmessage = (m: MessageEvent) => {
      const event = parseHTTPSEvent(m.data)
      if (event)
        dispatch(id, event)
    }

    current.onerror = () => dispatch(id, { type: 'socket_closed' })
    current.onclose = () => dispatch(id, { type: 'socket_closed' })
  }

  // Issues the certificate on the main node, which sends it to the node, and
  // resolves with the record the node keeps it in, or 0 after a failure.
  function issueOnMainNode(id: number, delegated: HTTPSDelegatedIssue, secureSessionId: string) {
    const domains = lastRequest.value?.domains ?? []
    const key = JSON.stringify({ ...delegated, domains })
    dispatch(id, { type: 'step', step: 'delegate', status: 'running' })
    if (delegatedIssued?.key === key) {
      dispatch(id, { type: 'step', step: 'delegate', status: 'success' })
      return Promise.resolve(delegatedIssued.certificateId)
    }

    return new Promise<number>(resolve => {
      const { ws } = useWebSocket(
        `/api/nodes/${delegated.nodeId}/domain/${encodeURIComponent(toValue(configName))}/cert`,
        false,
        undefined,
        { 'X-Secure-Session-ID': secureSessionId },
        true,
      )
      const current = ws.value
      let settled = false
      function settle(certificateId: number, failure?: HTTPSDoneErrorEvent) {
        if (settled)
          return
        settled = true
        if (socket === current)
          socket = undefined
        if (failure)
          dispatch(id, failure)
        else
          dispatch(id, { type: 'step', step: 'delegate', status: 'success' })
        resolve(certificateId)
      }

      if (!current) {
        dispatch(id, { type: 'socket_closed' })
        resolve(0)
        return
      }
      socket = current

      current.onopen = () => {
        if (id === runId)
          current.send(JSON.stringify({ ...delegated.payload, server_name: domains }))
      }

      current.onmessage = (m: MessageEvent) => {
        let r: DelegatedIssueMessage
        try {
          r = JSON.parse(m.data)
        }
        catch {
          return
        }
        if (r.status === 'error') {
          settle(0, {
            type: 'done',
            status: 'error',
            step: 'delegate',
            message: issueFailureDetail(r),
            hint: r.hint,
          })
          return
        }
        if (r.message)
          dispatch(id, { type: 'log', message: r.message, args: r.args })
        if (r.status !== 'success')
          return
        if (!r.remote_certificate_id) {
          settle(0, { type: 'done', status: 'error', step: 'delegate', message: $gettext('The node did not confirm that it received the certificate. Update Nginx UI on the node and try again.') })
          return
        }
        delegatedIssued = { key, certificateId: r.remote_certificate_id }
        settle(r.remote_certificate_id)
      }

      const closed = () => {
        if (settled)
          return
        settled = true
        dispatch(id, { type: 'socket_closed' })
        resolve(0)
      }
      current.onerror = closed
      current.onclose = closed
    })
  }

  async function retry() {
    if (lastRequest.value)
      await start(lastRequest.value, lastDelegated.value)
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
