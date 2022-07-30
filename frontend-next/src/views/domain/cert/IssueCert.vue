<template>
    <div>
        <a-form-item :label="$gettext('Encrypt website with Let\'s Encrypt')">
            <a-switch
                :loading="issuing_cert"
                v-model="M_enabled"
                @change="onchange"
                :disabled="no_server_name||server_name_more_than_one"
            />
            <a-alert
                v-if="no_server_name||server_name_more_than_one"
                :message="$gettext('Warning')"
                type="warning"
                show-icon
            >
                <template slot="description">
                    <span v-if="no_server_name" v-translate>
                        server_name parameter is required
                    </span>
                    <span v-if="server_name_more_than_one" v-translate>
                        server_name parameters more than one
                    </span>
                </template>
            </a-alert>
        </a-form-item>
        <p v-translate>
            Note: The server_name in the current configuration must be the domain name
            you need to get the certificate.
        </p>
        <p v-if="enabled" v-translate>
            The certificate for the domain will be checked every hour,
            and will be renewed if it has been more than 1 month since it was last issued.
        </p>
        <p v-translate>
            Make sure you have configured a reverse proxy for .well-known
            directory to HTTPChallengePort (default: 9180) before getting the certificate.
        </p>
    </div>
</template>

<script>
import {issue_cert} from '@/views/domain/methods'
import $gettext, {$interpolate} from '@/lib/translate/gettext'

export default {
    name: 'IssueCert',
    props: {
        directivesMap: Object,
        current_server_directives: Array,
        enabled: Boolean
    },
    model: {
        prop: 'enabled',
        event: 'changeEnabled'
    },
    data() {
        return {
            issuing_cert: false,
            M_enabled: this.enabled,
        }
    },
    methods: {
        onchange(r) {
            this.$emit('changeEnabled', r)
            this.change_auto_cert(r)
            if (r) {
                this.job()
            }
        },
        job() {
            this.issuing_cert = true

            if (this.no_server_name) {
                this.$message.error($gettext('server_name not found in directives'))
                this.issuing_cert = false
                return
            }

            if (this.server_name_more_than_one) {
                this.$message.error($gettext('server_name parameters more than one'))
                this.issuing_cert = false
                return
            }

            const server_name = this.directivesMap['server_name'][0]

            if (!this.directivesMap['ssl_certificate']) {
                this.current_server_directives.splice(server_name.idx + 1, 0, {
                    directive: 'ssl_certificate',
                    params: ''
                })
            }

            this.$nextTick(() => {
                if (!this.directivesMap['ssl_certificate_key']) {
                    const ssl_certificate = this.directivesMap['ssl_certificate'][0]
                    this.current_server_directives.splice(ssl_certificate.idx + 1, 0, {
                        directive: 'ssl_certificate_key',
                        params: ''
                    })
                }
            })

            setTimeout(() => {
                issue_cert(this.name, this.callback)
            }, 100)
        },
        callback(ssl_certificate, ssl_certificate_key) {
            this.$set(this.directivesMap['ssl_certificate'][0], 'params', ssl_certificate)
            this.$set(this.directivesMap['ssl_certificate_key'][0], 'params', ssl_certificate_key)
            this.issuing_cert = false
            this.$emit('callback')
        },
        change_auto_cert(r) {
            if (r) {
                this.$api.domain.add_auto_cert(this.name).then(() => {
                    this.$message.success($interpolate($gettext('Auto-renewal enabled for %{name}'), {name: this.name}))
                }).catch(e => {
                    this.$message.error(e.message ?? $interpolate($gettext('Enable auto-renewal failed for %{name}'), {name: this.name}))
                })
            } else {
                this.$api.domain.remove_auto_cert(this.name).then(() => {
                    this.$message.success($interpolate($gettext('Auto-renewal disabled for %{name}'), {name: this.name}))
                }).catch(e => {
                    this.$message.error(e.message ?? $interpolate($gettext('Disable auto-renewal failed for %{name}'), {name: this.name}))
                })
            }
        },
    },
    watch: {
        server_name_more_than_one() {
            this.M_enabled = false
            this.onchange(false)
        },
        no_server_name() {
            this.M_enabled = false
            this.onchange(false)
        }
    },
    computed: {
        is_demo() {
            return this.$store.getters.env.demo === true
        },
        server_name_more_than_one() {
            return this.directivesMap['server_name'] && (this.directivesMap['server_name'].length > 1 ||
                this.directivesMap['server_name'][0].params.trim().indexOf(' ') > 0)
        },
        no_server_name() {
            return !this.directivesMap['server_name']
        },
        name() {
            return this.directivesMap['server_name'][0].params.trim()
        }
    }
}
</script>

<style lang="less" scoped>
.switch-wrapper {
    position: relative;

    .text {
        position: absolute;
        top: 50%;
        transform: translateY(-50%);
        margin-left: 10px;
    }
}
</style>
