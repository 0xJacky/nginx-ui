export const user = {
    namespace: true,
    state: {
        token: null
    },
    mutations: {
        login(state, payload) {
            state.token = payload.token
        },
        logout(state) {
            sessionStorage.clear()
            state.token = null
        }
    },
    actions: {
        async login({commit}, data) {
            commit('login', data)
        },
        async logout({commit}) {
            commit('logout')
        }
    },
    getters: {
        token(state) {
            return state.token
        }
    }
}
