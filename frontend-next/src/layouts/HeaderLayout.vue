<script setup lang="ts">
import SetLanguage from '@/components/SetLanguage/SetLanguage.vue'
import gettext from '@/gettext'
import {message} from "ant-design-vue"
import auth from '@/api/auth'
// import {HomeOutlined, LogoutOutlined} from "@ant-design/icons-vue"

const {$gettext} = gettext
import {useRouter} from "vue-router"

const router = useRouter()

function logout() {
    auth.logout().then(() => {
        message.success($gettext('Logout successful'))
        window.location.reload()
    })
}

</script>

<template>
    <div class="header">
        <div class="user-wrapper">
            <set-language class="set_lang"/>

            <a href="/">
                <!--                <HomeOutlined/>-->
            </a>

            <a @click="logout" style="margin-left: 20px">
                <!--                <LogoutOutlined/>-->
            </a>
        </div>
    </div>
</template>


<style lang="less" scoped>
.header {
    height: 64px;
    padding: 0 20px 0 0;
    background: transparent;
    box-shadow: 0 0 20px 0 rgba(0, 0, 0, 0.05);
    position: fixed;
    width: 100%;

    a {
        color: #000000;
    }
}

@media (prefers-color-scheme: dark) {
    .header {
        box-shadow: 1px 1px 0 0 #404040;

        a {
            color: #fafafa;
        }
    }
}

.tool {
    position: fixed;
    left: 20px;
    @media (min-width: 512px) {
        display: none;
    }
}

.user-wrapper {
    position: fixed;
    right: 20px;
}

.set_lang {
    display: inline;
    margin-right: 25px;
}
</style>
