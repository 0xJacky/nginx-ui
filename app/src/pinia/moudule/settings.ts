import { defineStore } from 'pinia'
import gettext from '@/gettext'

export const useSettingsStore = defineStore('settings', {
  state: () => ({
    language: '',
    theme: 'light',
    preference_theme: 'auto',
    node: {
      id: 0,
      name: 'Local',
    },
    server_name: '',
    route_path: '',
  }),
  getters: {
    is_remote(): boolean {
      return this.node.id !== 0
    },
  },
  actions: {
    set_language(lang: string) {
      this.language = lang
      gettext.current = lang
    },
    set_theme(t: string) {
      this.theme = t
      document.body.setAttribute('class', t === 'dark' ? 'dark' : 'light')
    },
    set_preference_theme(t: string) {
      this.preference_theme = t
    },
    clear_node() {
      this.node.id = 0
      this.node.name = 'Local'
    },
  },
  persist: [
    {
      key: `LOCAL_${window.name || 'main'}`,
      storage: localStorage,
      pick: ['environment', 'server_name', 'route_path'],
    },
    {
      storage: localStorage,
      pick: ['language', 'theme', 'preference_theme'],
    },
  ],
})
