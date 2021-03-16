<template>
    <a-config-provider :locale="zh_CN">
        <a-layout style="min-height: 100%;">
            <a-drawer
                v-show="clientWidth<512"
                :closable="false"
                :visible="collapsed"
                placement="left"
                @close="collapsed=false"
            >
                <side-bar/>
            </a-drawer>

            <a-layout-sider
                v-show="clientWidth>=512"
                v-model="collapsed"
                :collapsible="true"
                :style="{zIndex: 11}"
                theme="light"
            >
                <side-bar/>
            </a-layout-sider>

            <a-layout>
                <a-layout-header :style="{position: 'fixed', zIndex: 10, width:'100%'}">
                    <header-layout @clickUnFold="collapsed=true"/>
                </a-layout-header>

                <a-layout-content>
                    <page-header :title="$route.name"/>
                    <div class="router-view">
                        <router-view/>
                    </div>
                </a-layout-content>

                <a-layout-footer>
                    <footer-layout/>
                </a-layout-footer>
            </a-layout>

        </a-layout>
    </a-config-provider>
</template>

<script>
import HeaderLayout from './HeaderLayout'
import SideBar from './SideBar'
import FooterLayout from './FooterLayout'
import PageHeader from '@/components/PageHeader/PageHeader'
import zh_CN from 'ant-design-vue/lib/locale-provider/zh_CN'

export default {
    name: 'BaseLayout',
    data() {
        return {
            collapsed: this.collapse(),
            zh_CN,
            clientWidth: document.body.clientWidth
        }
    },
    mounted() {
        window.onresize = () => {
            this.collapsed = this.collapse()
            this.clientWidth = this.getClientWidth()
        }
    },
    components: {
        SideBar,
        PageHeader,
        HeaderLayout,
        FooterLayout
    },
    methods: {}
}
</script>

<style lang="less">
@dark: ~"(prefers-color-scheme: dark)";

body {
    overflow: unset !important;
}

p {
    padding: 0 0 10px 0;
}

.ant-layout-sider {
    background-color: #ffffff;
    @media @dark {
        background-color: #28292c;
    }
    box-shadow: 2px 0 6px rgba(0, 21, 41, 0.01);
}

@media @dark {
    .ant-checkbox-indeterminate {
        .ant-checkbox-inner {
            background-color: transparent !important;
        }
    }
}

.ant-layout-header {
    padding: 0;
}

.ant-table-small {
    font-size: 13px;
}

.ant-card-bordered {
    border: unset;
}

.header-notice-wrapper .ant-tabs-content {
    max-height: 250px;
}

.header-notice-wrapper .ant-tabs-tabpane-active {
    overflow-y: scroll;
}

.ant-layout-footer {
    @media (max-width: 320px) {
        padding: 10px;
    }
}

.ant-layout-content {
    margin: 64px 0;
    min-height: auto;

    .router-view {
        padding: 20px;
        @media (max-width: 512px) {
            padding: 20px 0;
        }
        position: relative;
    }
}
</style>
