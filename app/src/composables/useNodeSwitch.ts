import { resetNodeScope } from '@/lib/node/nodeScope'
import { resolveSwitchLanding } from '@/lib/node/switch'
import { useSettingsStore } from '@/pinia'

/**
 * True while a switch is moving to its landing page. App.vue keeps the layout
 * unmounted meanwhile, so the page being left does not mount once more against
 * the new node (fetching from it, rewriting the URL) before the router moves on.
 */
export const isNodeSwitching = ref(false)

export interface NodeSwitchTarget {
  id: number
  name: string
}

/**
 * Switches the node that every request is proxied to, without reloading the
 * page. App.vue keys the layout by node id, so everything under it remounts:
 * pages refetch and their sockets reconnect to the new node. State that
 * outlives the layout (stores, module caches) is dropped via resetNodeScope().
 */
export function useNodeSwitch() {
  const router = useRouter()
  const settingsStore = useSettingsStore()

  async function switchNode(target: NodeSwitchTarget) {
    if (target.id === settingsStore.node.id)
      return

    isNodeSwitching.value = true
    try {
      settingsStore.node.id = target.id
      settingsStore.node.name = target.name

      resetNodeScope()

      // After the node change on purpose: hiddenInSidebar reads the new node.
      const current = router.currentRoute.value
      const landing = resolveSwitchLanding(current)

      // Queries often carry the old node's ids (namespace filters, directories),
      // so land on the bare path.
      if (landing !== current.fullPath)
        await router.replace(landing)
    }
    finally {
      isNodeSwitching.value = false
    }
  }

  function switchToLocal() {
    return switchNode({ id: 0, name: 'Local' })
  }

  return {
    switchNode,
    switchToLocal,
  }
}
