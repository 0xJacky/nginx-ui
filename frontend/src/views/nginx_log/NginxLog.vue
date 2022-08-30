<script setup lang="ts">
import {useGettext} from 'vue3-gettext'
import ws from '@/lib/websocket'
import {computed, nextTick, onMounted, onUnmounted, reactive, ref, watch} from 'vue'
import ReconnectingWebSocket from 'reconnecting-websocket'
import {useRoute} from 'vue-router'

const {$gettext} = useGettext()

const logContainer = ref(null)

let websocket: ReconnectingWebSocket | WebSocket
const route = useRoute()

function logType() {
    return route.path.indexOf('access') > 0 ? 'access' : 'error'
}

const control = reactive({
    fetch: 'new',
    type: logType()
})

function openWs() {
    websocket = ws('/api/nginx_log')
    websocket.send(JSON.stringify(control))
    websocket.onmessage = (m: any) => {
        const para = document.createElement('p')
        para.appendChild(document.createTextNode(m.data.trim()));

        (logContainer.value as any as Node).appendChild(para);

        (logContainer.value as any as Element).scroll({
            top: (logContainer.value as any as Element).scrollHeight,
            left: 0,
            behavior: 'smooth'
        })
    }
}

onMounted(() => {
    openWs()
})

const auto_refresh = ref(true)

watch(auto_refresh, (value) => {
    if (value) {
        openWs();
        (logContainer.value as any as Element).innerHTML = ''

    } else {
        websocket.close()
    }
})

watch(route, () => {
    control.type = logType();
    (logContainer.value as any as Element).innerHTML = ''

    nextTick(() => {
        websocket.send(JSON.stringify(control))
    })
})

watch(control, () => {
    (logContainer.value as any as Element).innerHTML = ''
    auto_refresh.value = true

    nextTick(() => {
        websocket.send(JSON.stringify(control))
    })
})

onUnmounted(() => {
    websocket.close()
})

</script>

<template>
    <a-card :title="$gettext('Nginx Log')" :bordered="false">
        <a-form layout="vertical">
            <a-form-item :label="$gettext('Auto Refresh')">
                <a-switch v-model:checked="auto_refresh"/>
            </a-form-item>
            <a-form-item :label="$gettext('Fetch')">
                <a-select v-model:value="control.fetch" style="max-width: 200px">
                    <a-select-option value="all">All logs</a-select-option>
                    <a-select-option value="new">New logs</a-select-option>
                </a-select>
            </a-form-item>
        </a-form>

        <a-card>
            <pre class="nginx-log-container" ref="logContainer"></pre>
        </a-card>
    </a-card>
</template>

<style lang="less">
.nginx-log-container {
    height: 60vh;
    overflow: scroll;
    padding: 5px;

    p {
        font-size: 12px;
        line-height: 1;
    }
}
</style>
