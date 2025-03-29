import type { RouteRecordRaw } from 'vue-router'
import { CloudOutlined } from '@ant-design/icons-vue'

export const sitesRoutes: RouteRecordRaw[] = [
  {
    path: 'sites',
    name: 'Manage Sites',
    component: () => import('@/layouts/BaseRouterView.vue'),
    meta: {
      name: () => $gettext('Manage Sites'),
      icon: CloudOutlined,
    },
    redirect: '/sites/list',
    children: [{
      path: 'list',
      name: 'Sites List',
      component: () => import('@/views/site/site_list/SiteList.vue'),
      meta: {
        name: () => $gettext('Sites List'),
      },
    }, {
      path: 'add',
      name: 'Add Site',
      component: () => import('@/views/site/site_add/SiteAdd.vue'),
      meta: {
        name: () => $gettext('Add Site'),
        lastRouteName: 'Sites List',
      },
    }, {
      path: 'categories',
      name: 'Site Categories',
      component: () => import('@/views/site/site_category/SiteCategory.vue'),
      meta: {
        name: () => $gettext('Site Categories'),
      },
    }, {
      path: ':name',
      name: 'Edit Site',
      component: () => import('@/views/site/site_edit/SiteEdit.vue'),
      meta: {
        name: () => $gettext('Edit Site'),
        hiddenInSidebar: true,
        lastRouteName: 'Sites List',
      },
    }],
  },
]
