---
# https://vitepress.dev/reference/default-theme-home-page
layout: home

title: Nginx UI
titleTemplate: Yet another Nginx Web UI

hero:
    name: "Nginx UI"
    text: "Nginx 网络管理界面的新选择"
    tagline: 智能、强大、高速
    image:
        src: /assets/icon.svg
        alt: Nginx UI
    actions:
        -   theme: brand
            text: 即刻开始
            link: /zh_CN/guide/about
        -   theme: alt
            text: 在 Github 上查看
            link: https://github.com/0xJacky/nginx-ui

features:
    -   icon: 📊
        title: 服务器指标在线统计
        details: 实时监控 CPU 使用率、内存使用率、平均负载和磁盘使用情况。
    -   icon: 💾
        title: 配置文件自动备份
        details: 配置修改后会自动备份，可以对比任意版本或恢复到任意版本。
    -   icon: 🔄
        title: 集群管理
        details: 支持镜像操作到多个集群节点，轻松管理多服务器环境。
    -   icon: 📤
        title: 导出加密配置
        details: 导出加密的 Nginx / Nginx UI 配置，方便快速部署和恢复到新环境。
    -   icon: 💬
        title: 增强版在线 ChatGPT 助手
        details: 支持多种模型，包括显示 Deepseek-R1 的思考链，帮助您更好地理解和优化配置。
    -   icon: 🔍
        title: 代码补全
        details: 代码编辑器支持代码补全，帮助您更快地编写配置。
    -   icon: 🤖
        title: MCP (Model Context Protocol)
        details: 提供特殊接口让 AI 代理与 Nginx UI 交互，实现自动化配置管理和服务控制。
    -   icon: 🖱️
        title: 一键部署和自动续期
        details: 只需一键即可轻松部署和自动续期 Let's Encrypt 证书。
    -   icon: 🛠️
        title: 在线编辑网站配置
        details: 使用我们的 NgxConfigEditor 区块编辑器或带有 Nginx 语法高亮的 Ace 代码编辑器编辑配置。
    -   icon: 📜
        title: 在线查看 Nginx 日志
        details: 在线查看和实时跟踪 Nginx 日志，按条件搜索，并通过流量面板和访客地图查看访问情况。
    -   icon: 🧩
        title: 插件系统
        details: 从插件市场、上传的文件或离线目录安装带签名的插件来扩展功能，例如官方 DNS-01 插件。
    -   icon: 💻
        title: 使用 Go 和 Vue 编写
        details: 平台使用 Go 和 Vue 构建，并作为单个可执行二进制文件分发。
    -   icon: 🔄
        title: 自动测试和重新加载配置
        details: 测试配置文件并在保存更改后自动重新加载 Nginx。
    -   icon: 🖥️
        title: Web 终端
        details: 访问基于 Web 的终端以便于管理。
    -   icon: 🌙
        title: 暗黑模式
        details: 启用暗黑模式以获得舒适的用户体验。
    -   icon: 📱
        title: 响应式网页设计
        details: 通过响应式网页设计在任何设备上享受无缝体验。
    -   icon: 🔐
        title: 双因素认证
        details: 使用双因素认证保护敏感操作。

---

