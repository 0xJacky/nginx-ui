<script setup>
import { VPTeamMembers } from 'vitepress/theme'

const members = [
  {
    avatar: 'https://www.github.com/0xJacky.png',
    name: '0xJacky',
    title: '创始人',
    links: [
      { icon: 'github', link: 'https://github.com/0xJacky' },
    ]
  },
{
    avatar: 'https://www.github.com/Hintay.png',
    name: 'Hintay',
    title: '开发者',
    links: [
      { icon: 'github', link: 'https://github.com/Hintay' },
    ]
  },
]
</script>

# 何为 Nginx UI?

![Dashboard](/dashboard_zh_CN.png)

Nginx UI 是一个全新的 Nginx 网络管理界面，旨在简化 Nginx 服务器的管理和配置。它提供实时服务器统计数据、ChatGPT
助手、一键部署、Let's Encrypt 证书的自动续签以及用户友好的网站配置编辑工具。此外，Nginx UI 还提供了在线访问 Nginx
日志、配置文件的自动测试和重载、网络终端、深色模式和自适应网页设计等功能。Nginx UI 采用 Go 和 Vue 构建，确保在管理 Nginx
服务器时提供无缝高效的体验。

## 我们的团队

<VPTeamMembers size="small" :members="members" />

## 在线预览

网址：[https://nginxui.jackyu.cn](https://nginxui.jackyu.cn)

- 用户名：admin
- 密码：admin

## 特色

- 在线查看服务器 CPU、内存、系统负载、磁盘使用率等指标
- 在线 ChatGPT 助理
- 一键申请和自动续签 Let's encrypt 证书
- 在线编辑 Nginx 配置文件，编辑器支持 Nginx 配置语法高亮
- 在线查看 Nginx 日志
- 使用 Go 和 Vue 开发，发行版本为单个可执行的二进制文件
- 保存配置后自动测试配置文件并重载 Nginx
- 基于网页浏览器的高级命令行终端
- 支持深色模式
- 自适应网页设计

## 国际化

- 英语
- 简体中文
- 繁体中文

我们欢迎您将项目翻译成任何语言。

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
