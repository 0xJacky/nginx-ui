import type { NgxModule } from '@/api/ngx'
import type { NginxStatus } from '@/constants'
import publicApi from '@/api/public'

interface ProcessingStatus {
  index_scanning: boolean
  auto_cert_processing: boolean
  nginx_log_indexing: boolean
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
    demoProbe ??= publicApi.getICP()
      .then(info => {
        isDemo.value = info.demo === true
        return isDemo.value
      })
      .catch(() => false)

    return demoProbe
  }

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
