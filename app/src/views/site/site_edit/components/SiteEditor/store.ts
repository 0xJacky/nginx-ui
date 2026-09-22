import type { CertificateInfo } from '@/api/cert'
import type { NgxConfig, NgxServer } from '@/api/ngx'
import type { Site } from '@/api/site'
import type { CosyError } from '@/lib/http/types'
import type { CheckedType } from '@/types'
import config from '@/api/config'
import ngx from '@/api/ngx'
import site from '@/api/site'
import { useNgxConfigStore } from '@/components/NgxConfigEditor'
import { translateError } from '@/lib/http/error'
import { isIPAddress, splitCertificateIdentifiers } from '@/utils/certificate'

interface SaveOptions {
  omitIncompleteTLSServers?: boolean
  syncResponse?: boolean
}

interface TLSServerIssue {
  serverIndex: number
  missingCertificate: boolean
  missingCertificateKey: boolean
}

function cloneNgxConfig(config: NgxConfig): NgxConfig {
  return JSON.parse(JSON.stringify(config))
}

function hasSSLListen(server?: NgxServer) {
  return server?.directives?.some(v => v.directive === 'listen' && v.params?.includes('ssl')) ?? false
}

function hasDirectiveWithValue(server: NgxServer | undefined, directive: string) {
  return server?.directives?.some(v => v.directive === directive && v.params?.trim()) ?? false
}

export const useSiteEditorStore = defineStore('siteEditor', () => {
  const advanceMode = ref(false)
  const parseErrorStatus = ref(false)
  const parseErrorMessage = ref('')
  const data = ref({}) as Ref<Site>
  const loading = ref(true)
  const saving = ref(false)
  const autoCert = ref(false)
  const certInfoMap = ref({}) as Ref<Record<number, CertificateInfo[]>>
  const filename = ref('')
  const filepath = ref('')
  const issuingCert = ref(false)
  const dnsLinked = ref(false) // Track if DNS is linked
  const linkedDNSName = ref('') // Store linked DNS name

  const ngxConfigStore = useNgxConfigStore()
  const { ngxConfig, configText, curServerIdx, curServer, curServerDirectives, curDirectivesMap } = storeToRefs(ngxConfigStore)

  const name = computed({
    get() {
      return ngxConfig.value.name
    },
    set(v) {
      ngxConfig.value.name = v
    },
  })

  const hasServers = computed(() => {
    return ngxConfig.value.servers && ngxConfig.value.servers.length > 0
  })

  // This store is a singleton, so opening another site reuses the state left
  // behind by the previous one. Every init() run takes a ticket and only the
  // newest one may write, otherwise a slow response for the site the operator
  // just left would land on top of the one now on screen.
  let loadSeq = 0

  function reset() {
    advanceMode.value = false
    parseErrorStatus.value = false
    parseErrorMessage.value = ''
    data.value = {} as Site
    autoCert.value = false
    certInfoMap.value = {}
    filename.value = ''
    filepath.value = ''
    issuingCert.value = false
    dnsLinked.value = false
    linkedDNSName.value = ''
    ngxConfigStore.reset()
  }

  async function init(_name: string) {
    const seq = ++loadSeq
    loading.value = true
    reset()
    await nextTick()
    if (seq !== loadSeq)
      return

    name.value = _name

    if (name.value) {
      try {
        const r = await site.getItem(encodeURIComponent(name.value))
        if (seq !== loadSeq)
          return
        await handleResponse(r)
      }
      catch (error) {
        if (seq !== loadSeq)
          return
        await handleParseError(error as CosyError)
      }
    }

    if (seq === loadSeq)
      loading.value = false
  }

  function getTLSServerIssues(config: NgxConfig = ngxConfig.value): TLSServerIssue[] {
    return (config.servers ?? []).reduce<TLSServerIssue[]>((issues, server, serverIndex) => {
      if (!hasSSLListen(server))
        return issues

      const missingCertificate = !hasDirectiveWithValue(server, 'ssl_certificate')
      const missingCertificateKey = !hasDirectiveWithValue(server, 'ssl_certificate_key')

      if (missingCertificate || missingCertificateKey) {
        issues.push({
          serverIndex,
          missingCertificate,
          missingCertificateKey,
        })
      }

      return issues
    }, [])
  }

  function getConfigWithoutIncompleteTLSServers(config: NgxConfig = ngxConfig.value) {
    const clonedConfig = cloneNgxConfig(config)

    const servers = clonedConfig.servers?.filter(server => {
      if (!hasSSLListen(server))
        return true

      return hasDirectiveWithValue(server, 'ssl_certificate')
        && hasDirectiveWithValue(server, 'ssl_certificate_key')
    }) ?? []

    if (servers.length === 0)
      return clonedConfig

    clonedConfig.servers = servers

    return clonedConfig
  }

  async function buildConfig(config: NgxConfig = ngxConfig.value, syncConfigText = true) {
    return ngx.build_config(config).then(r => {
      if (syncConfigText)
        configText.value = r.content

      return r.content
    })
  }

  async function save(options: SaveOptions = {}) {
    saving.value = true

    try {
      let content = configText.value

      if (!advanceMode.value) {
        const configForSave = options.omitIncompleteTLSServers
          ? getConfigWithoutIncompleteTLSServers()
          : ngxConfig.value

        content = await buildConfig(configForSave, !options.omitIncompleteTLSServers)
      }

      if (data.value.sync_node_ids === null) {
        data.value.sync_node_ids = []
      }

      // @ts-expect-error allow comparing with empty string for legacy data
      if (data.value.namespace_id === '') {
        data.value.namespace_id = 0
      }

      const response = await site.updateItem(encodeURIComponent(name.value), {
        content,
        description: data.value.description,
        overwrite: true,
        namespace_id: data.value.namespace_id,
        sync_node_ids: data.value.sync_node_ids,
        post_action: 'reload_nginx',
        dns_domain_id: data.value.dns_domain_id,
        dns_records: data.value.dns_records,
        dns_record_id: data.value.dns_record_id,
        dns_record_name: data.value.dns_record_name,
        dns_record_type: data.value.dns_record_type,
      })

      if (options.syncResponse !== false)
        await handleResponse(response)

      return response
    }
    catch (error) {
      await handleParseError(error as CosyError)
      throw error
    }
    finally {
      saving.value = false
    }
  }

  async function handleParseError(e: CosyError) {
    console.error(e)
    parseErrorStatus.value = true
    parseErrorMessage.value = await translateError(e)
    const target = name.value
    config.getItem(`sites-available/${encodeURIComponent(target)}`).then(r => {
      // Another site may have taken over the store while this was in flight.
      if (name.value === target)
        configText.value = r.content
    })
  }

  async function handleResponse(r: Site) {
    if (r.advanced)
      advanceMode.value = true

    parseErrorStatus.value = false
    parseErrorMessage.value = ''
    filename.value = r.name
    filepath.value = r.filepath
    configText.value = r.config
    autoCert.value = r.auto_cert
    data.value = r
    autoCert.value = r.auto_cert
    certInfoMap.value = r.cert_info || {}

    if (r.tokenized)
      ngxConfigStore.setNgxConfig(r.tokenized)
  }

  async function handleModeChange(advanced: CheckedType) {
    loading.value = true

    try {
      await site.advance_mode(encodeURIComponent(name.value), { advanced: advanced as boolean })
      advanceMode.value = advanced as boolean
      if (advanced) {
        await buildConfig()
      }
      else {
        const r = await site.getItem(encodeURIComponent(name.value))
        await handleResponse(r)
      }
    }
    // eslint-disable-next-line ts/no-explicit-any
    catch (e: any) {
      handleParseError(e)
    }

    loading.value = false
  }

  const curSupportSSL = computed(() => {
    if (curDirectivesMap.value.listen) {
      for (const v of curDirectivesMap.value.listen) {
        if (v?.params.indexOf('ssl') > 0)
          return true
      }
    }

    return false
  })

  const isDefaultServer = computed(() => {
    if (curDirectivesMap.value.listen) {
      for (const v of curDirectivesMap.value.listen) {
        const params = v?.params || ''
        if (params.includes('443') && params.includes('ssl') && params.includes('default_server'))
          return true
      }
    }

    return false
  })

  const rawServerNames = computed(() => curDirectivesMap.value.server_name
    ?.flatMap(directive => directive.params?.split(/\s+/) ?? [])
    .map(value => value.trim())
    .filter(Boolean) ?? [])

  const certificateIdentifiers = computed(() => splitCertificateIdentifiers(rawServerNames.value))

  const hasWildcardServerName = computed(() => rawServerNames.value.includes('_'))

  const hasExplicitIpAddress = computed(() => certificateIdentifiers.value.some(isIPAddress))

  const isIpCertificate = computed(() => {
    return hasExplicitIpAddress.value
  })

  const needsManualIpInput = computed(() => {
    return (isDefaultServer.value || hasWildcardServerName.value)
      && certificateIdentifiers.value.length === 0
  })

  return {
    name,
    advanceMode,
    parseErrorStatus,
    parseErrorMessage,
    data,
    loading,
    saving,
    autoCert,
    certInfoMap,
    ngxConfig,
    curServerIdx,
    curServer,
    curServerDirectives,
    curDirectivesMap,
    filename,
    filepath,
    configText,
    issuingCert,
    curSupportSSL,
    isDefaultServer,
    hasWildcardServerName,
    hasExplicitIpAddress,
    certificateIdentifiers,
    isIpCertificate,
    needsManualIpInput,
    hasServers,
    getTLSServerIssues,
    getConfigWithoutIncompleteTLSServers,
    buildConfig,
    dnsLinked,
    linkedDNSName,
    init,
    reset,
    save,
    handleModeChange,
  }
})
