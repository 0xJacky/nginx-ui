import type { CertificateInfo } from '@/api/cert'
import type { ChatComplicationMessage } from '@/api/openai'
import type { Stream } from '@/api/stream'
import type { CheckedType } from '@/types'
import config from '@/api/config'
import ngx from '@/api/ngx'
import stream from '@/api/stream'
import { useNgxConfigStore } from '@/components/NgxConfigEditor'

export const useStreamEditorStore = defineStore('streamEditor', () => {
  const name = ref('')
  const advanceMode = ref(false)
  const parseErrorStatus = ref(false)
  const parseErrorMessage = ref('')
  const data = ref({}) as Ref<Stream>
  const historyChatgptRecord = ref([]) as Ref<ChatComplicationMessage[]>
  const loading = ref(true)
  const saving = ref(false)
  const autoCert = ref(false)
  const certInfoMap = ref({}) as Ref<Record<number, CertificateInfo[]>>
  const filename = ref('')
  const filepath = ref('')
  const enabled = ref(false)

  const ngxConfigStore = useNgxConfigStore()
  const { ngxConfig, configText, curServerIdx, curServer, curServerDirectives, curDirectivesMap } = storeToRefs(ngxConfigStore)

  async function init(_name: string) {
    loading.value = true
    name.value = _name
    await nextTick()

    if (name.value) {
      try {
        const r = await stream.get(name.value)
        handleResponse(r)
      }
      catch (error) {
        handleParseError(error as { error?: string, message: string })
      }
    }
    else {
      historyChatgptRecord.value = []
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

      const response = await stream.save(name.value, {
        content: configText.value,
        overwrite: true,
        env_group_id: data.value.env_group_id,
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
    config.get(`streams-available/${name.value}`).then(r => {
      configText.value = r.content
    })
  }

  async function handleResponse(r: Stream) {
    if (r.advanced)
      advanceMode.value = true

    enabled.value = r.enabled
    parseErrorStatus.value = false
    parseErrorMessage.value = ''
    filename.value = r.name
    filepath.value = r.filepath
    configText.value = r.config
    historyChatgptRecord.value = r.chatgpt_messages
    data.value = r
    Object.assign(ngxConfig, r.tokenized)

    const ngxConfigStore = useNgxConfigStore()

    if (r.tokenized)
      ngxConfigStore.setNgxConfig(r.tokenized)
  }

  async function handleModeChange(advanced: CheckedType) {
    loading.value = true

    try {
      await stream.advance_mode(name.value, { advanced: advanced as boolean })
      advanceMode.value = advanced as boolean
      if (advanced) {
        await buildConfig()
      }
      else {
        let r = await stream.get(name.value)
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
    historyChatgptRecord,
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
    enabled,
    init,
    save,
    handleModeChange,
  }
})
