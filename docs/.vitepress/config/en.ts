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
                        {text: 'Nginx Log', link: '/guide/config-nginx-log'},
                        {text: 'Open AI', link: '/guide/config-openai'}
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
            ],
        },
    }
}
