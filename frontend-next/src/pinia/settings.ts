import { defineStore } from "pinia"

export const useSettingsStore = defineStore('settings', {
    state: () => ({
        language: '',
    }),
    getters: {

    },
    actions: {
        set_language(lang:string) {
            this.language = lang
        },
    },
    persist: true
})
