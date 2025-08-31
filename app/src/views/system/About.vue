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
    <p>
      <a href="https://nginxui.com" target="_blank" rel="noopener noreferrer">
        {{ $gettext('Official Document') }}
      </a>
    </p>
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
    <h3 class="mb-4">
      {{ $gettext('Sponsor') }}
    </h3>
    <div class="mb-4">
      <GithubButton
        href="https://github.com/sponsors/nginxui"
        data-color-scheme="no-preference: light; light: light; dark: dark;"
        data-icon="octicon-heart" data-size="large"
        aria-label="Sponsor @nginxui on GitHub"
      >
        Sponsor
      </GithubButton>
    </div>
    <div class="mb-4">
      <a
        href="https://afdian.com/a/nginxui"
        target="_blank"
        rel="noopener noreferrer"
      >
        <img
          src="https://img.shields.io/badge/爱发电-Support-946ce6?style=for-the-badge&logo=data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMjQiIGhlaWdodD0iMjQiIHZpZXdCb3g9IjAgMCAyNCAyNCIgZmlsbD0ibm9uZSIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj4KPHBhdGggZD0iTTEyIDJMMTMuMDkgOC4yNkwyMCA5TDEzLjA5IDE1Ljc0TDEyIDIyTDEwLjkxIDE1Ljc0TDQgOUwxMC45MSA4LjI2TDEyIDJaIiBmaWxsPSJ3aGl0ZSIvPgo8L3N2Zz4K"
          alt="Support on Afdian"
        >
      </a>
    </div>
    <h3 class="mb-4">
      {{ $gettext('Project Team') }}
    </h3>
    <p><a href="https://jackyu.cn/">@0xJacky</a> <a href="https://blog.kugeek.com/">@Hintay</a> <a href="https://github.com/akinoccc">@Akino</a></p>
    <h3>
      {{ $gettext('License') }}
    </h3>
    <div class="mb-3">
      <AButton
        type="link"
        size="small"
        @click="$router.push('/system/licenses')"
      >
        {{ $gettext('View Third-party Components') }}
      </AButton>
    </div>
    <p>GNU Affero General Public License v3.0</p>
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
