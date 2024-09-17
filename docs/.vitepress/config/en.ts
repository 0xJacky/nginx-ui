import {LocaleSpecificConfig, DefaultTheme} from 'vitepress'
import {demoUrl} from './common'

export const enConfig: LocaleSpecificConfig<DefaultTheme.Config> = {
  themeConfig: {
    nav: [
      {text: 'Home', link: '/'},
      {text: 'Guide', link: '/guide/about'},
      {text: 'Demo', link: demoUrl}
    ],

    sidebar: {
      '/guide/': [
        {
          text: 'Introduction',
          collapsed: false,
          items: [
            {text: 'What is Nginx UI?', link: '/guide/about'},
            {text: 'Getting Started', link: '/guide/getting-started'},
            {text: 'Install Script', link: '/guide/install-script-linux'}
          ]
        },
        {
          text: 'Development',
          collapsed: false,
          items: [
            {text: 'Build', link: '/guide/build'},
            {text: 'Project Structure', link: '/guide/project-structure'},
            {text: 'Config Template', link: '/guide/nginx-ui-template'},
            {text: 'Contributing', link: '/guide/contributing'}
          ]
        },
        {
          text: 'Configuration',
          collapsed: false,
          items: [
            {text: 'Server', link: '/guide/config-server'},
            {text: 'Nginx', link: '/guide/config-nginx'},
            {text: 'Open AI', link: '/guide/config-openai'},
            {text: 'Casdoor', link: '/guide/config-casdoor'},
            {text: 'Logrotate', link: '/guide/config-logrotate'},
            {text: 'Cluster', link: '/guide/config-cluster'},
            {text: 'Auth', link: '/guide/config-auth'},
            {text: 'Crypto', link: '/guide/config-crypto'},
            {text: 'Webauthn', link: '/guide/config-webauthn'}
          ]
        },
        {
          text: 'Environment Variables',
          collapsed: false,
          items: [
            {text: 'Reference', link: '/guide/env'},
          ]
        },
        {
          text: 'Appendix',
          collapsed: false,
          items: [
            {text: 'Nginx Proxy Example', link: '/guide/nginx-proxy-example'},
            {text: 'License', link: '/guide/license'}
          ]
        }
      ]
    }
  }
}
