import type { ConvertSiteUpstreamRequest } from '@/api/upstream'
import { kbToZoneSize } from '@/views/upstream/zoneSize'

export interface ZoneDirectiveLike {
  directive?: string
  params?: string
}

/**
 * Returns the parameters of the zone directive an upstream block declares
 * (for example `backend 64k` or `shared`), or undefined when it has none.
 */
export function findDeclaredZone(directives: ZoneDirectiveLike[] | undefined): string | undefined {
  const zone = directives?.find(d => d.directive?.trim() === 'zone')
  if (!zone)
    return undefined
  return (zone.params ?? '').trim().replace(/;$/, '').trim()
}

export type ConvertZoneChoice = Pick<ConvertSiteUpstreamRequest, 'zone' | 'zone_size'>

/**
 * Builds the zone fields of a convert request. A block that already declares
 * a zone keeps it whatever the switch says, so nothing is sent for it.
 */
export function convertZoneChoice(
  declaredZone: string | undefined,
  isZoneEnabled: boolean,
  zoneSizeKb: number | null | undefined,
): ConvertZoneChoice {
  if (declaredZone !== undefined)
    return {}
  if (!isZoneEnabled)
    return { zone: false }
  const size = kbToZoneSize(zoneSizeKb)
  return size ? { zone: true, zone_size: size } : { zone: true }
}
