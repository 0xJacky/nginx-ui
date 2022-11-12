<script setup lang="ts">
import Logo from '@/components/Logo/Logo.vue'
import {routes} from '@/routes'
import {useRoute} from 'vue-router'
import {computed, ref, watch} from 'vue'

const route = useRoute()

let openKeys = [openSub()]

const selectedKey = ref([route.name])

function openSub() {
    let path = route.path
    let lastSepIndex = path.lastIndexOf('/')
    return path.substring(1, lastSepIndex)
}

watch(route, () => {
    selectedKey.value = [route.name]
    const sub = openSub()
    const p = openKeys.indexOf(sub)
    if (p === -1) openKeys.push(sub)
})

const sidebars = computed(() => {
    return routes[0]['children']
})

interface meta {
    icon: any
    hiddenInSidebar: boolean
}

interface sidebar {
    path: string
    name: Function
    meta: meta,
    children: sidebar[]
}

const visible = computed(() => {

    const res: sidebar[] = [];

    (sidebars.value || []).forEach((s) => {
        if (s.meta && s.meta.hiddenInSidebar) {
            return
        }
        const t: sidebar = {
            path: s.path,
            name: s.name,
            meta: s.meta as meta,
            children: []
        };

        (s.children || []).forEach((c: any) => {
            if (c.meta && c.meta.hiddenInSidebar) {
                return
            }
            t.children.push((c as sidebar))
        })
        res.push(t)
    })


    return res
})
</script>

<template>
    <div class="sidebar">
        <logo/>
        <a-menu
            :openKeys="openKeys"
            mode="inline"
            v-model:openKeys="openKeys"
            v-model:selectedKeys="selectedKey"
        >
            <template v-for="sidebar in visible">
                <a-menu-item v-if="sidebar.children.length===0 || sidebar.meta.hideChildren === true"
                             :key="sidebar.name"
                             @click="$router.push('/'+sidebar.path).catch(() => {})">
                    <component :is="sidebar.meta.icon"/>
                    <span>{{ sidebar.name() }}</span>
                </a-menu-item>

                <a-sub-menu v-else :key="sidebar.path">
                    <template #title>
                        <component :is="sidebar.meta.icon"/>
                        <span>{{ sidebar.name() }}</span>
                    </template>
                    <a-menu-item v-for="child in sidebar.children" :key="child.name">
                        <router-link :to="'/'+sidebar.path+'/'+child.path">
                            {{ child.name() }}
                        </router-link>
                    </a-menu-item>
                </a-sub-menu>
            </template>
        </a-menu>
    </div>
</template>

<style lang="less">
.sidebar {
    position: sticky;
    top: 0;
}

.ant-layout-sider-collapsed .logo {
    overflow: hidden;
}

.ant-menu-inline, .ant-menu-vertical, .ant-menu-vertical-left {
    border-right: unset;
}
</style>
