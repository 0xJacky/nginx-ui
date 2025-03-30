import type { RouteRecordRaw } from 'vue-router'
import { FileOutlined } from '@ant-design/icons-vue'

export const configRoutes: RouteRecordRaw[] = [
  {
    path: 'config',
    name: 'Manage Configs',
    component: () => import('@/views/config/ConfigList.vue'),
    meta: {
      name: () => $gettext('Manage Configs'),
      icon: FileOutlined,
      hideChildren: true,
    },
  },
  {
    path: 'config/add',
    name: 'Add Configuration',
    component: () => import('@/views/config/ConfigEditor.vue'),
    meta: {
      name: () => $gettext('Add Configuration'),
      hiddenInSidebar: true,
      lastRouteName: 'Manage Configs',
    },
  },
  {
    path: 'config/:name+/edit',
    name: 'Edit Configuration',
    component: () => import('@/views/config/ConfigEditor.vue'),
    meta: {
      name: () => $gettext('Edit Configuration'),
      hiddenInSidebar: true,
      lastRouteName: 'Manage Configs',
    },
  },
]
