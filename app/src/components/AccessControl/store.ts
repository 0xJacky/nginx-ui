import type {
  AccessChange,
  AccessControlSource,
  AccessList,
  AccessLocationState,
  AccessServerState,
} from '@/api/access_list'
import type { NgxConfig, NgxDirective } from '@/api/ngx'
import type { CosyError } from '@/lib/http/types'
import { useDebounceFn } from '@vueuse/core'
import accessList from '@/api/access_list'
import { useNgxConfigStore } from '@/components/NgxConfigEditor'
import { translateError } from '@/lib/http/error'

function directivesKey(directives: NgxDirective[] = []) {
  return JSON.stringify(directives.map(d => [d.directive, d.params, d.comments ?? '']))
}

// Access state of the site or stream open in the editor. The backend detects
// and rewrites the access directives, so basic mode, advanced mode and the
// batch actions share one implementation.
export const useAccessControlStore = defineStore('accessControl', () => {
  const ngxConfigStore = useNgxConfigStore()
  const { ngxConfig, configText } = storeToRefs(ngxConfigStore)

  const active = ref(false)
  const advanced = ref(false)
  const servers = ref<AccessServerState[]>([])
  const lists = ref<AccessList[]>([])
  const isLoadingLists = ref(false)
  const isApplying = ref(false)
  const stateError = ref('')

  const listsBySlug = computed(() => {
    const map: Record<string, AccessList> = {}
    lists.value.forEach(l => {
      map[l.slug] = l
    })
    return map
  })

  function source(): AccessControlSource {
    return advanced.value
      ? { content: configText.value }
      : { config: ngxConfig.value }
  }

  // Each request takes a ticket; only the newest may write, so a slow state
  // request cannot overwrite the result of an apply.
  let seq = 0

  async function refresh() {
    if (!active.value)
      return
    const ticket = ++seq
    try {
      const result = await accessList.getState(source())
      if (ticket !== seq)
        return
      servers.value = result.servers ?? []
      stateError.value = ''
    }
    catch (e) {
      if (ticket !== seq)
        return
      servers.value = []
      stateError.value = await translateError(e as CosyError)
    }
  }

  const scheduleRefresh = useDebounceFn(refresh, 400)

  watch(
    () => advanced.value ? configText.value : ngxConfig.value,
    () => scheduleRefresh(),
    { deep: true },
  )
  watch(advanced, () => refresh())

  async function loadLists() {
    isLoadingLists.value = true
    try {
      lists.value = (await accessList.getAll()).data ?? []
    }
    finally {
      isLoadingLists.value = false
    }
  }

  // Only replace the parts the backend changed, so the directive and
  // location editors keep their own state for everything else.
  function patchConfig(next?: NgxConfig) {
    if (!next)
      return
    next.servers?.forEach((server, i) => {
      const current = ngxConfig.value.servers?.[i]
      if (!current)
        return
      if (directivesKey(current.directives) !== directivesKey(server.directives))
        current.directives = server.directives
      server.locations?.forEach((location, j) => {
        const target = current.locations?.[j]
        if (target && target.content !== location.content)
          target.content = location.content
      })
    })
  }

  async function apply(changes: AccessChange[]) {
    if (changes.length === 0)
      return
    isApplying.value = true
    const ticket = ++seq
    try {
      const result = await accessList.apply(source(), changes)
      if (advanced.value) {
        if (result.content !== undefined)
          configText.value = result.content
      }
      else {
        patchConfig(result.config)
      }
      if (ticket === seq) {
        servers.value = result.servers ?? []
        stateError.value = ''
      }
    }
    finally {
      isApplying.value = false
    }
  }

  function serverState(serverIdx: number): AccessServerState | undefined {
    return servers.value[serverIdx]
  }

  function locationState(serverIdx: number, locationIdx: number): AccessLocationState | undefined {
    return servers.value[serverIdx]?.locations?.[locationIdx]
  }

  function listName(slug?: string) {
    if (!slug)
      return ''
    return listsBySlug.value[slug]?.name ?? slug
  }

  // Called by the site and stream editors while they are mounted.
  function activate() {
    active.value = true
    loadLists()
    refresh()
  }

  function deactivate() {
    active.value = false
    seq++
    servers.value = []
    stateError.value = ''
  }

  return {
    active,
    advanced,
    servers,
    lists,
    listsBySlug,
    isLoadingLists,
    isApplying,
    stateError,
    refresh,
    loadLists,
    apply,
    serverState,
    locationState,
    listName,
    activate,
    deactivate,
  }
})
