// src/types/vue-router.d.ts
import 'vue-router'

import type {AntDesignOutlinedIconType} from '@ant-design/icons-vue/lib/icons/AntDesignOutlined'

/**
 * @description Extend the types of router meta
 */

declare module 'vue-router' {
  interface RouteMeta {
    name: (() => string)
    icon?: AntDesignOutlinedIconType
    hiddenInSidebar?: boolean
    hideChildren?: boolean
    noAuth?: boolean
    status_code?: number
    error?: () => string
  }
}
