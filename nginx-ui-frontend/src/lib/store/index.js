import Vue from 'vue'
import Vuex from 'vuex'
import VuexPersistence from 'vuex-persist'

Vue.use(Vuex)

const debug = process.env.NODE_ENV !== 'production'

const vuexLocal = new VuexPersistence({
    storage: window.localStorage,
    modules: []
})

export default new Vuex.Store({
    // 将各组件分别模块化存入 Store
    modules: {},
    plugins: [vuexLocal.plugin],
    strict: debug
})
