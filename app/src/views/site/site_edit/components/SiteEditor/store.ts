import type { CertificateInfo } from '@/api/cert'
import type { Site } from '@/api/site'
import type { CosyError } from '@/lib/http/types'
import type { CheckedType } from '@/types'
import config from '@/api/config'
import ngx from '@/api/ngx'
import site from '@/api/site'
import { useNgxConfigStore } from '@/components/NgxConfigEditor'
import { translateError } from '@/lib/http/error'

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

  async function buildConfig() {
    return ngx.build_config(ngxConfig.value).then(r => {
      configText.value = r.content
    })
  }

  async function save() {
    saving.value = true

    try {
      if (!advanceMode.value) {
        await buildConfig()
      }

      const response = await site.updateItem(encodeURIComponent(name.value), {
        content: configText.value,
        overwrite: true,
        env_group_id: data.value.env_group_id,
        sync_node_ids: data.value.sync_node_ids,
        post_action: 'reload_nginx',
      })

      handleResponse(response)
    }
    catch (error) {
      handleParseError(error as CosyError)
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
    hasServers,
    init,
    save,
    handleModeChange,
  }
})
