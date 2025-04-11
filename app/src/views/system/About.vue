<script setup lang="ts">
import GithubButton from '@0xjacky/vue-github-button'
import logo from '@/assets/svg/icon.svg?url'
import ver from '@/version.json'

const thisYear = new Date().getFullYear()
</script>

<template>
  <ACard
    class="text-center"
    :bordered="false"
  >
    <div class="flex justify-center">
      <div class="logo">
        <div class="logo-container">
          <div class="image-bg" />
          <img
            class="image-src"
            :src="logo"
            alt="logo"
          >
        </div>
      </div>
    </div>
    <h2>Nginx UI</h2>
    <p>Yet another WebUI for Nginx</p>
    <p>Version: {{ ver.version }} ({{ ver.total_build || $gettext('Development Mode') }})</p>
    <div class="mb-2">
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
    <div class="mb-2">
      <GithubButton
        href="https://github.com/sponsors/nginxui"
        data-color-scheme="no-preference: light; light: light; dark: dark;"
        data-icon="octicon-heart" data-size="large"
        aria-label="Sponsor @nginxui on GitHub"
      >
        Sponsor
      </GithubButton>
    </div>
    <h3>
      {{ $gettext('Project Team') }}
    </h3>
    <p><a href="https://jackyu.cn/">@0xJacky</a> <a href="https://blog.kugeek.com/">@Hintay</a> <a href="https://github.com/akinoccc">@Akino</a></p>
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
  position: relative;
  height: 256px;
  width: 256px;

  .image-bg {
    height: 120px;
    width: 120px;
    position: absolute;
    top: 50%;
    left: 50%;
    border-radius: 50%;
    filter: blur(72px);
    transform: translate(-50%, -50%);
    background-image: linear-gradient(-45deg, #3682D8 50%, #00D2FF 50%);
  }

  .image-src {
    position: absolute;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
  }

  img {
    max-width: 120px
  }
}
</style>
