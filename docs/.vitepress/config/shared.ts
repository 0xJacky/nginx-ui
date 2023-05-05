import { defineConfig } from 'vitepress'

function thisYear() {
    return new Date().getFullYear()
}

export const sharedConfig = defineConfig({
    title: 'Nginx UI',
    description: 'Yet another Nginx Web UI',

    lastUpdated: true,

    themeConfig: {
        logo: '/logo.svg',

        search: {
            provider: 'local'
        },

        editLink: {
            pattern: 'https://github.com/0xJacky/nginx-ui/edit/master/docs/:path'
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
