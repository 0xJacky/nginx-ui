import { defineConfig } from 'vitepress'
import { projectUrl, editLinkPattern } from './common'

export const commitRef = process.env.COMMIT_REF ?
    `<a href="${projectUrl}/commit/${process.env.COMMIT_REF}">` + process.env.COMMIT_REF.slice(0, 8) + '</a>' :
    'dev'

function thisYear() {
    return new Date().getFullYear()
}

export const sharedConfig = defineConfig({
    title: 'Nginx UI',
    description: 'Yet another Nginx Web UI',

    head: [
        ['link', { rel: 'icon', type: 'image/svg+xml', href: '/assets/logo.svg' }],
        ['meta', { name: 'theme-color', content: '#3682D8' }]
    ],

    lastUpdated: true,

    themeConfig: {
        logo: '/assets/logo.svg',

        search: {
            provider: 'local'
        },

        editLink: {
            pattern: editLinkPattern
        },

        footer: {
            message: `Released under the AGPL-3.0 License. (${commitRef})`,
            copyright: 'Copyright © 2021-' + thisYear() + ' Nginx UI Team'
        },

        socialLinks: [
            { icon: 'github', link: projectUrl }
        ]
    },

    vite: {
        server: {
            port: Number.parseInt(process.env.VITE_PORT ?? '3003')
        }
    },

    ignoreDeadLinks: true
})
