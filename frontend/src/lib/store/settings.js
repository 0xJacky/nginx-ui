export const settings = {
    namespace: true,
    state: {
        language: '',
        env: {}
    },
    mutations: {
        set_language(state, payload) {
            state.language = payload
        },
        update_env(state, payload) {
            state.env = {...payload}
        }
    },
    getters: {
        current_language(state) {
            return state.language
        },
        env(state) {
            return state.env
        }
    }
}
