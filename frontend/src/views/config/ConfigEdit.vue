<template>
    <a-card :title="$gettext('Edit Configuration')">
        <vue-itextarea v-model="configText"/>
        <footer-tool-bar>
            <a-space>
                <a-button @click="$router.go(-1)"><translate>Cancel</translate></a-button>
                <a-button type="primary" @click="save"><translate>Save</translate></a-button>
            </a-space>
        </footer-tool-bar>
    </a-card>
</template>

<script>
import FooterToolBar from '@/components/FooterToolbar/FooterToolBar'
import VueItextarea from '@/components/VueItextarea/VueItextarea'
import {$gettext, $interpolate} from "@/lib/translate/gettext"

export default {
    name: 'DomainEdit',
    components: {FooterToolBar, VueItextarea},
    data() {
        return {
            name: this.$route.params.name,
            configText: ''
        }
    },
    watch: {
        '$route'() {
            this.init()
        },
        config: {
            handler() {
                this.unparse()
            },
            deep: true
        }
    },
    created() {
        this.init()
    },
    methods: {
        init() {
            if (this.name) {
                this.$api.config.get(this.name).then(r => {
                    this.configText = r.config
                }).catch(r => {
                    console.log(r)
                    this.$message.error($gettext('Server error'))
                })
            } else {
                this.configText = ''
            }
        },
        save() {
            this.$api.config.save(this.name ? this.name : this.config.name, {content: this.configText}).then(r => {
                this.configText = r.config
                this.$message.success($gettext('Saved successfully'))
            }).catch(r => {
                console.log(r)
                this.$message.error($interpolate($gettext('Save error %{msg}'), {msg: r.message ?? ""}))
            })
        }
    }
}
</script>

<style lang="less" scoped>
.ant-card {
    margin: 10px;
    @media (max-width: 512px) {
        margin: 10px 0;
    }
}
</style>
