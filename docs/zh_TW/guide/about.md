<script setup>
import { VPTeamMembers } from 'vitepress/theme';

const blogIcon = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" xml:space="preserve"><title>Blog</title><path d="M5 23c-2.2 0-4-1.8-4-4v-8h2v4.5c.6-.3 1.3-.5 2-.5 2.2 0 4 1.8 4 4s-1.8 4-4 4zm0-6c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm19 2h-2C22 9.6 14.4 2 5 2V0c10.5 0 19 8.5 19 19zm-5 0h-2c0-6.6-5.4-12-12-12V5c7.7 0 14 6.3 14 14zm-5 0h-2c0-3.9-3.1-7-7-7v-2c5 0 9 4 9 9z"/></svg>';

const members = [
  {
    avatar: 'https://www.github.com/0xJacky.png',
    name: '0xJacky',
    title: '創始人',
    links: [
      { icon: 'github', link: 'https://github.com/0xJacky' },
      { icon: { svg: blogIcon }, link: 'https://jackyu.cn' }
    ]
  },
{
    avatar: 'https://www.github.com/Hintay.png',
    name: 'Hintay',
    title: '開發者',
    links: [
      { icon: 'github', link: 'https://github.com/Hintay' },
      { icon: { svg: blogIcon }, link: 'https://blog.kugeek.com' }
    ]
  },
]
</script>

# 何為 Nginx UI?

![Dashboard](/assets/dashboard_zh_CN.png)

<div class="tip custom-block" style="padding-top: 8px">

想快速試試嗎？跳轉到 [即刻開始](./getting-started)。

</div>

Nginx UI 是一個全新的 Nginx 網路管理介面，旨在簡化 Nginx 伺服器的管理和配置。它提供實時伺服器統計資料、ChatGPT
助手、一鍵部署、Let's Encrypt 證書的自動續簽以及使用者友好的網站配置編輯工具。此外，Nginx UI 還提供了線上訪問 Nginx
日誌、配置檔案的自動測試和過載、網路終端、深色模式和自適應網頁設計等功能。Nginx UI 採用 Go 和 Vue 構建，確保在管理 Nginx
伺服器時提供無縫高效的體驗。

## 我們的團隊

<VPTeamMembers size="small" :members="members" />

## 特色

- 線上檢視伺服器 CPU、記憶體、系統負載、磁碟使用率等指標
- 線上 ChatGPT 助理
- 一鍵申請和自動續簽 Let's encrypt 憑證
- 線上編輯 Nginx 配置檔案，編輯器支援 Nginx 配置語法突顯
- 線上檢視 Nginx 日誌
- 使用 Go 和 Vue 開發，發行版本為單個可執行檔案
- 儲存配置後自動測試配置檔案並重載 Nginx
- 基於網頁瀏覽器的高階命令列終端
- 支援暗黑模式
- 自適應網頁設計

## 可用作業系統

Nginx UI 可在以下作業系統中使用：

- macOS 11 Big Sur 及之後版本（amd64 / arm64）
- Linux 2.6.23 及之後版本（x86 / amd64 / arm64 / armv5 / armv6 / armv7）
    - 包括但不限於 Debian 7 / 8、Ubuntu 12.04 / 14.04 及後續版本、CentOS 6 / 7、Arch Linux
- FreeBSD
- OpenBSD
- Dragonfly BSD
- Openwrt

## 國際化

- 英語
- 簡體中文
- 繁體中文

我們歡迎您將專案翻譯成任何語言。

## 構建基於

- [The Go Programming Language](https://go.dev)
- [Gin Web Framework](https://gin-gonic.com)
- [GORM](http://gorm.io)
- [Vue 3](https://v3.vuejs.org)
- [Vite](https://vitejs.dev)
- [TypeScript](https://www.typescriptlang.org/)
- [Ant Design Vue](https://antdv.com)
- [vue3-gettext](https://github.com/jshmrtn/vue3-gettext)
- [vue3-ace-editor](https://github.com/CarterLi/vue3-ace-editor)
- [Gonginx](https://github.com/tufanbarisyildirim/gonginx)
