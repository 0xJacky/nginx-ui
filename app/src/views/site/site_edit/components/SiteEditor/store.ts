import type { CertificateInfo } from '@/api/cert'
import type { NgxConfig, NgxServer } from '@/api/ngx'
import type { Site } from '@/api/site'
import type { CosyError } from '@/lib/http/types'
import type { CheckedType } from '@/types'
import config from '@/api/config'
import ngx from '@/api/ngx'
import site from '@/api/site'
import { useNgxConfigStore } from '@/components/NgxConfigEditor'
import { useGlobalApp } from '@/composables/useGlobalApp'
import { translateError } from '@/lib/http/error'

interface SaveOptions {
  omitIncompleteTLSServers?: boolean
  skipTLSValidation?: boolean
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
  const { message } = useGlobalApp()
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

  async function init(_name: string) {
    loading.value = true
    await nextTick()
    name.value = _name

    if (name.value) {
      try {
        const r = await site.getItem(encodeURIComponent(name.value))
        handleResponse(r)
      }
      catch (error) {
        handleParseError(error as CosyError)
      }
    }
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
        const tlsServerIssues = getTLSServerIssues()

        if (tlsServerIssues.length > 0 && !options.skipTLSValidation) {
          message.error($gettext('Please select a certificate before saving the TLS server configuration.'))
          throw new Error('tls_certificate_required')
        }

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
        overwrite: true,
        namespace_id: data.value.namespace_id,
        sync_node_ids: data.value.sync_node_ids,
        post_action: 'reload_nginx',
        dns_domain_id: data.value.dns_domain_id,
        dns_record_id: data.value.dns_record_id,
        dns_record_name: data.value.dns_record_name,
        dns_record_type: data.value.dns_record_type,
      })

      if (options.syncResponse !== false)
        await handleResponse(response)

      return response
    }
    catch (error) {
      if ((error as Error)?.message === 'tls_certificate_required')
        throw error

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
    config.getItem(`sites-available/${encodeURIComponent(name.value)}`).then(r => {
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
    Object.assign(ngxConfig, r.tokenized)

    const ngxConfigStore = useNgxConfigStore()

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
        let r = await site.getItem(encodeURIComponent(name.value))
        await handleResponse(r)
        r = await ngx.tokenize_config(configText.value)
        Object.assign(ngxConfig, {
          ...r,
          name: name.value,
        })
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

  const hasWildcardServerName = computed(() => {
    if (curDirectivesMap.value.server_name) {
      for (const v of curDirectivesMap.value.server_name) {
        const params = v?.params || ''
        if (params.includes('_'))
          return true
      }
    }

    return false
  })

  const hasExplicitIpAddress = computed(() => {
    if (curDirectivesMap.value.server_name) {
      for (const v of curDirectivesMap.value.server_name) {
        const params = v?.params || ''
        // Check for IPv4 or IPv6 addresses
        const ipv4Regex = /\b(?:\d{1,3}\.){3}\d{1,3}\b/
        const ipv6Regex = /\[?(?:[\da-f]{0,4}:){1,7}[\da-f]{0,4}\]?/i
        if (ipv4Regex.test(params) || ipv6Regex.test(params))
          return true
      }
    }

    return false
  })

  const isIpCertificate = computed(() => {
    return isDefaultServer.value || hasWildcardServerName.value
  })

  const needsManualIpInput = computed(() => {
    return isIpCertificate.value && !hasExplicitIpAddress.value
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
    isIpCertificate,
    needsManualIpInput,
    hasServers,
    getTLSServerIssues,
    getConfigWithoutIncompleteTLSServers,
    dnsLinked,
    linkedDNSName,
    init,
    save,
    handleModeChange,
  }
})
