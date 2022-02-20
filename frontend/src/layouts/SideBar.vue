<template>
    <div class="sidebar">
        <logo/>
        <a-menu
            :openKeys="openKeys"
            mode="inline"
            @openChange="onOpenChange"
            v-model="selectedKey"
        >
            <template v-for="sidebar in visible(sidebars)">
                <a-menu-item v-if="!sidebar.children" :key="sidebar.name"
                             @click="$router.push('/'+sidebar.path).catch(() => {})">
                    <a-icon :type="sidebar.meta.icon"/>
                    <span>{{ sidebar.name }}</span>
                </a-menu-item>

                <a-sub-menu v-else :key="sidebar.path">
                    <span slot="title"><a-icon :type="sidebar.meta.icon"/><span>{{ sidebar.name }}</span></span>
                    <a-menu-item v-for="child in visible(sidebar.children)" :key="child.name">
                        <router-link :to="'/'+sidebar.path+'/'+child.path">
                            {{ child.name }}
                        </router-link>
                    </a-menu-item>
                </a-sub-menu>
            </template>
        </a-menu>
    </div>
</template>

<script>
import Logo from '@/components/Logo/Logo'

export default {
    name: 'SideBar',
    components: {Logo},
    data() {
        return {
            rootSubmenuKeys: [],
            openKeys: [this.openSub()],
            sidebars: this.$routeConfig[0]['children'],
            selectedKey: [this.$route.name],
        }
    },
    watch: {
        '$route'() {
            this.selectedKey = [this.$route.name]
            const sub = this.openSub()
            const p = this.openKeys.indexOf(sub)
            if (p === -1) this.openKeys.push(this.openSub())
        }
    },
    created() {
        this.sidebars.forEach((element) => {
            this.rootSubmenuKeys.push(element)
        })
    },
    methods: {
        openSub() {
            let path = this.$route.path
            let lastSepIndex = path.lastIndexOf('/')
            return path.substring(1, lastSepIndex)
        },
        onOpenChange(openKeys) {
            const latestOpenKey = openKeys.find(key => this.openKeys.indexOf(key) === -1)
            if (this.rootSubmenuKeys.indexOf(latestOpenKey) === -1) {
                this.openKeys = openKeys
            } else {
                this.openKeys = latestOpenKey ? [latestOpenKey] : []
            }
        },
        visible(sidebars) {
            return sidebars.filter(c => {
                return c.meta === undefined ||
                    (c.meta.hiddenInSidebar === undefined || c.meta.hiddenInSidebar !== true)
                    && (c.meta.power === undefined || this.$store.state.user.power === c.meta.power)
            })
        }
    }
}
</script>


<style lang="less">
.ant-layout-sider-collapsed .logo {
    overflow: hidden;
}

.ant-menu-inline, .ant-menu-vertical, .ant-menu-vertical-left {
    border-right: unset;
}
</style>
