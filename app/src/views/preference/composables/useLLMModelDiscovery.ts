import type { Ref } from 'vue'
import type { LLMModel } from '@/api/llm'
import type { OpenaiSettings } from '@/api/settings'
import type { CosyError } from '@/lib/http/types'
import { debounce } from 'lodash'
import llm from '@/api/llm'
import { translateError } from '@/lib/http/error'
import { normalizeHttpError } from '@/lib/http/normalizeError'

export type LLMModelDiscoveryStatus
  = | 'idle'
    | 'missing_token'
    | 'loading'
    | 'success'
    | 'unauthorized'
    | 'unsupported'
    | 'timeout'
    | 'error'

// Codes of the llm error scope, see internal/llm/errors.go.
const statusByErrorCode: Record<number, LLMModelDiscoveryStatus> = {
  40101: 'unauthorized',
  40102: 'unsupported',
  40103: 'timeout',
}

// Providers that can run without an API token, such as a local Ollama.
const TOKENLESS_PROVIDERS = new Set(['custom'])

const REFETCH_DELAY = 800

// Lists the models of the provider on the settings form. The list is fetched
// when the form loads and again whenever the connection fields change, so the
// result doubles as a connection check.
export function useLLMModelDiscovery(openai: Ref<OpenaiSettings | undefined>, isLoaded: Ref<boolean>) {
  const status = ref<LLMModelDiscoveryStatus>('idle')
  const models = ref<LLMModel[]>()
  const errorMessage = ref('')

  let controller: AbortController | undefined

  const connection = computed(() => {
    const value = openai.value
    if (!value || !isLoaded.value)
      return undefined

    return {
      provider: value.provider,
      base_url: value.base_url?.trim() ?? '',
      token: value.token?.trim() ?? '',
      proxy: value.proxy?.trim() ?? '',
      api_type: value.api_type,
    }
  })

  async function refresh() {
    const request = connection.value
    if (!request)
      return

    controller?.abort()

    if (!request.token && !TOKENLESS_PROVIDERS.has(request.provider)) {
      status.value = 'missing_token'
      models.value = undefined
      errorMessage.value = ''
      return
    }

    const current = new AbortController()
    controller = current
    status.value = 'loading'
    errorMessage.value = ''

    try {
      const response = await llm.list_models(request, current.signal)
      if (current.signal.aborted)
        return

      models.value = response.models ?? []
      status.value = 'success'
    }
    catch (err) {
      if (current.signal.aborted)
        return

      const cosyError = normalizeHttpError(err) as CosyError
      models.value = undefined
      status.value = (cosyError.scope === 'llm' && statusByErrorCode[Number(cosyError.code)]) || 'error'
      // A 406 means the base URL or proxy is not a valid URL
      errorMessage.value = cosyError.httpStatus === 406
        ? $gettext('Check the format of the API Base Url and API Proxy.')
        : await translateError(cosyError)
    }
  }

  const refreshLater = debounce(refresh, REFETCH_DELAY)

  watch(connection, (value, previous) => {
    if (!value)
      return

    // The first load of the settings fetches right away, edits wait for the
    // user to pause typing.
    if (!previous) {
      void refresh()
      return
    }

    if (JSON.stringify(value) !== JSON.stringify(previous))
      refreshLater()
  }, { immediate: true })

  onScopeDispose(() => {
    refreshLater.cancel()
    controller?.abort()
  })

  return {
    status,
    models,
    errorMessage,
    refresh,
  }
}
