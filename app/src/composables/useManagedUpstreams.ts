import type { ManagedUpstreamDetail } from '@/api/upstream'
import upstream from '@/api/upstream'

// Shared across every picker on the page so opening several location editors
// issues one request.
const upstreams = ref<ManagedUpstreamDetail[]>([])
const isLoading = ref(false)
let pending: Promise<void> | null = null

async function load() {
  if (pending)
    return pending

  isLoading.value = true
  pending = upstream.getManagedList()
    .then(res => {
      upstreams.value = res.data ?? []
    })
    .catch(() => {
      // The global error handler already reported it; keep the last list.
    })
    .finally(() => {
      isLoading.value = false
      pending = null
    })
  return pending
}

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
