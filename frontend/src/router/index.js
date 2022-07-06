import Vue from 'vue'
import VueRouter from 'vue-router'
import axios from 'axios'
import store from '@/lib/store'
import $gettext from '@/lib/translate/gettext'

Vue.use(VueRouter)

export const routes = [
    {
        path: '/',
        name: $gettext('Home'),
        component: () => import('@/layouts/BaseLayout'),
        redirect: '/dashboard',
        children: [
            {
                path: 'dashboard',
                component: () => import('@/views/dashboard/DashBoard'),
                name:  $gettext('Dashboard'),
                meta: {
                    //hiddenHeaderContent: true,
                    icon: 'home'
                }
            },
            {
                path: 'user',
                name: $gettext('Manage Users'),
                component: () => import('@/views/user/User.vue'),
                meta: {
                    icon: 'user'
                },
            },
            {
                path: 'domain',
                name: $gettext('Manage Sites'),
                component: () => import('@/layouts/BaseRouterView'),
                meta: {
                    icon: 'cloud'
                },
                redirect: '/domain/list',
                children: [{
                    path: 'list',
                    name: $gettext('Sites List'),
                    component: () => import('@/views/domain/DomainList.vue'),
                }, {
                    path: 'add',
                    name: $gettext('Add Site'),
                    component: () => import('@/views/domain/DomainAdd.vue'),
                }, {
                    path: ':name',
                    name: $gettext('Edit Site'),
                    component: () => import('@/views/domain/DomainEdit.vue'),
                    meta: {
                        hiddenInSidebar: true
                    }
                },]
            },
            {
                path: 'config',
                name: $gettext('Manage Configs'),
                component: () => import('@/views/config/Config.vue'),
                meta: {
                    icon: 'file',
                    hideChildren: true
                }
            },
            {
                path: 'config/:name',
                name: $gettext('Edit Configuration'),
                component: () => import('@/views/config/ConfigEdit.vue'),
                meta: {
                    hiddenInSidebar: true
                },
            },
            {
                path: 'terminal',
                name: $gettext('Terminal'),
                component: () => import('@/views/pty/Terminal'),
                meta: {
                    icon: 'code'
                }
            },
            {
                path: 'about',
                name: $gettext('About'),
                component: () => import('@/views/other/About.vue'),
                meta: {
                    icon: 'info-circle'
                }
            },
        ]
    },
    {
        path: '/install',
        name: $gettext('Install'),
        component: () => import('@/views/other/Install'),
        meta: {noAuth: true}
    },
    {
        path: '/login',
        name: $gettext('Login'),
        component: () => import('@/views/other/Login'),
        meta: {noAuth: true}
    },
    {
        path: '/404',
        name: $gettext('404 Not Found'),
        component: () => import('@/views/other/Error'),
        meta: {noAuth: true, status_code: 404, error: 'Not Found'}
    },
    {
        path: '*',
        name: $gettext('Not Found'),
        redirect: '/404',
        meta: {noAuth: true}
    }
]

const router = new VueRouter({
    routes,
    mode: 'history'
})

router.beforeEach((to, from, next) => {
    document.title = to.name + ' | Nginx UI'

    if (process.env.NODE_ENV === 'production') {
        axios.get('/version.json?' + Date.now()).then(r => {
            if (!(process.env.VUE_APP_VERSION === r.data.version
                && Number(process.env.VUE_APP_BUILD_ID) === r.data.build_id)) {
                Vue.prototype.$info({
                    title: $gettext('System message'),
                    content: $gettext('Detected version update, this page will refresh.'),
                    onOk() {
                        location.reload()
                    },
                    okText: $gettext('OK')
                })
            }
        })
    }

    if (to.meta.noAuth || store.getters.token) {
        next()
    } else {
        next({path: '/login', query: {next: to.fullPath}})
    }

})

export {router}
