<script setup lang="ts">
import { useGettext } from 'vue3-gettext'

const { $gettext } = useGettext()
</script>

<template>
  <div class="wrapper">
    <h1 class="title">
      {{ $route.meta.status_code || 404 }}
    </h1>
    <p>{{ $route.meta.error?.() ?? $gettext('File Not Found') }}</p>
    <AButton
      type="primary"
      @click="$router.push('/')"
    >
      {{ $gettext('Back Home') }}
    </AButton>
  </div>
</template>

<style lang="less" scoped>
body, div, h1, html {
  padding: 0;
  margin: 0
}

body, html {
  color: #444;
  position: relative;
  font-family: "PingFang SC", "Helvetica Neue", Helvetica, Arial, CustomFont, "Microsoft YaHei UI", "Microsoft YaHei", "Hiragino Sans GB", sans-serif;
  background: #fcfcfc;
  height: 100%
}

h1 {
  font-size: 8em;
  font-weight: 100;
  margin: 10px 0;
}

a {
  color: #4181b9;
  text-decoration: none;
  -webkit-transition: all .3s ease;
  -moz-transition: all .3s ease;
  -ms-transition: all .3s ease;
  -o-transition: all .3s ease;
  transition: all .3s ease;

  &:active, &:hover {
    color: #5bb0ed
  }

}

.wrapper {
  position: absolute;
  top: 0;
  bottom: 0;
  left: 0;
  right: 0;
  font-size: 1em;
  font-weight: 400;
  width: 100%;
  height: 30%;
  line-height: 1;
  margin: auto;
  text-align: center
}
</style>
