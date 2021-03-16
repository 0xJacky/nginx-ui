<template>
    <a-card>
        <h2>Nginx UI</h2>
        <p>Yet another WebUI for Nginx</p>
        <p>Version: {{ version }}-{{ build_id }}</p>
        <h3>项目组</h3>
        <p>前端：0xJacky</p>
        <p>后端：0xJacky</p>
        <h3>技术栈</h3>
        <p>Go</p>
        <p>Gin</p>
        <p>Vue</p>

        <p>Copyright © 2020 - {{ this_year }} 0xJacky </p>
    </a-card>
</template>

<script>
export default {
    name: 'About',
    data() {
        const date = new Date()
        return {
            this_year: date.getFullYear(),
            version: process.env.VUE_APP_VERSION,
            build_id: process.env.VUE_APP_BUILD_ID ? process.env.VUE_APP_BUILD_ID : 'dev',
            api_root: process.env.VUE_APP_API_ROOT
        }
    },
    methods: {
        async changeUserPower(power) {
            await this.$store.dispatch('update_mock_user', {power: power})
            await this.$api.user.info()
            await this.$message.success("修改成功")
        }
    }
}
</script>

<style lang="less" scoped>
.egg {
    padding: 10px 0;
}

.ant-btn {
    margin: 10px 10px 0 0;
}
</style>
