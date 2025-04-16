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
{
    avatar: 'https://www.github.com/akinoccc.png',
    name: 'Akino',
    title: '開發者',
    links: [
      { icon: 'github', link: 'https://github.com/akinoccc' }
    ]
  },
]
</script>

# 何為 Nginx UI?

![Dashboard](/assets/dashboard_zh_CN.png)

<div class="tip custom-block" style="padding-top: 8px">

想快速試試嗎？跳轉到 [即刻開始](./getting-started)。

</div>

Nginx UI 是一個全新的 Nginx 網路管理介面，目的是簡化 Nginx 伺服器的管理和設定。它提供即時伺服器統計資料、ChatGPT
助手、一鍵部署、Let's Encrypt 證書的自動續簽以及使用者友好的網站設定編輯工具。此外，Nginx UI 還提供了線上存取 Nginx
日誌、設定檔案的自動測試和過載、網路終端、深色模式和自適應網頁設計等功能。Nginx UI 採用 Go 和 Vue 建構，確保在管理 Nginx
伺服器時提供無縫高效的體驗。

## 我們的團隊

<VPTeamMembers size="small" :members="members" />

## 特色

- 線上檢視伺服器 CPU、記憶體、系統負載、磁碟使用率等指標
- 設定修改後會自動備份，可以對比任意版本或恢復到任意版本
- 支援鏡像操作到多個叢集節點，輕鬆管理多伺服器環境
- 匯出加密的 Nginx/NginxUI 設定，方便快速部署和恢復到新環境
- 增強版線上 ChatGPT 助手，支援多種模型，包括顯示 Deepseek-R1 的思考鏈，幫助您更好地理解和最佳化設定
- 一鍵申請和自動續簽 Let's encrypt 憑證
- 線上編輯 Nginx 配置檔案，編輯器支援 **大模型代碼補全** 和 Nginx 配置語法突顯
- 線上檢視 Nginx 日誌
- 使用 Go 和 Vue 開發，發行版本為單個可執行檔案
- 儲存設定後自動測試設定檔案並過載 Nginx
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

我們目前官方支援以下語言：

- 英文
- 簡體中文
- 正體中文

由於我們並非英文母語者，儘管已盡力確保準確性，仍可能有改進的空間。若您發現任何問題，歡迎提供回饋！

此外，感謝熱心的社群貢獻更多語言支援，歡迎前往 [Weblate](https://weblate.nginxui.com) 瀏覽並參與翻譯，共同打造更完善的多語言體驗！

## 建構基於

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
- [lego](https://github.com/go-acme/lego)
