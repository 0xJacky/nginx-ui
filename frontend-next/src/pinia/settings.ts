import {defineStore} from "pinia"

export const useSettingsStore = defineStore('settings', {
    state: () => ({
        language: '',
        theme: 'light',
    }),
    getters: {},
    actions: {
        set_language(lang: string) {
            this.language = lang
        },
        set_theme(t: string) {
            this.theme = t
        }
    },
    persist: true
})
