import { LocaleSpecificConfig, DefaultTheme } from 'vitepress'
import { demoUrl } from './common'

export const enConfig: LocaleSpecificConfig<DefaultTheme.Config> = {
  themeConfig: {
    nav: [
      { text: 'Home', link: '/' },
      { text: 'Guide', link: '/guide/about' },
      { text: 'Sponsor', link: '/sponsor' },
      { text: 'Demo', link: demoUrl }
    ],

    sidebar: {
      '/guide/': [
        {
          text: 'Introduction',
          collapsed: false,
          items: [
            { text: 'What is Nginx UI?', link: '/guide/about' },
            { text: 'Getting Started', link: '/guide/getting-started' },
            { text: 'Install Script', link: '/guide/install-script-linux' }
          ]
        },
        {
          text: 'Development',
          collapsed: false,
          items: [
            { text: 'Devcontainer', link: '/guide/devcontainer' },
            { text: 'Build', link: '/guide/build' },
            { text: 'Project Structure', link: '/guide/project-structure' },
            { text: 'Config Template', link: '/guide/nginx-ui-template' },
            { text: 'Contributing', link: '/guide/contributing' }
          ]
        },
        {
          text: 'MCP',
          collapsed: false,
          items: [
            { text: 'Overview', link: '/guide/mcp' },
            { text: 'Configuration Management', link: '/guide/mcp-config' },
            { text: 'Nginx Service Management', link: '/guide/mcp-nginx' },
          ]
        },
        {
          text: 'Configuration',
          collapsed: false,
          items: [
            { text: 'App', link: '/guide/config-app' },
            { text: 'Auth', link: '/guide/config-auth' },
            { text: 'Backup', link: '/guide/config-backup' },
            { text: 'Casdoor', link: '/guide/config-casdoor' },
            { text: 'Cert', link: '/guide/config-cert' },
            { text: 'Cluster', link: '/guide/config-cluster' },
            { text: 'Crypto', link: '/guide/config-crypto' },
            { text: 'Database', link: '/guide/config-database' },
            { text: 'Http', link: '/guide/config-http' },
            { text: 'Logrotate', link: '/guide/config-logrotate' },
            { text: 'Nginx', link: '/guide/config-nginx' },
            { text: 'Node', link: '/guide/config-node' },
            { text: 'Open AI', link: '/guide/config-openai' },
            { text: 'Server', link: '/guide/config-server' },
            { text: 'Terminal', link: '/guide/config-terminal' },
            { text: 'Webauthn', link: '/guide/config-webauthn' }
          ]
        },
        {
          text: 'Environment Variables',
          collapsed: false,
          items: [
            { text: 'Reference', link: '/guide/env' },
          ]
        },
        {
          text: 'Appendix',
          collapsed: false,
          items: [
            { text: 'Nginx Proxy Example', link: '/guide/nginx-proxy-example' },
            { text: 'Reset Password', link: '/guide/reset-password' },
            { text: 'License', link: '/guide/license' }
          ]
        }
      ]
    }
  }
}
