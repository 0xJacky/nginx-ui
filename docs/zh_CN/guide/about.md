<script setup>
import { VPTeamMembers } from 'vitepress/theme';

const blogIcon = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" xml:space="preserve"><title>Blog</title><path d="M5 23c-2.2 0-4-1.8-4-4v-8h2v4.5c.6-.3 1.3-.5 2-.5 2.2 0 4 1.8 4 4s-1.8 4-4 4zm0-6c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm19 2h-2C22 9.6 14.4 2 5 2V0c10.5 0 19 8.5 19 19zm-5 0h-2c0-6.6-5.4-12-12-12V5c7.7 0 14 6.3 14 14zm-5 0h-2c0-3.9-3.1-7-7-7v-2c5 0 9 4 9 9z"/></svg>';

const members = [
  {
    avatar: 'https://www.github.com/0xJacky.png',
    name: '0xJacky',
    title: '创始人',
    links: [
      { icon: 'github', link: 'https://github.com/0xJacky' },
      { icon: { svg: blogIcon }, link: 'https://jackyu.cn' }
    ]
  },
  {
    avatar: 'https://www.github.com/Hintay.png',
    name: 'Hintay',
    title: '开发者',
    links: [
      { icon: 'github', link: 'https://github.com/Hintay' },
      { icon: { svg: blogIcon }, link: 'https://blog.kugeek.com' }
    ]
  },
  {
    avatar: 'https://www.github.com/akinoccc.png',
    name: 'Akino',
    title: '开发者',
    links: [
      { icon: 'github', link: 'https://github.com/akinoccc' }
    ]
  },
]
</script>

# 何为 Nginx UI?

![Dashboard](/assets/dashboard_zh_CN.png)

<div class="tip custom-block" style="padding-top: 8px">

想快速试试吗？跳转到 [即刻开始](./getting-started)。

</div>

Nginx UI 是一个全新的 Nginx 网络管理界面，旨在简化 Nginx 单机和集群节点的管理和配置。
它提供实时服务器运行数据、Nginx 性能监控、ChatGPT 助手、支持大模型代码补全的代码编辑器、
一键部署 Let's Encrypt 证书的自动续签以及用户友好的网站配置编辑工具。此外，Nginx UI 还提供了在线访问 Nginx
日志、配置文件的自动测试和重载、网络终端、深色模式和自适应网页设计等功能。
Nginx UI 采用 Go 和 Vue 构建，确保在管理 Nginx 服务器时提供无缝高效的体验。

## 我们的团队

<VPTeamMembers size="small" :members="members" />

## 特色

- 在线查看服务器 CPU、内存、系统负载、磁盘使用率等指标
- 配置修改后会自动备份，可以对比任意版本或恢复到任意版本
- 支持镜像操作到多个集群节点，轻松管理多服务器环境
- 导出加密的 Nginx / Nginx UI 配置，方便快速部署和恢复到新环境
- 增强版在线 ChatGPT 助手，支持多种模型，包括显示 Deepseek-R1 的思考链，帮助您更好地理解和优化配置
- 一键申请和自动续签 Let's encrypt 证书
- 在线编辑 Nginx 配置文件，编辑器支持 **大模型代码补全** 和 Nginx 配置语法高亮
- 在线查看 Nginx 日志
- 使用 Go 和 Vue 开发，发行版本为单个可执行的二进制文件
- 保存配置后自动测试配置文件并重载 Nginx
- 基于网页浏览器的高级命令行终端
- 支持深色模式
- 自适应网页设计

## 可用平台

Nginx UI 可在以下平台中使用：

- macOS 11 Big Sur 及之后版本（amd64 / arm64）
- Linux 2.6.23 及之后版本（x86 / amd64 / arm64 / armv5 / armv6 / armv7）
    - 包括但不限于 Debian 7 / 8、Ubuntu 12.04 / 14.04 及后续版本、CentOS 6 / 7、Arch Linux
- FreeBSD
- OpenBSD
- Dragonfly BSD
- Openwrt

## 国际化

我们官方支持以下语言：

- 英语
- 简体中文
- 繁体中文

由于我们并非英语母语者，尽管已尽力确保准确性，但仍可能存在改进空间。如果您发现任何问题，欢迎向我们反馈！

此外，感谢我们优秀的社区提供了更多语言，欢迎访问 [Weblate](https://weblate.nginxui.com) 进行查看和贡献翻译。

## 构建基于

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
