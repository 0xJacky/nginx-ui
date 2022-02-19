export const settings = {
    namespace: true,
    state: {
        language: ''
    },
    mutations: {
        set_language(state, payload) {
            state.language = payload
        },
    },
    actions: {
        set_language({commit}, data) {
            commit('set_language', data)
        },
    },
    getters: {
        current_language(state) {
            return state.language
        }
    }
}
