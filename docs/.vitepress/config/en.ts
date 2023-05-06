import {LocaleSpecificConfig, DefaultTheme} from "vitepress"
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
                        {text: 'Nginx Proxy Example', link: '/guide/nginx-proxy-example'},
                        {text: 'Contributing', link: '/guide/contributing'},
                        {text: 'License', link: '/guide/license'}
                    ]
                },
                {
                    text: 'Configuration',
                    collapsed: false,
                    items: [
                        {text: 'Server', link: '/guide/config-server'},
                        {text: 'Nginx Log', link: '/guide/config-nginx-log'},
                        {text: 'Open AI', link: '/guide/config-openai'}
                    ]
                }
            ],
        },
    }
}
