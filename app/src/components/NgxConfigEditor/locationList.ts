// Helpers for editing a server's location list. Each returns a new list instead
// of changing the given one, so the editor can assign the result back to its
// v-model. A server without locations comes in as null (or undefined), and an
// in-place push would then fail or only reach the editor's local default list,
// never the site config that gets built and saved.

import type { NgxLocation } from '@/api/ngx'
import { cloneDeep } from 'lodash'

type LocationList = NgxLocation[] | null | undefined

export function appendLocation(locations: LocationList, location: NgxLocation): NgxLocation[] {
  return [...(locations ?? []), location]
}

export function removeLocation(locations: LocationList, index: number): NgxLocation[] {
  return (locations ?? []).filter((_, i) => i !== index)
}

/** Inserts a deep copy of the location at `index` right before it. */
export function duplicateLocation(locations: LocationList, index: number): NgxLocation[] {
  const list = [...(locations ?? [])]
  const location = list[index]
  if (!location)
    return list

  list.splice(index, 0, cloneDeep(location))
  return list
}
