<template>
    <div>
        <a-card :bordered="false">
            <template v-slot:title>
                <span style="margin-right: 10px">{{ $gettextInterpolate($gettext('Edit %{n}'), {n: name}) }}</span>
                <a-tag color="blue" v-if="enabled">
                    {{ $gettext('Enabled') }}
                </a-tag>
                <a-tag color="orange" v-else>
                    {{ $gettext('Disabled') }}
                </a-tag>
            </template>
            <template v-slot:extra>
                <a-switch size="small" v-model="advance_mode" @change="on_mode_change"/>
                <template v-if="advance_mode">
                    {{ $gettext('Advance Mode') }}
                </template>
                <template v-else>
                    {{ $gettext('Basic Mode') }}
                </template>
            </template>

            <transition name="slide-fade">
                <div v-if="advance_mode" key="advance">
                    <vue-itextarea v-model="configText"/>
                </div>

                <div class="domain-edit-container" key="basic" v-else>
                    <a-form-item :label="$gettext('Enabled')">
                        <a-switch v-model="enabled" @change="checked=>{checked?enable():disable()}"/>
                    </a-form-item>

                    <ngx-config-editor
                        ref="ngx_config"
                        :ngx_config="ngx_config"
                        v-model="auto_cert"
                        :enabled="enabled"
                    />
                </div>
            </transition>

        </a-card>

        <footer-tool-bar>
            <a-space>
                <a-button @click="$router.go(-1)">
                    <translate>Back</translate>
                </a-button>
                <a-button type="primary" @click="save" :loading="saving">
                    <translate>Save</translate>
                </a-button>
            </a-space>
        </footer-tool-bar>
    </div>
</template>


<script>
import FooterToolBar from '@/components/FooterToolbar/FooterToolBar'
import VueItextarea from '@/components/VueItextarea/VueItextarea'
import {$gettext, $interpolate} from '@/lib/translate/gettext'
import NgxConfigEditor from '@/views/domain/ngx_conf/NgxConfigEditor'


export default {
    name: 'DomainEdit',
    components: {NgxConfigEditor, FooterToolBar, VueItextarea},
    data() {
        return {
            name: this.$route.params.name.toString(),
            update: 0,
            ngx_config: {
                filename: '',
                upstreams: [],
                servers: []
            },
            auto_cert: false,
            current_server_index: 0,
            enabled: false,
            configText: '',
            ws: null,
            ok: false,
            issuing_cert: false,
            advance_mode: false,
            saving: false
        }
    },
    watch: {
        '$route'() {
            this.init()
        },
    },
    created() {
        this.init()
    },
    destroyed() {
        if (this.ws !== null) {
            this.ws.close()
        }
    },
    methods: {
        init() {
            if (this.name) {
                this.$api.domain.get(this.name).then(r => {
                    this.configText = r.config
                    this.enabled = r.enabled
                    this.ngx_config = r.tokenized
                    this.auto_cert = r.auto_cert
                }).catch(r => {
                    this.$message.error(r.message ?? $gettext('Server error'))
                })
            }
        },
        on_mode_change(advance_mode) {
            if (advance_mode) {
                this.build_config()
            } else {
                return this.$api.ngx.tokenize_config(this.configText).then(r => {
                    this.ngx_config = r
                }).catch(r => {
                    this.$message.error(r.message ?? $gettext('Server error'))
                })
            }
        },
        build_config() {
            return this.$api.ngx.build_config(this.ngx_config).then(r => {
                this.configText = r.content
            }).catch(r => {
                this.$message.error(r.message ?? $gettext('Server error'))
            })
        },
        async save() {
            this.saving = true

            if (!this.advance_mode) {
                await this.build_config()
            }

            this.$api.domain.save(this.name, {content: this.configText}).then(r => {
                this.configText = r.config
                this.enabled = r.enabled
                this.ngx_config = r.tokenized
                this.$message.success($gettext('Saved successfully'))

                this.$refs.ngx_config.update_cert_info()

            }).catch(r => {
                this.$message.error($interpolate($gettext('Save error %{msg}'), {msg: r.message ?? ''}), 10)
            }).finally(() => {
                this.saving = false
            })

        },
        enable() {
            this.$api.domain.enable(this.name).then(() => {
                this.$message.success($gettext('Enabled successfully'))
                this.enabled = true
            }).catch(r => {
                this.$message.error($interpolate($gettext('Failed to enable %{msg}'), {msg: r.message ?? ''}), 10)
            })
        },
        disable() {
            this.$api.domain.disable(this.name).then(() => {
                this.$message.success($gettext('Disabled successfully'))
                this.enabled = false
            }).catch(r => {
                this.$message.error($interpolate($gettext('Failed to disable %{msg}'), {msg: r.message ?? ''}))
            })
        }
    },
    computed: {
        is_demo() {
            return this.$store.getters.env.demo === true
        }
    }
}
</script>

<style lang="less">

</style>

<style lang="less" scoped>
.ant-card {
    margin: 10px 0;
    box-shadow: unset;
}

.domain-edit-container {
    max-width: 800px;
    margin: 0 auto;

    /deep/ .ant-form-item-label > label::after {
        content: none;
    }
}

.slide-fade-enter-active {
    transition: all .5s ease-in-out;
}

.slide-fade-leave-active {
    transition: all .5s cubic-bezier(1.0, 0.5, 0.8, 1.0);
}

.slide-fade-enter, .slide-fade-leave-to
    /* .slide-fade-leave-active for below version 2.1.8 */ {
    transform: translateX(10px);
    opacity: 0;
}

.location-block {

}

.directive-params-wrapper {
    margin: 10px 0;
}

.tab-content {
    padding: 10px;
}
</style>
