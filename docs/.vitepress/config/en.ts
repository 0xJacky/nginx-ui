import { LocaleSpecificConfig, DefaultTheme } from 'vitepress'
import { demoUrl } from './common'

export const enConfig: LocaleSpecificConfig<DefaultTheme.Config> = {
  themeConfig: {
    nav: [
      { text: 'Home', link: '/' },
      { text: 'Guide', link: '/guide/about' },
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
            { text: 'Build', link: '/guide/build' },
            { text: 'Project Structure', link: '/guide/project-structure' },
            { text: 'Config Template', link: '/guide/nginx-ui-template' },
            { text: 'Contributing', link: '/guide/contributing' }
          ]
        },
        {
          text: 'Configuration',
          collapsed: false,
          items: [
            { text: 'App', link: '/guide/config-app' },
            { text: 'Server', link: '/guide/config-server' },
            { text: 'Database', link: '/guide/config-database' },
            { text: 'Auth', link: '/guide/config-auth' },
            { text: 'Casdoor', link: '/guide/config-casdoor' },
            { text: 'Cert', link: '/guide/config-cert' },
            { text: 'Cluster', link: '/guide/config-cluster' },
            { text: 'Crypto', link: '/guide/config-crypto' },
            { text: 'Http', link: '/guide/config-http' },
            { text: 'Logrotate', link: '/guide/config-logrotate' },
            { text: 'Nginx', link: '/guide/config-nginx' },
            { text: 'Node', link: '/guide/config-node' },
            { text: 'Open AI', link: '/guide/config-openai' },
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
            { text: 'License', link: '/guide/license' }
          ]
        }
      ]
    }
  }
}
