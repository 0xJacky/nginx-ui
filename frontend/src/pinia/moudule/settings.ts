import {defineStore} from 'pinia'

export const useSettingsStore = defineStore('settings', {
    state: () => ({
        language: '',
        theme: 'light',
        preference_theme: 'auto',
        environment: {
            id: 0,
            name: 'Local'
        }
    }),
    getters: {},
    actions: {
        set_language(lang: string) {
            this.language = lang
        },
        set_theme(t: string) {
            this.theme = t
        },
        set_preference_theme(t: string) {
            this.preference_theme = t
        },
        clear_environment() {
            this.environment.id = 0
            this.environment.name = 'Local'
        }
    },
    persist: true
})
