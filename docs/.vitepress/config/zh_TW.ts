import {LocaleSpecificConfig, DefaultTheme} from 'vitepress'
import {demoUrl, editLinkPattern} from './common'

export const zhTWConfig: LocaleSpecificConfig<DefaultTheme.Config> = {
  themeConfig: {
    nav: [
      {text: '首頁', link: '/zh_TW/'},
      {text: '手冊', link: '/zh_TW/guide/about'},
      {text: '演示', link: demoUrl}
    ],

    editLink: {
      text: '編輯此頁',
      pattern: editLinkPattern
    },

    sidebar: {
      '/zh_TW/guide/': [
        {
          text: '介紹',
          collapsed: false,
          items: [
            {text: '何為 Nginx UI?', link: '/zh_TW/guide/about'},
            {text: '即刻開始', link: '/zh_TW/guide/getting-started'},
            {text: '安裝指令碼', link: '/zh_TW/guide/install-script-linux'}
          ]
        },
        {
          text: '開發',
          collapsed: false,
          items: [
            {text: '構建', link: '/zh_TW/guide/build'},
            {text: '專案結構', link: '/zh_TW/guide/project-structure'},
            {text: '貢獻程式碼', link: '/zh_TW/guide/contributing'}
          ]
        },
        {
          text: '配置',
          collapsed: false,
          items: [
            {text: '服務端', link: '/zh_TW/guide/config-server'},
            {text: 'Nginx', link: '/zh_TW/guide/config-nginx'},
            {text: 'Open AI', link: '/zh_TW/guide/config-openai'},
            {text: 'Casdoor', link: '/zh_TW/guide/config-casdoor'},
            {text: 'Logrotate', link: '/zh_TW/guide/config-logrotate'},
            {text: '集群', link: '/zh_TW/guide/config-cluster'}
          ]
        },
        {
          text: '附錄',
          collapsed: false,
          items: [
            {text: 'Nginx 代理示例', link: '/zh_TW/guide/nginx-proxy-example'},
            {text: '開源協議', link: '/zh_TW/guide/license'}
          ]
        }
      ]
    },

    docFooter: {
      prev: '上一頁',
      next: '下一頁'
    },
    returnToTopLabel: '返回頂部',
    outline: {
      label: '導航欄'
    },
    darkModeSwitchLabel: '外觀',
    sidebarMenuLabel: '歸檔',
    lastUpdated: {
      text: '更新於'
    },

    search: {
      provider: 'local',
      options: {
        locales: {
          zh_TW: {
            translations: {
              button: {
                buttonText: '搜尋文件',
                buttonAriaLabel: '搜尋文件'
              },
              modal: {
                noResultsText: '無法找到相關結果',
                resetButtonTitle: '清除查詢條件',
                footer: {
                  selectText: '選擇',
                  navigateText: '切換',
                  closeText: '關閉'
                }
              }
            }
          }
        }
      }
    }
  }
}
