import type { CertificateInfo } from '@/api/cert'
import type { Stream } from '@/api/stream'
import type { CheckedType } from '@/types'
import config from '@/api/config'
import ngx from '@/api/ngx'
import stream from '@/api/stream'
import { useNgxConfigStore } from '@/components/NgxConfigEditor'
import { ConfigStatus } from '@/constants'

export const useStreamEditorStore = defineStore('streamEditor', () => {
  const { message } = App.useApp()

  const name = ref('')
  const advanceMode = ref(false)
  const parseErrorStatus = ref(false)
  const parseErrorMessage = ref('')
  const data = ref({}) as Ref<Stream>
  const loading = ref(true)
  const saving = ref(false)
  const autoCert = ref(false)
  const certInfoMap = ref({}) as Ref<Record<number, CertificateInfo[]>>
  const filename = ref('')
  const filepath = ref('')
  const status = ref(ConfigStatus.Disabled)

  const ngxConfigStore = useNgxConfigStore()
  const { ngxConfig, configText, curServerIdx, curServer, curServerDirectives, curDirectivesMap } = storeToRefs(ngxConfigStore)

  async function init(_name: string) {
    loading.value = true
    name.value = _name
    await nextTick()

    if (name.value) {
      try {
        const r = await stream.getItem(encodeURIComponent(name.value))
        handleResponse(r)
      }
      catch (error) {
        handleParseError(error as { error?: string, message: string })
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

      const response = await stream.updateItem(encodeURIComponent(name.value), {
        content: configText.value,
        overwrite: true,
        namespace_id: data.value.namespace_id,
        sync_node_ids: data.value.sync_node_ids,
        post_action: 'reload_nginx',
      })

      handleResponse(response)

      message.success($gettext('Saved successfully'))
    }
    catch (error) {
      handleParseError(error as { error?: string, message: string })
    }
    finally {
      saving.value = false
    }
  }

  function handleParseError(e: { error?: string, message: string }) {
    console.error(e)
    parseErrorStatus.value = true
    parseErrorMessage.value = e.message
    config.getItem(`streams-available/${encodeURIComponent(name.value)}`).then(r => {
      configText.value = r.content
    })
  }

  async function handleResponse(r: Stream) {
    if (r.advanced)
      advanceMode.value = true

    status.value = r.status
    parseErrorStatus.value = false
    parseErrorMessage.value = ''
    filename.value = r.name
    filepath.value = r.filepath
    configText.value = r.config
    data.value = r
    Object.assign(ngxConfig, r.tokenized)

    const ngxConfigStore = useNgxConfigStore()

    if (r.tokenized)
      ngxConfigStore.setNgxConfig(r.tokenized)
  }

  async function handleModeChange(advanced: CheckedType) {
    loading.value = true

    try {
      await stream.advance_mode(encodeURIComponent(name.value), { advanced: advanced as boolean })
      advanceMode.value = advanced as boolean
      if (advanced) {
        await buildConfig()
      }
      else {
        let r = await stream.getItem(encodeURIComponent(name.value))
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
    status,
    init,
    save,
    handleModeChange,
  }
})
