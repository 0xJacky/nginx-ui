<script setup lang="ts">
import Breadcrumb from '@/components/Breadcrumb/Breadcrumb.vue'
import {useRoute} from 'vue-router'
import {computed, ref, watch} from 'vue'

const {title, logo, avatar} = defineProps(['title', 'logo', 'avatar'])

const route = useRoute()

const display = computed(() => {
    return !route.meta.hiddenHeaderContent
})

const name = ref(route.name)
watch(() => route.name, () => {
    name.value = route.name
})

</script>

<template>
    <div v-if="display" class="page-header">
        <div class="page-header-index-wide">
            <Breadcrumb/>
            <div class="detail">
                <div class="main">
                    <div class="row">
                        <img v-if="logo" :src="logo" class="logo"/>
                        <h1 class="title">
                            {{ name() }}
                        </h1>
                        <div class="action">
                            <slot name="action"></slot>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>

<style lang="less" scoped>
.page-header {
    background: #fff;
    padding: 16px 32px 0;
    border-bottom: 1px solid #e8e8e8;
    @media (prefers-color-scheme: dark) {
        background: #28292c !important;
        border-bottom: unset;
        h1 {
            color: #fafafa;
        }
    }


    .breadcrumb {
        margin-bottom: 16px;
    }

    .detail {
        display: flex;
        /*margin-bottom: 16px;*/

        .avatar {
            flex: 0 1 72px;
            margin: 0 24px 8px 0;

            & > span {
                border-radius: 72px;
                display: block;
                width: 72px;
                height: 72px;
            }
        }

        .main {
            width: 100%;
            flex: 0 1 auto;

            .row {
                display: flex;
                width: 100%;

                .avatar {
                    margin-bottom: 16px;
                }
            }

            .title {
                font-size: 20px;
                font-weight: 500;
                line-height: 28px;
                margin-bottom: 16px;
                flex: auto;
            }

            .logo {
                width: 28px;
                height: 28px;
                border-radius: 4px;
                margin-right: 16px;
            }

            .content,
            .headerContent {
                flex: auto;
                line-height: 22px;

                .link {
                    margin-top: 16px;
                    line-height: 24px;

                    a {
                        font-size: 14px;
                        margin-right: 32px;
                    }
                }
            }

            .extra {
                flex: 0 1 auto;
                margin-left: 88px;
                min-width: 242px;
                text-align: right;
            }

            .action {
                margin-left: 56px;
                min-width: 266px;
                flex: 0 1 auto;
                text-align: right;

                &:empty {
                    display: none;
                }
            }
        }
    }
}

.mobile .page-header {
    .main {
        .row {
            flex-wrap: wrap;

            .avatar {
                flex: 0 1 25%;
                margin: 0 2% 8px 0;
            }

            .content,
            .headerContent {
                flex: 0 1 70%;

                .link {
                    margin-top: 16px;
                    line-height: 24px;

                    a {
                        font-size: 14px;
                        margin-right: 10px;
                    }
                }
            }

            .extra {
                flex: 1 1 auto;
                margin-left: 0;
                min-width: 0;
                text-align: right;
            }

            .action {
                margin-left: unset;
                min-width: 266px;
                flex: 0 1 auto;
                text-align: left;
                margin-bottom: 12px;

                &:empty {
                    display: none;
                }
            }
        }
    }
}
</style>
