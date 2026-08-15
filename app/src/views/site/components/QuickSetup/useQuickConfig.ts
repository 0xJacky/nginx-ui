import type { QuickConfigRequest, QuickConfigResponse, QuickConfigType } from '@/api/template'
import template from '@/api/template'

export interface QuickConfigState {
  name: string
  type: QuickConfigType
  domains: string
  enableTLS: boolean
  redirectHTTPToHTTPS: boolean
  rpScheme: 'http' | 'https'
  rpHost: string
  rpPort: string
  rpWebSocket: boolean
  rpMaxBodySize: string
  stWebRoot: string
  stIndex: string
  stSpa: boolean
  rdTarget: string
  rdStatus: '301' | '302' | '308'
}

export function createDefaultQuickConfigState(): QuickConfigState {
  return {
    name: '',
    type: 'reverse_proxy',
    domains: '',
    enableTLS: false,
    redirectHTTPToHTTPS: true,
    rpScheme: 'http',
    rpHost: '127.0.0.1',
    rpPort: '9000',
    rpWebSocket: true,
    rpMaxBodySize: '1000m',
    stWebRoot: '',
    stIndex: 'index.html',
    stSpa: false,
    rdTarget: '',
    rdStatus: '301',
  }
}

export function useQuickConfig() {
  const state = reactive<QuickConfigState>(createDefaultQuickConfigState())
  const quickGenerating = ref(false)
  const quickNameTouched = ref(false)

  const quickDomainsList = computed(() =>
    state.domains
      .split(/[\s,]+/)
      .map(domain => domain.trim())
      .filter(Boolean),
  )

  const quickDerivedName = computed(() => quickDomainsList.value[0]?.replace(/^\*\./, '') ?? '')

  const quickFormValid = computed(() => {
    if (!state.name.trim() || quickDomainsList.value.length === 0)
      return false

    switch (state.type) {
      case 'reverse_proxy':
        return state.rpHost.trim() !== '' && state.rpPort.trim() !== ''
      case 'static':
        return state.stWebRoot.trim() !== ''
      case 'redirect':
        return state.rdTarget.trim() !== ''
      default:
        return false
    }
  })

  // Auto-suggest the config name from the first domain until the user edits it.
  watch(quickDerivedName, domain => {
    if (domain && !quickNameTouched.value)
      state.name = domain
  })

  function buildPayload(): QuickConfigRequest {
    const payload: QuickConfigRequest = {
      type: state.type,
      domains: quickDomainsList.value,
      enable_tls: state.type !== 'redirect' && state.enableTLS,
      redirect_http_to_https: state.enableTLS && state.redirectHTTPToHTTPS,
    }

    if (state.type === 'reverse_proxy') {
      payload.scheme = state.rpScheme
      payload.host = state.rpHost.trim()
      payload.port = state.rpPort.trim()
      payload.enable_websocket = state.rpWebSocket
      payload.client_max_body_size = state.rpMaxBodySize.trim()
    }
    else if (state.type === 'static') {
      payload.web_root = state.stWebRoot.trim()
      payload.index = state.stIndex.trim() || 'index.html'
      payload.spa_fallback = state.stSpa
    }
    else {
      payload.target_url = state.rdTarget.trim()
      payload.redirect_status = state.rdStatus
    }

    return payload
  }

  async function generate(): Promise<QuickConfigResponse> {
    quickGenerating.value = true
    try {
      return await template.get_quick_config(buildPayload())
    }
    finally {
      quickGenerating.value = false
    }
  }

  function applyInitial(initial: QuickConfigRequest | null | undefined) {
    if (!initial)
      return

    state.type = initial.type ?? 'reverse_proxy'
    state.domains = (initial.domains ?? []).join(' ')
    state.name = state.domains.split(/\s+/)[0] || state.name
    state.enableTLS = !!initial.enable_tls
    state.redirectHTTPToHTTPS = !!initial.redirect_http_to_https

    state.rpScheme = initial.scheme ?? 'http'
    state.rpHost = initial.host ?? ''
    state.rpPort = initial.port ?? ''
    state.rpWebSocket = !!initial.enable_websocket
    state.rpMaxBodySize = initial.client_max_body_size ?? '1000m'

    state.stWebRoot = initial.web_root ?? ''
    state.stIndex = initial.index ?? 'index.html'
    state.stSpa = !!initial.spa_fallback

    state.rdTarget = initial.target_url ?? ''
    state.rdStatus = (initial.redirect_status as '301' | '302' | '308') ?? '301'
  }

  function reset() {
    Object.assign(state, createDefaultQuickConfigState())
    quickNameTouched.value = false
  }

  return {
    state,
    quickGenerating,
    quickNameTouched,
    quickDomainsList,
    quickDerivedName,
    quickFormValid,
    buildPayload,
    generate,
    applyInitial,
    reset,
  }
}

export type QuickConfig = ReturnType<typeof useQuickConfig>
