<script setup lang="ts">
import gettext from '@/gettext'

const {$gettext} = gettext
import ngx from '@/api/ngx'
import logLevel from '@/views/config/constants'
import {message} from 'ant-design-vue'
import {ReloadOutlined} from '@ant-design/icons-vue'
import Template from '@/views/template/Template.vue'
import {ref, watch} from 'vue'

function get_status() {
    ngx.status().then(r => {
        if (r?.running === true) {
            status.value = 0
        } else {
            status.value = -1
        }
    })
}

function reload_nginx() {
    status.value = 1
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
    }).finally(() => {
        status.value = 0
    })
}

function restart_nginx() {
    status.value = 2
    ngx.restart().then(r => {
        if (r.level < logLevel.Warn) {
            message.success($gettext('Nginx restarted successfully'))
        } else if (r.level === logLevel.Warn) {
            message.warn(r.message)
        } else {
            message.error(r.message)
        }
    }).catch(e => {
        message.error($gettext('Server error') + ' ' + e?.message)
    }).finally(() => {
        status.value = 0
    })
}

const status = ref(0)

const visible = ref(false)

watch(visible, (v) => {
    if (v) get_status()
})
</script>

<template>
    <a-popover
        v-model:visible="visible"
        @confirm="reload_nginx"
        placement="bottomRight"
    >
        <template #content>
            <div class="content-wrapper">
                <h4>{{ $gettext('Nginx Control') }}</h4>
                <a-badge v-if="status===0" color="green" :text="$gettext('Running')"/>
                <a-badge v-else-if="status===1" color="blue" :text="$gettext('Reloading')"/>
                <a-badge v-else-if="status===2" color="orange" :text="$gettext('Restarting')"/>
                <a-badge v-else color="red" :text="$gettext('Stopped')"/>
            </div>
            <a-space>
                <a-button size="small" @click="restart_nginx" type="link">{{ $gettext('Restart') }}</a-button>
                <a-button size="small" @click="reload_nginx" type="link">{{ $gettext('Reload') }}</a-button>
            </a-space>
        </template>
        <a>
            <ReloadOutlined/>
        </a>
    </a-popover>
</template>

<style lang="less" scoped>
a {
    color: #000000;
}

.dark {
    a {
        color: #fafafa;
    }
}

.content-wrapper {
    text-align: center;
    padding-top: 5px;
    padding-bottom: 5px;

    h4 {
        margin-bottom: 5px;
    }
}
</style>
