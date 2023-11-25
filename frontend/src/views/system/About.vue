<script setup lang="ts">
import gettext from '@/gettext'
import logo from '@/assets/img/logo.png'
import version from '@/version.json'
import GithubButton from 'vue-github-button'

const {$gettext} = gettext

const this_year = new Date().getFullYear()
</script>

<template>
  <a-card style="text-align: center" :bordered="false">
    <div class="logo">
      <img :src="logo" alt="logo"/>
    </div>
    <h2>Nginx UI</h2>
    <p>Yet another WebUI for Nginx</p>
    <p>Version: {{ version.version }} ({{ version.build_id || $gettext('Development Mode') }})</p>
    <div class="star-on-github">
      <github-button href="https://github.com/0xJacky/nginx-ui"
                     data-color-scheme="no-preference: light; light: light; dark: dark;" data-size="large"
                     data-show-count="true" aria-label="Star 0xJacky/nginx-ui on GitHub">Star
      </github-button>
    </div>
    <h3 v-translate>Project Team</h3>
    <p><a href="https://jackyu.cn/">@0xJacky</a> <a href="https://blog.kugeek.com/">@Hintay</a></p>
    <h3 v-translate>Build with</h3>
    <p>❤️</p>
    <p>Go</p>
    <p>Gin</p>
    <p>Vue3 + Vite + TypeScript</p>
    <p>Websocket</p>
    <h3 v-translate translate-context="Project">License</h3>
    <p>GNU General Public License v3.0</p>
    <p>Copyright © 2020 - {{ this_year }} Nginx UI </p>
  </a-card>
</template>

<style lang="less" scoped>
.logo {
  img {
    max-width: 120px
  }
}

.egg {
  padding: 10px 0;
}

.ant-btn {
  margin: 10px 10px 0 0;
}

.star-on-github {
  margin-bottom: 10px;
}
</style>
