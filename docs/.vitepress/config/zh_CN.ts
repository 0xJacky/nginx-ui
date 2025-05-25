import { LocaleSpecificConfig, DefaultTheme } from 'vitepress'
import { demoUrl, editLinkPattern } from './common'

export const zhCNConfig: LocaleSpecificConfig<DefaultTheme.Config> = {
  themeConfig: {
    nav: [
      { text: '首页', link: '/zh_CN/' },
      { text: '手册', link: '/zh_CN/guide/about' },
      { text: '演示', link: demoUrl }
    ],

    editLink: {
      text: '编辑此页',
      pattern: editLinkPattern
    },

    sidebar: {
      '/zh_CN/guide/': [
        {
          text: '介绍',
          collapsed: false,
          items: [
            { text: '何为 Nginx UI?', link: '/zh_CN/guide/about' },
            { text: '即刻开始', link: '/zh_CN/guide/getting-started' },
            { text: '安装脚本', link: '/zh_CN/guide/install-script-linux' }
          ]
        },
        {
          text: '开发',
          collapsed: false,
          items: [
            { text: '开发容器', link: '/zh_CN/guide/devcontainer' },
            { text: '构建', link: '/zh_CN/guide/build' },
            { text: '项目结构', link: '/zh_CN/guide/project-structure' },
            { text: '配置模板', link: '/zh_CN/guide/nginx-ui-template' },
            { text: '贡献代码', link: '/zh_CN/guide/contributing' }
          ]
        },
        {
          text: 'MCP',
          collapsed: false,
          items: [
            { text: '概述', link: '/zh_CN/guide/mcp' },
            { text: '配置文件管理', link: '/zh_CN/guide/mcp-config' },
            { text: 'Nginx 服务管理', link: '/zh_CN/guide/mcp-nginx' },
          ]
        },
        {
          text: '配置',
          collapsed: false,
          items: [
            { text: 'App', link: '/zh_CN/guide/config-app' },
            { text: 'Auth', link: '/zh_CN/guide/config-auth' },
            { text: 'Backup', link: '/zh_CN/guide/config-backup' },
            { text: 'Casdoor', link: '/zh_CN/guide/config-casdoor' },
            { text: 'Cert', link: '/zh_CN/guide/config-cert' },
            { text: 'Cluster', link: '/zh_CN/guide/config-cluster' },
            { text: 'Crypto', link: '/zh_CN/guide/config-crypto' },
            { text: 'Database', link: '/zh_CN/guide/config-database' },
            { text: 'Http', link: '/zh_CN/guide/config-http' },
            { text: 'Logrotate', link: '/zh_CN/guide/config-logrotate' },
            { text: 'Nginx', link: '/zh_CN/guide/config-nginx' },
            { text: 'Node', link: '/zh_CN/guide/config-node' },
            { text: 'Open AI', link: '/zh_CN/guide/config-openai' },
            { text: 'Server', link: '/zh_CN/guide/config-server' },
            { text: 'Terminal', link: '/zh_CN/guide/config-terminal' },
            { text: 'Webauthn', link: '/zh_CN/guide/config-webauthn' }
          ]
        },
        {
          text: '环境变量',
          collapsed: false,
          items: [
            { text: '参考手册', link: '/zh_CN/guide/env' },
          ]
        },
        {
          text: '附录',
          collapsed: false,
          items: [
            { text: 'Nginx 代理示例', link: '/zh_CN/guide/nginx-proxy-example' },
            { text: '重置密码', link: '/zh_CN/guide/reset-password' },
            { text: '开源协议', link: '/zh_CN/guide/license' }
          ]
        }
      ]
    },

    docFooter: {
      prev: '上一页',
      next: '下一页'
    },
    returnToTopLabel: '返回顶部',
    outline: {
      label: '导航栏'
    },
    darkModeSwitchLabel: '外观',
    sidebarMenuLabel: '归档',
    lastUpdated: {
      text: '更新于'
    },
    search: {
      provider: 'local',
      options: {
        locales: {
          zh_CN: {
            translations: {
              button: {
                buttonText: '搜索文档',
                buttonAriaLabel: '搜索文档'
              },
              modal: {
                noResultsText: '无法找到相关结果',
                resetButtonTitle: '清除查询条件',
                footer: {
                  selectText: '选择',
                  navigateText: '切换',
                  closeText: '关闭'
                }
              }
            }
          }
        }
      }
    }
  }
}
