import type { LocationQueryRaw } from 'vue-router'

export interface Bread {
  name: string
  translatedName: () => string
  path?: string
  query?: LocationQueryRaw
  hasChildren?: boolean
}
