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
    /** Id of the plugin that contributed this route. */
    pluginId?: string
    /** Name of the top level sidebar entry this plugin route is nested under. */
    pluginParent?: string
    /** Sort order of this plugin route inside its sidebar group. */
    pluginOrder?: number
  }
}
