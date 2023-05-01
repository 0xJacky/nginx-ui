import {defineConfig} from 'vitepress'

// https://vitepress.dev/reference/site-config

function thisYear() {
    return new Date().getFullYear()
}

export default defineConfig({
    lang: 'en-US',
    title: 'Nginx UI',
    description: 'Yet another Nginx Web UI',

    lastUpdated: true,

    locales: {
        root: {
            label: 'English',
            lang: 'en'
        },
        zh_CN: {
            label: '简体中文',
            lang: 'zh_CN'
        }
    },

    themeConfig: {
        // https://vitepress.dev/reference/default-theme-config
        nav: [
            {text: 'Home', link: '/'},
            {text: 'Guide', link: '/guide/about'},
            {text: 'Demo', link: 'https://nginxui.jackyu.cn'}
        ],

        sidebar: sidebar(),

        editLink: {
            pattern: 'https://github.com/0xJacky/nginx-ui/edit/master/frontend/docs/:path'
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

function sidebar() {
    return {
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
        '/zh_CN/guide/': [
            {
                text: '介绍',
                collapsed: false,
                items: [
                    {text: '何为 Nginx UI?', link: '/zh_CN/guide/about'},
                    {text: '即刻开始', link: '/zh_CN/guide/getting-started'},
                    {text: 'Nginx 代理示例', link: '/zh_CN/guide/nginx-proxy-example'},
                    {text: '贡献代码', link: '/zh_CN/guide/contributing'},
                    {text: '开源协议', link: '/zh_CN/guide/license'}
                ]
            },
            {
                text: '配置',
                collapsed: false,
                items: [
                    {text: '服务端', link: '/zh_CN/guide/config-server'},
                    {text: 'Nginx 日志', link: '/zh_CN/guide/config-nginx-log'},
                    {text: 'Open AI', link: '/zh_CN/guide/config-openai'}
                ]
            }
        ]
    }
}
