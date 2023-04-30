import {defineConfig} from 'vitepress'

// https://vitepress.dev/reference/site-config

function thisYear() {
    return new Date().getFullYear()
}

export default defineConfig({
    lang: 'en-US',
    title: 'Nginx UI',
    description: 'Yet another Nginx Web UI',
    themeConfig: {
        // https://vitepress.dev/reference/default-theme-config
        nav: [
            {text: 'Home', link: '/'},
            {text: 'Guide', link: '/guide/about'},
            {text: 'Demo', link: 'https://nginxui.jackyu.cn'}
        ],

        sidebar: {
            '/guide/': [
                {
                    text: 'Introduction',
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
                    items: [
                        {text: 'Server', link: '/guide/config-server'},
                        {text: 'Nginx Log', link: '/guide/config-nginx-log'},
                        {text: 'Open AI', link: '/guide/config-openai'}
                    ]
                }
            ]
        },

        search: {
            provider: 'local'
        },

        footer: {
            message: 'Released under the AGPL-3.0 License.',
            copyright: 'Copyright © 2021-' + thisYear() + ' Nginx UI Team'
        },

        socialLinks: [
            {icon: 'github', link: 'https://github.com/0xJacky/nginx-ui'}
        ]
    }
})
