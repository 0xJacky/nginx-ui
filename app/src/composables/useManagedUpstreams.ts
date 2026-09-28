import type { ManagedUpstreamDetail } from '@/api/upstream'
import upstream from '@/api/upstream'
import { nodeScopeGeneration, onNodeScopeReset } from '@/lib/node/nodeScope'

// Shared across every picker on the page so opening several location editors
// issues one request.
const upstreams = ref<ManagedUpstreamDetail[]>([])
const isLoading = ref(false)
let pending: Promise<void> | null = null

async function load() {
  if (pending)
    return pending

  isLoading.value = true
  const generation = nodeScopeGeneration()
  pending = upstream.getManagedList()
    .then(res => {
      if (generation === nodeScopeGeneration())
        upstreams.value = res.data ?? []
    })
    .catch(() => {
      // The global error handler already reported it; keep the last list.
    })
    .finally(() => {
      // A switch may have started a newer request; leave that one alone.
      if (generation !== nodeScopeGeneration())
        return

      isLoading.value = false
      pending = null
    })
  return pending
}

// Upstream groups live on the selected node. Forgetting `pending` too makes
// the next picker fetch from the new node instead of joining the old request.
onNodeScopeReset(() => {
  upstreams.value = []
  isLoading.value = false
  pending = null
})

/**
 * Lists the upstream groups managed from the Upstream page, for the pickers
 * that let a site proxy to one of them by name.
 */
export function useManagedUpstreams() {
  onMounted(load)

  const options = computed(() => upstreams.value.map(item => ({
    label: item.name,
    value: item.name,
  })))

  const names = computed(() => new Set(upstreams.value.map(item => item.name)))

  return {
    upstreams,
    options,
    names,
    isLoading,
    reload: load,
  }
}
