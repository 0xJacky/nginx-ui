<script setup lang="ts">
import StdTable from '@/components/StdDataDisplay/StdTable.vue'
import gettext from '@/gettext'
import config from '@/api/config'
import {customRender, datetime} from '@/components/StdDataDisplay/StdTableTransformer'
import {computed, h, nextTick, ref, watch} from 'vue'

const {$gettext} = gettext

const api = config

import configColumns from '@/views/config/config'
import {useRoute} from 'vue-router'
import FooterToolBar from '@/components/FooterToolbar/FooterToolBar.vue'
import router from '@/routes'
import InspectConfig from '@/views/config/InspectConfig.vue'

const table = ref(null)
const route = useRoute()

const basePath = computed(() => {
    let dir = route?.query?.dir ?? ''
    if (dir) dir += '/'
    return dir
})

const get_params = computed(() => {
    return {
        dir: basePath.value
    }
})

const update = ref(1)

watch(get_params, () => {
    update.value++
})

const inspect_config = ref()

watch(route, () => {
    inspect_config.value?.test()
})
</script>

<template>
    <inspect-config ref="inspect_config"/>

    <a-card :title="$gettext('Configurations')">
        <std-table
            :key="update"
            ref="table"
            :api="api"
            :columns="configColumns"
            :deletable="false"
            :disable_search="true"
            row-key="name"
            :get_params="get_params"
            @clickEdit="(r, row) => {
                if (!row.is_dir) {
                    $router.push({
                        path: '/config/' + basePath + r + '/edit'
                    })
                } else {
                    $router.push({
                        query: {
                            dir: basePath + r
                        }
                    })
                }
            }"
        />
        <footer-tool-bar v-if="basePath">
            <a-button @click="router.go(-1)">{{ $gettext('Back') }}</a-button>
        </footer-tool-bar>
    </a-card>
</template>

<style scoped>

</style>
