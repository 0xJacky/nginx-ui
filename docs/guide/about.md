<script setup>
import { VPTeamMembers } from 'vitepress/theme';

const blogIcon = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" xml:space="preserve"><title>Blog</title><path d="M5 23c-2.2 0-4-1.8-4-4v-8h2v4.5c.6-.3 1.3-.5 2-.5 2.2 0 4 1.8 4 4s-1.8 4-4 4zm0-6c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm19 2h-2C22 9.6 14.4 2 5 2V0c10.5 0 19 8.5 19 19zm-5 0h-2c0-6.6-5.4-12-12-12V5c7.7 0 14 6.3 14 14zm-5 0h-2c0-3.9-3.1-7-7-7v-2c5 0 9 4 9 9z"/></svg>';

const members = [
  {
    avatar: 'https://www.github.com/0xJacky.png',
    name: '0xJacky',
    title: 'Creator',
    links: [
      { icon: 'github', link: 'https://github.com/0xJacky' },
      { icon: { svg: blogIcon }, link: 'https://jackyu.cn' }
    ]
  },
{
    avatar: 'https://www.github.com/Hintay.png',
    name: 'Hintay',
    title: 'Developer',
    links: [
      { icon: 'github', link: 'https://github.com/Hintay' },
      { icon: { svg: blogIcon }, link: 'https://blog.kugeek.com' }
    ]
  },
]
</script>

# What is Nginx UI?

![Dashboard](/assets/dashboard_en.png)

<div class="tip custom-block" style="padding-top: 8px">

Just want to try it out? Skip to the [Quickstart](./getting-started).

</div>

Nginx UI is a comprehensive web-based interface designed to simplify the management and configuration of Nginx servers.
It offers real-time server statistics, AI-powered ChatGPT assistance, one-click deployment, automatic renewal of Let's
Encrypt certificates, and user-friendly editing tools for website configurations. Additionally, Nginx UI provides
features such as online access to Nginx logs, automatic testing and reloading of configuration files, a web terminal,
dark mode, and responsive web design. Built with Go and Vue, Nginx UI ensures a seamless and efficient experience for
managing your Nginx server.

## Our Team

<VPTeamMembers size="small" :members="members" />

## Features

- Online statistics for server indicators such as CPU usage, memory usage, load average, and disk usage.
- Online ChatGPT Assistant.
- One-click deployment and automatic renewal Let's Encrypt certificates.
- Online editing websites configurations with our self-designed **NgxConfigEditor** which is a user-friendly block
  editor for nginx configurations, or **Ace Code Editor** which supports highlighting nginx configuration syntax.
- Online view Nginx logs.
- Written in Go and Vue, distribution is a single executable binary.
- Automatically test configuration file and reload nginx after saving configuration.
- Web Terminal.
- Dark Mode.
- Responsive Web Design.

## Available Platforms

Nginx UI is available on the following platforms:

- macOS 11 Big Sur and later (amd64 / arm64)
- Linux 2.6.23 and later (x86 / amd64 / arm64 / armv5 / armv6 / armv7)
    - Including but not limited to Debian 7 / 8, Ubuntu 12.04 / 14.04 and later, CentOS 6 / 7, Arch Linux
- FreeBSD
- OpenBSD
- Dragonfly BSD
- Openwrt

## Internationalization

- English
- Simplified Chinese
- Traditional Chinese

We welcome translations into any language.

## Built With

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
