export interface TerminalTab {
  id: string
  name: string
  isActive: boolean
  created: Date
}

export const useTerminalStore = defineStore('terminal', {
  state: () => ({
    llm_panel_visible: false,
    tabs: [] as TerminalTab[],
    activeTabId: null as string | null,
  }),
  getters: {
    activeTab(): TerminalTab | undefined {
      return this.tabs.find(tab => tab.id === this.activeTabId)
    },
    hasActiveTabs(): boolean {
      return this.tabs.length > 0
    },
  },
  actions: {
    set_llm_panel_visible(visible: boolean) {
      this.llm_panel_visible = visible
    },
    toggle_llm_panel() {
      this.llm_panel_visible = !this.llm_panel_visible
    },
    createTab(): TerminalTab {
      // Find the smallest available ID starting from 1
      const usedNumbers = new Set(
        this.tabs.map(tab => {
          const match = tab.id.match(/terminal-(\d+)/)
          return match ? Number.parseInt(match[1]) : 0
        }).filter(num => num > 0),
      )

      let tabNumber = 1
      while (usedNumbers.has(tabNumber)) {
        tabNumber++
      }

      const tab: TerminalTab = {
        id: `terminal-${tabNumber}`,
        name: `Terminal ${tabNumber}`,
        isActive: false,
        created: new Date(),
      }
      this.tabs.push(tab)
      this.setActiveTab(tab.id)
      return tab
    },
    setActiveTab(tabId: string) {
      this.tabs.forEach(tab => {
        tab.isActive = tab.id === tabId
      })
      this.activeTabId = tabId
    },
    closeTab(tabId: string) {
      const tabIndex = this.tabs.findIndex(tab => tab.id === tabId)
      if (tabIndex === -1)
        return

      const wasActive = this.tabs[tabIndex].isActive
      this.tabs.splice(tabIndex, 1)

      if (wasActive && this.tabs.length > 0) {
        const newActiveIndex = Math.min(tabIndex, this.tabs.length - 1)
        this.setActiveTab(this.tabs[newActiveIndex].id)
      }
      else if (this.tabs.length === 0) {
        this.activeTabId = null
      }
    },
    renameTab(tabId: string, newName: string) {
      const tab = this.tabs.find(tab => tab.id === tabId)
      if (tab) {
        tab.name = newName
      }
    },
    initializeFirstTab() {
      if (this.tabs.length === 0) {
        this.createTab()
      }
    },
  },
  persist: [
    {
      storage: localStorage,
      pick: ['llm_panel_visible'],
    },
  ],
})
