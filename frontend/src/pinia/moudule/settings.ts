import {defineStore} from 'pinia'

export const useSettingsStore = defineStore('settings', {
    state: () => ({
        language: '',
        theme: 'light',
        preference_theme: 'auto'
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
        }
    },
    persist: true
})
