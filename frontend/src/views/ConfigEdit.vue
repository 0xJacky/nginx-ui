<template>
    <a-card title="配置文件编辑">
        <vue-itextarea v-model="configText"/>
        <footer-tool-bar>
            <a-space>
                <a-button @click="$router.go(-1)">返回</a-button>
                <a-button type="primary" @click="save">保存</a-button>
            </a-space>
        </footer-tool-bar>
    </a-card>
</template>

<script>
import FooterToolBar from "@/components/FooterToolbar/FooterToolBar"
import VueItextarea from "@/components/VueItextarea/VueItextarea"

export default {
    name: "DomainEdit",
    components: {FooterToolBar, VueItextarea},
    data() {
        return {
            name: this.$route.params.name,
            configText: ""
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
                    this.$message.error("服务器错误")
                })
            } else {
                this.configText = ""
            }
        },
        save() {
            this.$api.config.save(this.name ? this.name : this.config.name, {content: this.configText}).then(r => {
                this.configText = r.config
                this.$message.success("保存成功")
            }).catch(r => {
                console.log(r)
                this.$message.error("保存错误")
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
