<template>
    <a-card title="添加站点">
        <p>在这里添加站点，添加完成后进入域名配置编辑页面即可配置 SSL</p>
        <std-data-entry :data-list="columns" :data-source="config"/>
        <footer-tool-bar>
            <a-button
                type="primary"
                @click="save"
            >
                完成
            </a-button>
        </footer-tool-bar>
    </a-card>
</template>

<script>
import FooterToolBar from "@/components/FooterToolbar/FooterToolBar"
import StdDataEntry from "@/components/StdDataEntry/StdDataEntry"
import {columns} from "@/views/domain/columns"
import {unparse} from "@/views/domain/methods"

export default {
    name: "DomainAdd",
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
                    this.$message.success("保存成功")

                    this.$api.domain.enable(this.config.name).then(() => {
                        this.$message.success("启用成功")

                        this.$router.push('/domain/' + this.config.name)

                    }).catch(r => {
                        console.log(r)
                        this.$message.error(r.message ?? '启用失败', 10)
                    })

                }).catch(r => {
                    console.log(r)
                    this.$message.error(r.message ?? '保存错误', 10)
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
