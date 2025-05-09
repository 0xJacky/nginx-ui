import type { AntDesignOutlinedIconType } from '@ant-design/icons-vue/lib/icons/AntDesignOutlined'

// src/types/vue-router.d.ts
import 'vue-router'

/**
 * @description Extend the types of router meta
 */

declare module 'vue-router' {
  interface RouteMeta {
    name: (() => string)
    icon?: AntDesignOutlinedIconType
    hiddenInSidebar?: boolean | (() => boolean)
    hideChildren?: boolean
    noAuth?: boolean
    status_code?: number
    error?: () => string
    lastRouteName?: string
    modules?: string[]
  }
}
