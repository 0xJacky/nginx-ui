import type { Component } from 'vue'

// src/types/vue-router.d.ts
import 'vue-router'

/**
 * @description Extend the types of router meta
 */

declare module 'vue-router' {
  interface RouteMeta {
    name: (() => string)
    icon?: Component
    hiddenInSidebar?: boolean | (() => boolean)
    hideChildren?: boolean
    noAuth?: boolean
    status_code?: number
    error?: () => string
    lastRouteName?: string
    modules?: string[]
  }
}
