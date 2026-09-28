import type { CosyError } from '@/lib/http/types'
import { T } from '@/language'
import { getErrorMessage } from '@/lib/http/error'

// Optional actionable hint the backend attaches to an issuance error. The
// message is a gettext msgid; params fill its %{name} placeholders.
export interface IssueHint {
  code: string
  message: string
  params?: Record<string, string>
}

// A terminal `status: "error"` message of the /api/domain/:name/cert stream.
export interface IssueErrorMessage {
  message?: string
  hint?: IssueHint
  // Structured cosy error behind the failure, when the backend has one.
  error?: CosyError
}

// issueHintTitle translates the hint headline, or returns '' without a hint.
export function issueHintTitle(hint?: IssueHint): string {
  return hint?.message ? T({ message: hint.message, args: hint.params }) : ''
}

// issueFailureDetail is the translated failure text of an error message: the
// cosy error translated by scope and code when present, else the raw message.
export function issueFailureDetail(r: IssueErrorMessage): string {
  const fallback = r.message ? T({ message: r.message }) : ''
  return getErrorMessage(r.error, fallback)
}
