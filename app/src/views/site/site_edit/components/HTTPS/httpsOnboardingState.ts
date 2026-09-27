import type {
  HTTPSDiagnostic,
  HTTPSEvent,
  HTTPSHint,
  HTTPSMessageArgs,
  HTTPSResult,
  HTTPSStep,
  HTTPSStepStatus,
} from '@/api/https'

// Pure state machine for the HTTPS onboarding websocket. It holds no Vue
// reactivity and no translations so it can be unit-tested in isolation;
// the composable and the card translate the English source strings.

export const HTTPS_MAIN_STEPS: readonly HTTPSStep[] = ['plan', 'stage', 'probe', 'issue', 'finalize']

const KNOWN_STEPS = new Set<HTTPSStep>([...HTTPS_MAIN_STEPS, 'rollback'])
const KNOWN_STEP_STATUSES = new Set<HTTPSStepStatus>(['running', 'success', 'warning', 'error', 'skipped'])

export type HTTPSOnboardingPhase = 'idle' | 'running' | 'success' | 'error'

export interface HTTPSStepState {
  status: HTTPSStepStatus
  message?: string
  args?: HTTPSMessageArgs
}

export interface HTTPSLogLine {
  message: string
  args?: HTTPSMessageArgs
}

export interface HTTPSOnboardingError {
  // Set when the socket dropped before `done`; the server sent no message.
  code?: 'connection_closed'
  step?: HTTPSStep
  message: string
  args?: HTTPSMessageArgs
  hint?: HTTPSHint
}

export interface HTTPSOnboardingState {
  phase: HTTPSOnboardingPhase
  steps: Partial<Record<HTTPSStep, HTTPSStepState>>
  // The step that most recently reported `running`.
  currentStep?: HTTPSStep
  logs: HTTPSLogLine[]
  diagnostics: HTTPSDiagnostic[]
  result?: HTTPSResult
  error?: HTTPSOnboardingError
}

// Client-side event emitted when the websocket errors or closes. It is a
// failure only while the run is still waiting for `done`.
export interface HTTPSSocketClosedEvent {
  type: 'socket_closed'
}

export type HTTPSOnboardingEvent = HTTPSEvent | HTTPSSocketClosedEvent

export const CONNECTION_CLOSED_MESSAGE = 'Connection closed before HTTPS setup finished'

export function createHTTPSOnboardingState(phase: HTTPSOnboardingPhase = 'idle'): HTTPSOnboardingState {
  return {
    phase,
    steps: {},
    logs: [],
    diagnostics: [],
  }
}

function isStep(value: unknown): value is HTTPSStep {
  return typeof value === 'string' && KNOWN_STEPS.has(value as HTTPSStep)
}

function withStep(state: HTTPSOnboardingState, step: HTTPSStep, next: HTTPSStepState): HTTPSOnboardingState['steps'] {
  return { ...state.steps, [step]: next }
}

// Marks the step as failed unless the server already reported a terminal status for it.
function failStep(state: HTTPSOnboardingState, step?: HTTPSStep): HTTPSOnboardingState['steps'] {
  if (!step)
    return state.steps

  const current = state.steps[step]
  if (current && current.status !== 'running')
    return state.steps

  return withStep(state, step, { ...current, status: 'error' })
}

export function reduceHTTPSEvent(state: HTTPSOnboardingState, event: HTTPSOnboardingEvent): HTTPSOnboardingState {
  // Only a running flow accepts events; anything after `done` (including the
  // server closing the socket) is ignored.
  if (state.phase !== 'running' || !event || typeof event !== 'object')
    return state

  switch (event.type) {
    case 'step': {
      if (!isStep(event.step) || !KNOWN_STEP_STATUSES.has(event.status))
        return state

      return {
        ...state,
        steps: withStep(state, event.step, {
          status: event.status,
          message: event.message,
          args: event.args,
        }),
        currentStep: event.status === 'running' ? event.step : state.currentStep,
      }
    }

    case 'log':
      if (typeof event.message !== 'string')
        return state

      return {
        ...state,
        logs: [...state.logs, { message: event.message, args: event.args }],
      }

    case 'diagnostic': {
      const { type: _type, ...diagnostic } = event

      return {
        ...state,
        diagnostics: [...state.diagnostics, diagnostic],
      }
    }

    case 'done': {
      if (event.status === 'success') {
        const { type: _type, status: _status, ...result } = event

        return {
          ...state,
          phase: 'success',
          result,
          error: undefined,
        }
      }

      const step = isStep(event.step) ? event.step : state.currentStep

      return {
        ...state,
        phase: 'error',
        steps: failStep(state, step),
        error: {
          step,
          message: event.message ?? '',
          args: event.args,
          hint: event.hint,
        },
      }
    }

    case 'socket_closed':
      return {
        ...state,
        phase: 'error',
        steps: failStep(state, state.currentStep),
        error: {
          code: 'connection_closed',
          step: state.currentStep,
          message: CONNECTION_CLOSED_MESSAGE,
        },
      }

    default:
      return state
  }
}

// Parses a raw websocket frame; returns undefined for anything that is not an event object.
export function parseHTTPSEvent(raw: unknown): HTTPSEvent | undefined {
  if (typeof raw !== 'string')
    return undefined

  try {
    const parsed = JSON.parse(raw)
    if (parsed && typeof parsed === 'object' && typeof parsed.type === 'string')
      return parsed as HTTPSEvent
  }
  catch {
    // Ignore malformed frames.
  }

  return undefined
}

const HOSTNAME_LABEL = /^(?!-)[\p{L}\p{N}-]{1,63}(?<!-)$/u

// Validates a certificate identifier: an IPv4/IPv6 literal is checked by the
// caller; this covers hostnames, optionally with a leading `*.` wildcard.
export function isValidHostname(value: string, allowWildcard = false): boolean {
  let host = value.trim().replace(/\.$/, '')
  if (allowWildcard && host.startsWith('*.'))
    host = host.slice(2)

  if (!host || host.length > 253)
    return false

  return host.split('.').every(label => HOSTNAME_LABEL.test(label))
}

/**
 * Drops diagnostics that repeat the failure hint. The backend classifies the
 * failure from the same DNS evidence it reported as a diagnostic, so both
 * would otherwise say the same thing twice.
 */
export function diagnosticsWithoutHint(diagnostics: HTTPSDiagnostic[], hintCode?: string): HTTPSDiagnostic[] {
  if (!hintCode)
    return diagnostics

  return diagnostics.filter(d => d.code !== hintCode)
}
