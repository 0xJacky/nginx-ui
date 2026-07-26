interface RestartErrorResponse {
  status?: number
  data?: {
    message?: string
  }
}

interface RestartError {
  isAxiosError?: boolean
  message?: string
  response?: RestartErrorResponse
}

export function isRestartOutcomeUnknown(error: unknown): boolean {
  const restartError = error as RestartError
  const status = restartError?.response?.status
  if (status !== undefined)
    return status === 408 || status >= 500

  return restartError?.isAxiosError === true
}

export function getRestartErrorMessage(error: unknown): string | undefined {
  const restartError = error as RestartError
  return restartError?.response?.data?.message || restartError?.message
}

export function shouldShowRestartError(error: unknown): boolean {
  const restartError = error as RestartError
  return restartError?.isAxiosError === true || restartError?.response?.status !== undefined
}
