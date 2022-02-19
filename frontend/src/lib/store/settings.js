export const settings = {
    namespace: true,
    state: {
        language: '',
        translations: {},
    },
    mutations: {
        set_language(state, payload) {
            state.language = payload
        },
        update_translations(state, payload) {
            state.translations = payload
        }
    },
    actions: {
        set_language({commit}, data) {
            commit('set_language', data)
        },
        update_translations({commit}, data) {
            commit('update_translations', data)
        }
    },
    getters: {
        current_language(state) {
            return state.language
        }
    }
}
