import type { CertificateInfo } from '@/api/cert'
import type { Stream } from '@/api/stream'
import type { CheckedType } from '@/types'
import config from '@/api/config'
import ngx from '@/api/ngx'
import stream from '@/api/stream'
import { useNgxConfigStore } from '@/components/NgxConfigEditor'
import { ConfigStatus } from '@/constants'

export const useStreamEditorStore = defineStore('streamEditor', () => {
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

  // This store is a singleton, so opening another stream reuses the state left
  // behind by the previous one. Every init() run takes a ticket and only the
  // newest one may write, otherwise a slow response for the stream the operator
  // just left would land on top of the one now on screen.
  let loadSeq = 0

  function reset() {
    name.value = ''
    advanceMode.value = false
    parseErrorStatus.value = false
    parseErrorMessage.value = ''
    data.value = {} as Stream
    autoCert.value = false
    certInfoMap.value = {}
    filename.value = ''
    filepath.value = ''
    status.value = ConfigStatus.Disabled
    ngxConfigStore.reset()
  }

  async function init(_name: string) {
    const seq = ++loadSeq
    loading.value = true
    reset()
    name.value = _name
    await nextTick()
    if (seq !== loadSeq)
      return

    if (name.value) {
      try {
        const r = await stream.getItem(encodeURIComponent(name.value))
        if (seq !== loadSeq)
          return
        handleResponse(r)
      }
      catch (error) {
        if (seq !== loadSeq)
          return
        handleParseError(error as { error?: string, message: string })
      }
    }

    if (seq === loadSeq)
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

      if (data.value.sync_node_ids === null) {
        data.value.sync_node_ids = []
      }

      // @ts-expect-error allow comparing with empty string for legacy data
      if (data.value.namespace_id === '') {
        data.value.namespace_id = 0
      }

      const response = await stream.updateItem(encodeURIComponent(name.value), {
        content: configText.value,
        overwrite: true,
        namespace_id: data.value.namespace_id,
        sync_node_ids: data.value.sync_node_ids,
        post_action: 'reload_nginx',
      })

      handleResponse(response)
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
    const target = name.value
    config.getItem(`streams-available/${encodeURIComponent(target)}`).then(r => {
      // Another stream may have taken over the store while this was in flight.
      if (name.value === target)
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
        const r = await stream.getItem(encodeURIComponent(name.value))
        await handleResponse(r)
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
    reset,
    save,
    handleModeChange,
  }
})
