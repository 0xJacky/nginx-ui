<script setup lang="ts">
import SetLanguage from '@/components/SetLanguage/SetLanguage.vue'
import gettext from '@/gettext'
import {message} from 'ant-design-vue'
import auth from '@/api/auth'
import {HomeOutlined, LogoutOutlined, MenuUnfoldOutlined, ReloadOutlined} from '@ant-design/icons-vue'
import {useRouter} from 'vue-router'
import ngx from '@/api/ngx'
import logLevel from '@/views/config/constants'

const {$gettext} = gettext

const router = useRouter()

function logout() {
    auth.logout().then(() => {
        message.success($gettext('Logout successful'))
    }).then(() => {
        router.push('/login')
    })
}

function reload_nginx() {
    ngx.reload().then(r => {
        if (r.level < logLevel.Warn) {
            message.success($gettext('Nginx reloaded successfully'))
        } else if (r.level === logLevel.Warn) {
            message.warn(r.message)
        } else {
            message.error(r.message)
        }
    }).catch(e => {
        message.error($gettext('Server error') + ' ' + e?.message)
    })
}
</script>

<template>
    <div class="header">
        <div class="tool">
            <MenuUnfoldOutlined @click="$emit('clickUnFold')"/>
        </div>

        <a-space class="user-wrapper" :size="24">
            <set-language class="set_lang"/>

            <a href="/">
                <HomeOutlined/>
            </a>

            <a-popconfirm
                :title="$gettext('Do you want to reload Nginx?')"
                :ok-text="$gettext('Yes')"
                :cancel-text="$gettext('No')"
                @confirm="reload_nginx"
                placement="bottomRight"
            >
                <a>
                    <ReloadOutlined/>
                </a>
            </a-popconfirm>

            <a @click="logout">
                <LogoutOutlined/>
            </a>
        </a-space>
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

.dark {
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
    @media (min-width: 600px) {
        display: none;
    }
}

.user-wrapper {
    position: fixed;
    right: 28px;
}

.set_lang {
    display: inline;
}
</style>
