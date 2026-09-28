/** A node color in both themes: the light variant is too dark on a dark background. */
export interface NodeColor {
  light: string
  dark: string
}

/** The local node always uses the app's primary blue. */
export const LOCAL_NODE_COLOR: NodeColor = { light: '#1677ff', dark: '#4096ff' }

/**
 * Remote node colors, ordered so neighbouring ids get clearly different hues.
 * Blue is reserved for the local node, and red and green are left out because
 * they already mean offline / online in the node lists.
 */
export const NODE_COLOR_PALETTE: readonly NodeColor[] = [
  { light: '#722ed1', dark: '#9254de' }, // purple
  { light: '#fa8c16', dark: '#ffa940' }, // orange
  { light: '#13c2c2', dark: '#36cfc9' }, // cyan
  { light: '#eb2f96', dark: '#f759ab' }, // magenta
  { light: '#7cb305', dark: '#a0d911' }, // lime
  { light: '#fa541c', dark: '#ff7a45' }, // volcano
  { light: '#d4b106', dark: '#fadb14' }, // yellow
]

/** Stable color of a node: the same id always maps to the same color. */
export function getNodeColor(nodeId: number): NodeColor {
  if (!nodeId || nodeId < 0 || !Number.isFinite(nodeId))
    return LOCAL_NODE_COLOR

  return NODE_COLOR_PALETTE[(Math.max(1, Math.trunc(nodeId)) - 1) % NODE_COLOR_PALETTE.length]
}

/** The node color for the current theme. */
export function nodeColorFor(nodeId: number, isDark: boolean): string {
  const color = getNodeColor(nodeId)
  return isDark ? color.dark : color.light
}
