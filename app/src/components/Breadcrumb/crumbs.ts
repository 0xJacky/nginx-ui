import type { Bread } from './types'

export type CrumbEntry
  = | { type: 'item', bread: Bread }
    | { type: 'collapsed', breads: Bread[] }

/** The header already has a home link, so a Home crumb only repeats it. */
export function withoutHome(breads: Bread[]): Bread[] {
  return breads.filter(bread => bread.name !== 'Home' && bread.path !== '/')
}

/**
 * Keeps the first crumb and the last few, folding the middle into one entry
 * once there are more than `maxVisible`. Deep config directories otherwise push
 * the header icons off screen.
 */
export function collapseCrumbs(breads: Bread[], maxVisible = 4): CrumbEntry[] {
  const items = (list: Bread[]) => list.map(bread => ({ type: 'item', bread }) as CrumbEntry)

  if (breads.length <= maxVisible)
    return items(breads)

  const tailLength = Math.max(1, maxVisible - 2)

  return [
    ...items(breads.slice(0, 1)),
    { type: 'collapsed', breads: breads.slice(1, breads.length - tailLength) },
    ...items(breads.slice(-tailLength)),
  ]
}
