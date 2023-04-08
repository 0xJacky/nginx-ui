<script setup lang="ts">
import {useGettext} from 'vue3-gettext'
import upgrade from '@/api/upgrade'
import {computed, ref} from 'vue'
import version from '@/version.json'
import dayjs from 'dayjs'
import {marked} from 'marked'
import Template from '@/views/template/Template.vue'
import websocket from '@/lib/websocket'

const {$gettext} = useGettext()

const data: any = ref({})
const last_check = ref('')
const loading = ref(false)

const progressStrokeColor = {
    from: '#108ee9',
    to: '#87d068'
}

const modalVisible = ref(false)
const progressPercent = ref(0)
const progressStatus = ref('active')
const modalClosable = ref(false)

function get_release_list() {
    loading.value = true
    upgrade.get_latest_release().then(r => {
        data.value = r
        last_check.value = dayjs().format('YYYY-MM-DD HH:mm:ss')
    }).finally(() => {
        setTimeout(() => {
            loading.value = false
        }, 2000)
    })
}

get_release_list()

const is_latest_ver = computed(() => {
    return data.value.name === `v${version.version}`
})

const logContainer = ref(null)

function log(msg: string) {
    const para = document.createElement('p')
    para.appendChild(document.createTextNode($gettext(msg)));

    (logContainer.value as any as Node).appendChild(para);

    (logContainer.value as any as Element).scroll({top: 320, left: 0, behavior: 'smooth'})
}

async function perform_upgrade() {
    progressStatus.value = 'active'
    modalClosable.value = false
    modalVisible.value = true
    progressPercent.value = 0;
    (logContainer.value as any as Element).innerHTML = ''

    log($gettext('Upgrading Nginx UI, please wait...'))

    const ws = websocket('/api/upgrade/perform', false)

    ws.onmessage = m => {
        const r = JSON.parse(m.data)
        log(r.message)

        switch (r.status) {
            case 'info':
                progressPercent.value += 10
                break
            case 'error':
                progressStatus.value = 'exception'
                modalClosable.value = true
                break
            default:
                modalClosable.value = true
                break
        }
    }

    ws.onclose = () => {
        const t = setInterval(() => {
            upgrade.current_version().then(() => {
                clearInterval(t)
                progressStatus.value = 'success'
                progressPercent.value = 100
                modalClosable.value = true
                log('Upgraded successfully')

                setInterval(() => {
                    location.reload()
                }, 1000)
            })
        }, 2000)
    }
}
</script>

<template>
    <a-modal
        :title="$gettext('Core Upgrade')"
        v-model:visible="modalVisible"
        :mask-closable="false"
        :footer="null" :closable="modalClosable" force-render>
        <a-progress
            :stroke-color="progressStrokeColor"
            :percent="progressPercent"
            :status="progressStatus"
        />

        <div class="core-upgrade-log-container" ref="logContainer"/>
    </a-modal>
    <a-card :title="$gettext('Upgrade')">
        <div class="upgrade-container">
            <p>{{ $gettext('You can check Nginx UI upgrade at this page.') }}</p>
            <h3>{{ $gettext('Current Version') }}: v{{ version.version }}</h3>
            <p>{{ $gettext('OS') }}: {{ data.os }}</p>
            <p>{{ $gettext('Arch') }}: {{ data.arch }}</p>
            <p>{{ $gettext('Executable Path') }}: {{ data.ex_path }}</p>
            <p>{{ $gettext('Last checked at') }}: {{ last_check }}
                <a-button type="link" @click="get_release_list" :loading="loading">
                    {{ $gettext('Check again') }}
                </a-button>
            </p>
            <a-alert type="success" v-if="is_latest_ver"
                     :message="$gettext('You are using the latest version')"
                     banner
            />
            <a-alert type="info" v-else
                     :message="$gettext('New version released')"
                     banner
            />
            <div class="control-btn">
                <a-space>
                    <a-button type="primary" @click="perform_upgrade"
                              ghost v-if="is_latest_ver">{{ $gettext('Reinstall') }}
                    </a-button>
                    <a-button type="primary" @click="perform_upgrade"
                              ghost v-else>{{ $gettext('Upgrade') }}
                    </a-button>
                </a-space>
            </div>
            <template v-if="data.body">
                <h2>{{ $gettext('Release Note') }}</h2>
                <div v-html="marked.parse(data.body)"></div>
            </template>
        </div>
    </a-card>
</template>

<style lang="less">
.core-upgrade-log-container {
    height: 320px;
    overflow: scroll;
    background-color: #f3f3f3;
    border-radius: 4px;
    margin-top: 15px;
    padding: 10px;

    p {
        font-size: 12px;
        line-height: 1.3;
    }
}
</style>

<style lang="less" scoped>
.upgrade-container {
    width: 100%;
    max-width: 600px;
    margin: 0 auto;
    padding: 0 10px;
}

.control-btn {
    margin: 15px 0;
}
</style>
