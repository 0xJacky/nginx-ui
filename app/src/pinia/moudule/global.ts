import type { NgxModule } from '@/api/ngx'
import type { NginxStatus } from '@/constants'
import publicApi from '@/api/public'
import { nodeScopeGeneration, onNodeScopeReset } from '@/lib/node/nodeScope'

/** Background work a plugin reports, label is an English source string. */
export interface ProcessingPluginEntry {
  plugin_id: string
  key: string
  label: string
}

interface ProcessingStatus {
  index_scanning: boolean
  auto_cert_processing: boolean
  nginx_log_indexing: boolean
  plugins: ProcessingPluginEntry[]
}

interface NginxLogStatus {
  indexing: boolean
}

type NginxStatusType = NginxStatus.Reloading | NginxStatus.Restarting | NginxStatus.Running | NginxStatus.Stopped

export const useGlobalStore = defineStore('global', () => {
  const nginxStatus: Ref<NginxStatusType> = ref(0)

  const processingStatus = ref<ProcessingStatus>({
    index_scanning: false,
    auto_cert_processing: false,
    nginx_log_indexing: false,
    plugins: [],
  })

  const nginxLogStatus = ref<NginxLogStatus>({
    indexing: false,
  })

  const modules = ref<NgxModule[]>([])
  const modulesMap = ref<Record<string, NgxModule>>({})

  // Whether this node is a public demo. Deliberately kept out of the settings
  // store: that one is persisted to localStorage, and a stale `true` carried
  // over from the demo would silently degrade a real installation.
  const isDemo = ref(false)
  let demoProbe: Promise<boolean> | null = null

  /**
   * Resolve the demo flag once per page load. Safe to await from a router
   * guard: it never rejects, and a failed probe leaves the flag false, which
   * is the conservative answer (behave like a normal install).
   */
  function ensureDemoFlag(): Promise<boolean> {
    const generation = nodeScopeGeneration()
    demoProbe ??= publicApi.getICP()
      .then(info => {
        const isDemoNode = info.demo === true
        if (generation === nodeScopeGeneration())
          isDemo.value = isDemoNode
        return isDemoNode
      })
      .catch(() => false)

    return demoProbe
  }

  // Everything here describes the selected node: its modules, its background
  // jobs and whether it is a demo.
  onNodeScopeReset(() => {
    processingStatus.value = {
      index_scanning: false,
      auto_cert_processing: false,
      nginx_log_indexing: false,
      plugins: [],
    }
    nginxLogStatus.value = { indexing: false }
    modules.value = []
    modulesMap.value = {}
    isDemo.value = false
    demoProbe = null
  })

  return {
    nginxStatus,
    processingStatus,
    nginxLogStatus,
    modules,
    modulesMap,
    isDemo,
    ensureDemoFlag,
  }
})
