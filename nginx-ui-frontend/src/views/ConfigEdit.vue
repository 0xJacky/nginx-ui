<template>
    <a-card title="配置文件实时编辑">
        <a-textarea v-model="configText" :rows="36" @keydown.tab.prevent="pressTab"/>
        <footer-tool-bar>
            <a-button type="primary" @click="save">保存</a-button>
        </footer-tool-bar>
    </a-card>
</template>

<script>
import FooterToolBar from "@/components/FooterToolbar/FooterToolBar"


export default {
    name: "DomainEdit",
    components: {FooterToolBar},
    data() {
        return {
            name: this.$route.params.name,
            configText: ""
        }
    },
    watch: {
        $route() {
            this.config = {}
            this.configText = ""
        },
        config: {
            handler() {
                this.unparse()
            },
            deep: true
        }
    },
    created() {
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
    methods: {
        save() {
            this.$api.config.save(this.name ? this.name : this.config.name, {content: this.configText}).then(r => {
                this.configText = r.config
                this.$message.success("保存成功")
            }).catch(r => {
                console.log(r)
                this.$message.error("保存错误")
            })
        },
        pressTab(event) {
            let target = event.target
            let value = target.value
            let start = target.selectionStart;
            let end = target.selectionEnd;
            if (event) {
                value = value.substring(0, start) + '\t' + value.substring(end);
                event.target.value = value;
                setTimeout(() => target.selectionStart = target.selectionEnd = start + 1, 0);
            }
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
