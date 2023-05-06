import { defineConfig } from 'vitepress'
import { sharedConfig } from './shared'
import { enConfig } from "./en"
import { zhCNConfig } from "./zh_CN"
import { zhTWConfig } from "./zh_TW";

export default defineConfig({
    ...sharedConfig,
    locales: {
        root: { label: 'English', lang: 'en', ...enConfig },
        zh_CN: { label: '简体中文', lang: 'zh-CN', ...zhCNConfig },
        zh_TW: { label: '繁體中文', lang: 'zh-TW', ...zhTWConfig }
    }
})
