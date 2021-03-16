export const user = {
    namespace: true,
    state: {
        info: {
            id: null,
            name: null,
            power: null,
            college_id: null,
            college_name: null,
            major_id: null,
            major_name: null,
            position: null
        },
        token: null
    },
    mutations: {
        login(state, payload) {
            state.token = payload.token
        },
        logout(state) {
            sessionStorage.clear()
            state.info = {}
            state.token = null
        },
        update_user(state, payload) {
            state.info = payload
        }
    },
    actions: {
        async login({commit}, data) {
            commit('login', data)
        },
        async logout({commit}) {
            commit('logout')
        },
        async update_user({commit}, data) {
            commit('update_user', data)
        }
    },
    getters: {
        info(state) {
            return state.info
        },
        token(state) {
            return state.token
        },
        isLogin(state) {
            return !!state.token
        }
    }
}
