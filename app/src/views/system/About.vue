<script setup lang="ts">
import logo from '@/assets/img/logo.png'
import ver from '@/version.json'
import GithubButton from '@0xjacky/vue-github-button'

const thisYear = new Date().getFullYear()
</script>

<template>
  <ACard
    class="text-center"
    :bordered="false"
  >
    <div class="logo">
      <img
        :src="logo"
        alt="logo"
      >
    </div>
    <h2>Nginx UI</h2>
    <p>Yet another WebUI for Nginx</p>
    <p>Version: {{ ver.version }} ({{ ver.build_id || $gettext('Development Mode') }})</p>
    <div class="star-on-github">
      <GithubButton
        href="https://github.com/0xJacky/nginx-ui"
        data-color-scheme="no-preference: light; light: light; dark: dark;"
        data-size="large"
        data-show-count="true"
        aria-label="Star 0xJacky/nginx-ui on GitHub"
      >
        Star
      </GithubButton>
    </div>
    <h3>
      {{ $gettext('Project Team') }}
    </h3>
    <p><a href="https://jackyu.cn/">@0xJacky</a> <a href="https://blog.kugeek.com/">@Hintay</a></p>
    <h3>
      {{ $gettext('Build with') }}
    </h3>
    <p>❤️</p>
    <p>Go</p>
    <p>Gin</p>
    <p>Vue3 + Vite + TypeScript</p>
    <p>Websocket</p>
    <h3>
      {{ $gettext('License') }}
    </h3>
    <p>GNU General Public License v3.0</p>
    <p>Copyright © 2021 - {{ thisYear }} Nginx UI Team</p>
  </ACard>
</template>

<style lang="less" scoped>
.logo {
  img {
    max-width: 120px
  }
}

.star-on-github {
  margin-bottom: 10px;
}
</style>
