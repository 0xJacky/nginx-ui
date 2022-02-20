<template>
    <a-card :title="$gettext('Add Site')">
        <p v-translate>Add site here first, then you can configure TLS on the domain edit page.</p>
        <std-data-entry :data-list="columns" :data-source="config"/>
        <footer-tool-bar>
            <a-button
                type="primary"
                @click="save"
            >
                <translate>Save</translate>
            </a-button>
        </footer-tool-bar>
    </a-card>
</template>

<script>
import FooterToolBar from '@/components/FooterToolbar/FooterToolBar'
import StdDataEntry from '@/components/StdDataEntry/StdDataEntry'
import {columns} from '@/views/domain/columns'
import {unparse} from '@/views/domain/methods'
import $gettext, {$interpolate} from "@/lib/translate/gettext";

export default {
    name: 'DomainAdd',
    components: {StdDataEntry, FooterToolBar},
    data() {
        return {
            config: {},
            columns: columns.slice(0, -1) // 隐藏SSL支持开关
        }
    },
    beforeCreate() {

    },
    methods: {
        save() {
            this.$api.domain.get_template('http-conf').then(r => {
                let text = unparse(r.template, this.config)
                this.$api.domain.save(this.config.name, {content: text, enabled: true}).then(() => {
                    this.$message.success($gettext('Saved successfully'))

                    this.$api.domain.enable(this.config.name).then(() => {
                        this.$message.success($gettext('Enabled successfully'))

                        this.$router.push('/domain/' + this.config.name)

                    }).catch(r => {
                        console.log(r)
                        this.$message.error(r.message ?? $gettext('Enable failed'), 10)
                    })

                }).catch(r => {
                    console.log(r)
                    this.$message.error($interpolate($gettext('Save error %{msg}'), {msg: r.message ?? ""}), 10)
                })
            })
        }
    }
}
</script>

<style lang="less" scoped>
.ant-steps {
    padding: 10px 0 20px 0;
}
</style>
