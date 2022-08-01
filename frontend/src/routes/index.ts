import {createRouter, createWebHistory} from 'vue-router'
import gettext from '../gettext'
import {useUserStore} from '@/pinia'

import {
    CloudOutlined,
    CodeOutlined,
    FileOutlined,
    HomeOutlined,
    InfoCircleOutlined,
    UserOutlined
} from '@ant-design/icons-vue'

const {$gettext} = gettext

export const routes = [
    {
        path: '/',
        name: () => $gettext('Home'),
        component: () => import('@/layouts/BaseLayout.vue'),
        redirect: '/dashboard',
        children: [
            {
                path: 'dashboard',
                component: () => import('@/views/dashboard/DashBoard.vue'),
                name: () => $gettext('Dashboard'),
                meta: {
                    // hiddenHeaderContent: true,
                    icon: HomeOutlined
                }
            },
            {
                path: 'user',
                name: () => $gettext('Manage Users'),
                component: () => import('@/views/user/User.vue'),
                meta: {
                    icon: UserOutlined
                },
            },
            {
                path: 'domain',
                name: () => $gettext('Manage Sites'),
                component: () => import('@/layouts/BaseRouterView.vue'),
                meta: {
                    icon: CloudOutlined
                },
                redirect: '/domain/list',
                children: [{
                    path: 'list',
                    name: () => $gettext('Sites List'),
                    component: () => import('@/views/domain/DomainList.vue'),
                }, {
                    path: 'add',
                    name: () => $gettext('Add Site'),
                    component: () => import('@/views/domain/DomainAdd.vue'),
                }, {
                    path: ':name',
                    name: () => $gettext('Edit Site'),
                    component: () => import('@/views/domain/DomainEdit.vue'),
                    meta: {
                        hiddenInSidebar: true
                    }
                },]
            },
            {
                path: 'config',
                name: () => $gettext('Manage Configs'),
                component: () => import('@/views/config/Config.vue'),
                meta: {
                    icon: FileOutlined,
                    hideChildren: true
                }
            },
            {
                path: 'config/:name',
                name: () => $gettext('Edit Configuration'),
                component: () => import('@/views/config/ConfigEdit.vue'),
                meta: {
                    hiddenInSidebar: true
                },
            },
            {
                path: 'terminal',
                name: () => $gettext('Terminal'),
                component: () => import('@/views/pty/Terminal.vue'),
                meta: {
                    icon: CodeOutlined
                }
            },
            {
                path: 'about',
                name: () => $gettext('About'),
                component: () => import('@/views/other/About.vue'),
                meta: {
                    icon: InfoCircleOutlined
                }
            },
        ]
    },
    {
        path: '/install',
        name: () => $gettext('Install'),
        // component: () => import('@/views/other/Install.vue'),
        meta: {noAuth: true}
    },
    {
        path: '/login',
        name: () => $gettext('Login'),
        component: () => import('@/views/other/Login.vue'),
        meta: {noAuth: true}
    },
    {
        path: '/404',
        name: () => $gettext('404 Not Found'),
        component: () => import('@/views/other/Error.vue'),
        meta: {noAuth: true, status_code: 404, error: 'Not Found'}
    },
    {
        path: '/*',
        name: () => $gettext('Not Found'),
        redirect: '/404',
        meta: {noAuth: true}
    }
]

const router = createRouter({
    history: createWebHistory(),
    // @ts-ignore
    routes: routes,
})

router.beforeEach((to, from, next) => {

    // @ts-ignore
    document.title = to.name() + ' | Nginx UI'

    if (import.meta.env.MODE === 'production') {
        // axios.get('/version.json?' + Date.now()).then(r => {
        //     if (!(process.env.VUE_APP_VERSION === r.data.version
        //         && Number(process.env.VUE_APP_BUILD_ID) === r.data.build_id)) {
        //         Vue.prototype.$info({
        //             title: $gettext('System message'),
        //             content: $gettext('Detected version update, this page will refresh.'),
        //             onOk() {
        //                 location.reload()
        //             },
        //             okText: $gettext('OK')
        //         })
        //     }
        // })
    }

    const user = useUserStore()
    const {is_login} = user

    if (to.meta.noAuth || is_login) {
        next()
    } else {
        next({path: '/login', query: {next: to.fullPath}})
    }

})

export default router
