<script setup lang="ts">
import {useSettingsStore} from '@/pinia'
import {useGettext} from 'vue3-gettext'
import {computed, ref} from 'vue'
import environment from '@/api/environment'
import Icon, {LinkOutlined, SendOutlined, ThunderboltOutlined} from '@ant-design/icons-vue'
import logo from '@/assets/img/logo.png'
import pulse from '@/assets/svg/pulse.svg'
import cpu from '@/assets/svg/cpu.svg'
import memory from '@/assets/svg/memory.svg'
import {formatDateTime} from '@/lib/helper'

const settingsStore = useSettingsStore()
const {$gettext} = useGettext()

const data = ref([])

environment.get_list().then(r => {
    data.value = r.data
})

export interface Node {
    id: number
    name: string
    token: string
}

const {environment: env} = settingsStore

function link_start(node: Node) {
    env.id = node.id
    env.name = node.name
}

const visible = computed(() => {
    if (env.id > 0) {
        return false
    } else {
        return data.value?.length
    }
})
</script>

<template>
    <a-card class="env-list-card" :title="$gettext('Environments')" v-if="visible">
        <a-list item-layout="horizontal" :data-source="data">
            <template #renderItem="{ item }">
                <a-list-item>
                    <template #actions>
                        <a-button type="primary" @click="link_start(item)" :disabled="env.id===item.id" ghost>
                            <send-outlined/>
                            {{ env.id !== item.id ? $gettext('Link Start') : $gettext('Connected') }}
                        </a-button>
                    </template>
                    <a-list-item-meta>
                        <template #title>
                            {{ item.name }}
                            <a-tag color="blue" v-if="item.status">{{ $gettext('Online') }}</a-tag>
                            <a-tag color="error" v-else>{{ $gettext('Offline') }}</a-tag>
                            <div class="runtime-meta">
                                <template v-if="item.status">
                                    <span><Icon :component="pulse"/> {{ formatDateTime(item.response_at) }}</span>
                                    <span><thunderbolt-outlined/>{{ item.version }}</span>
                                </template>
                                <span><link-outlined/>{{ item.url }}</span>
                            </div>
                        </template>
                        <template #avatar>
                            <a-avatar :src="logo"/>
                        </template>
                        <template #description>
                            <div class="runtime-meta">
                                <span><Icon :component="cpu"/> {{ item.cpu_num }} CPU</span>
                                <span><Icon :component="memory"/> {{ item.memory_total }}</span>
                            </div>
                        </template>
                    </a-list-item-meta>
                </a-list-item>
            </template>
        </a-list>
    </a-card>
</template>

<style scoped lang="less">
.env-list-card {
    margin-top: 16px;

    .runtime-meta {
        display: inline-flex;

        span {
            font-weight: 400;
            font-size: 13px;
            margin-right: 16px;
            color: #9b9b9b;

            &.anticon {
                margin-right: 4px;
            }
        }
    }
}
</style>
