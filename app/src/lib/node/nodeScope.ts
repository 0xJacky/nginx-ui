type NodeScopeReset = () => void

const resets = new Set<NodeScopeReset>()
let generation = 0

/**
 * Registers state that belongs to the selected node (caches, snapshots,
 * "already loaded" flags) so a node switch can drop it without a page reload.
 * Returns an unregister function.
 */
export function onNodeScopeReset(reset: NodeScopeReset): () => void {
  resets.add(reset)
  return () => {
    resets.delete(reset)
  }
}

/**
 * Increases on every node switch. Async loaders read it before awaiting and
 * compare afterwards, so a response for the previous node is dropped instead
 * of landing in the freshly reset state.
 */
export function nodeScopeGeneration(): number {
  return generation
}

/** Drops every registered piece of node-scoped state. */
export function resetNodeScope(): void {
  generation++
  resets.forEach(reset => reset())
}
