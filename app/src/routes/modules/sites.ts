import type { RouteRecordRaw } from 'vue-router'
import { CloudOutlined } from '@antdv-next/icons'

export const sitesRoutes: RouteRecordRaw[] = [
  {
    path: 'sites',
    name: 'Manage Sites',
    component: () => import('@/layouts/BaseRouterView.vue'),
    meta: {
      name: () => $gettext('Manage Sites'),
      icon: CloudOutlined,
    },
    children: [{
      path: '',
      name: 'Sites Home',
      component: () => import('@/views/site/index.vue'),
      meta: {
        name: () => $gettext('Manage Sites'),
        hiddenInSidebar: true,
      },
    }, {
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
      path: 'snippets',
      name: 'Snippets',
      component: () => import('@/views/snippet/SnippetList.vue'),
      meta: {
        name: () => $gettext('Snippets'),
      },
    }, {
      path: 'snippets/add',
      name: 'Create Snippet',
      component: () => import('@/views/snippet/SnippetEdit.vue'),
      meta: {
        name: () => $gettext('Create Snippet'),
        hiddenInSidebar: true,
        lastRouteName: 'Snippets',
      },
    }, {
      path: 'snippets/templates/:name',
      name: 'Built-in Template',
      component: () => import('@/views/snippet/BuiltinTemplateView.vue'),
      meta: {
        name: () => $gettext('Template'),
        hiddenInSidebar: true,
        lastRouteName: 'Snippets',
      },
    }, {
      path: 'snippets/:file',
      name: 'Edit Snippet',
      component: () => import('@/views/snippet/SnippetEdit.vue'),
      meta: {
        name: () => $gettext('Edit Snippet'),
        hiddenInSidebar: true,
        lastRouteName: 'Snippets',
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
