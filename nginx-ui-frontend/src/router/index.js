import Vue from 'vue'
import VueRouter from 'vue-router'
import axios from 'axios'
import store from '@/lib/store'

Vue.use(VueRouter)

export const routes = [
    {
        path: '/',
        name: '首页',
        component: () => import('@/layouts/BaseLayout'),
        redirect: '/dashboard',
        children: [
            {
                path: 'dashboard',
                component: () => import('@/views/DashBoard'),
                name: '仪表盘',
                meta: {
                    //hiddenHeaderContent: true,
                    icon: 'home'
                }
            },
            {
                path: 'domain',
                name: '网站管理',
                component: () => import('@/layouts/BaseRouterView'),
                meta: {
                    icon: 'cloud'
                },
                redirect: '/domain/list',
                children: [{
                    path: 'list',
                    name: '网站列表',
                    component: () => import('@/views/Domain.vue'),
                }, {
                    path: 'add',
                    name: '添加站点',
                    component: () => import('@/views/domain_edit/DomainEdit.vue'),
                }, {
                    path: ':name',
                    name: '编辑站点',
                    component: () => import('@/views/domain_edit/DomainEdit.vue'),
                    meta: {
                        hiddenInSidebar: true
                    }
                }, ]
            },
            {
                path: 'config',
                name: '配置管理',
                component: () => import('@/views/Config.vue'),
                meta: {
                    icon: 'file'
                },
            },
            {
                path: 'config/:name',
                name: '配置编辑',
                component: () => import('@/views/ConfigEdit.vue'),
                meta: {
                    hiddenInSidebar: true
                },
            },
            {
                path: 'about',
                name: '关于',
                component: () => import('@/views/About.vue'),
                meta: {
                    icon: 'info-circle'
                }
            },
        ]
    },
    {
        path: '/login',
        name: '登录',
        component: () => import('@/views/Login'),
        meta: {noAuth: true}
    },
    {
        path: '/404',
        name: '404 Not Found',
        component: () => import('@/views/Error'),
        meta: {noAuth: true, status_code: 404, error: 'Not Found'}
    },
    {
        path: '*',
        name: '未找到页面',
        redirect: '/404',
        meta: {noAuth: true}
    }
]

const router = new VueRouter({
    routes
})

router.beforeEach((to, from, next) => {
    document.title = 'Nginx UI | ' + to.name

    if (process.env.NODE_ENV === 'production') {
        axios.get('/version.json?' + Date.now()).then(r => {
            if (!(process.env.VUE_APP_VERSION === r.data.version
                && Number(process.env.VUE_APP_BUILD_ID) === r.data.build_id)) {
                Vue.prototype.$info({
                    title: '系统信息',
                    content: '检测到版本更新，将会自动刷新本页',
                    onOk() {
                        location.reload()
                    },
                    okText: '好的'
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
