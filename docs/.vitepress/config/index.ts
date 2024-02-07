import { defineConfig } from 'vitepress'
import { sharedConfig } from './shared'
import locales from './locales'

export default defineConfig({
    ...sharedConfig,
    locales
})
