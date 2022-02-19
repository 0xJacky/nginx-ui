import Vue from 'vue'
import App from './App.vue'
import store from './lib/store'
import '@/lazy'
import '@/assets/css/dark.less'
import '@/assets/css/style.less'
import {router, routes} from './router'
import NProgress from 'nprogress'
import 'nprogress/nprogress.css'
import utils from '@/lib/utils'
import api from '@/api'
import GetTextPlugin from 'vue-gettext'
import {availableLanguages} from '@/lib/translate'
import http from '@/lib/http'

Vue.use(utils)

Vue.config.productionTip = false

Vue.prototype.$routeConfig = routes
Vue.prototype.$api = api

Vue.use(GetTextPlugin, {
    availableLanguages,
    defaultLanguage: store.getters.current_language,
    translations: store.state.settings.translations,
    silent: true
})

http.get('/translations.json').then(r => {
    store.commit('update_translations', r)
})

NProgress.configure({
    easing: 'ease',
    speed: 500,
    showSpinner: false,
    trickleSpeed: 200,
    minimum: 0.3
})

router.beforeEach((to, from, next) => {
    NProgress.start()
    next()
})

router.afterEach(() => {
    NProgress.done()
})

new Vue({
    store,
    router,
    render: h => h(App)
}).$mount('#app')
