<template>
    <a-card style="text-align: center">
        <div class="logo">
            <img :src="logo" alt="logo"/>
        </div>
        <h2>Nginx UI</h2>
        <p>Yet another WebUI for Nginx</p>
        <p>Version: {{ version }} ({{ build_id }})</p>
        <h3 v-translate>Project Team</h3>
        <p><a href="https://jackyu.cn/">@0xJacky</a></p>
        <p><a href="https://blog.kugeek.com/">@Hintay</a></p>
        <h3 v-translate>Build with</h3>
        <p>Go</p>
        <p>Gin</p>
        <p>Vue</p>
        <p>Websocket</p>
        <h3 v-translate translate-context="Project">License</h3>
        <p>GNU General Public License v3.0</p>
        <p>Copyright © 2020 - {{ this_year }} Nginx UI </p>
    </a-card>
</template>

<script>
import $gettext from "@/lib/translate/gettext";

export default {
    name: 'About',
    data() {
        const date = new Date()
        return {
            logo: require('@/assets/img/logo.png'),
            this_year: date.getFullYear(),
            version: process.env.VUE_APP_VERSION,
            build_id: process.env.VUE_APP_TOTAL_BUILD ?? $gettext('Development Mode'),
            api_root: process.env.VUE_APP_API_ROOT
        }
    }
}
</script>

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
