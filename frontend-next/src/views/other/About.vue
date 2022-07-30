<script setup lang="ts">
import gettext from '@/gettext'
const {$gettext} = gettext
import logo from '@/assets/img/logo.png'

const this_year = new Date().getFullYear()
const version = import.meta.env.VITE_APP_VERSION
const build_id = import.meta.env.VITE_APP_TOTAL_BUILD ?? $gettext('Development Mode')
const api_root = import.meta.env.VITE_API_ROOT
</script>

<template>
    <a-card style="text-align: center" :bordered="false">
        <div class="logo">
            <img :src="logo" alt="logo"/>
        </div>
        <h2>Nginx UI</h2>
        <p>Yet another WebUI for Nginx</p>
        <p>Version: {{ version }} ({{ build_id }})</p>
        <h3 v-translate>Project Team</h3>
        <p><a href="https://jackyu.cn/">@0xJacky</a>  <a href="https://blog.kugeek.com/">@Hintay</a></p>
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
</style>
