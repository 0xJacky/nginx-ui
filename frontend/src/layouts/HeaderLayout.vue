<template>
    <div class="header">
        <div class="tool">
            <a-icon type="menu-unfold" @click="$emit('clickUnFold')"/>
        </div>
        <div class="user-wrapper">
            <set-language class="set_lang" />

            <a href="/">
                <a-icon type="home"/>
            </a>

            <a @click="logout" style="margin-left: 20px">
                <a-icon type="logout"/>
            </a>
        </div>
    </div>
</template>

<script>
import SetLanguage from '@/components/SetLanguage/SetLanguage'
export default {
    name: 'HeaderComponent',
    components: {SetLanguage},
    methods: {
        logout() {
            this.$api.auth.logout().then(() => {
                this.$message.success('注销成功')
                this.$router.push('/login')
            })
        }
    }
}
</script>

<style lang="less" scoped>
.header {
    height: 64px;
    padding: 0 20px 0 0;
    background: #fff;
    box-shadow: 0 0 20px 0 rgba(0, 0, 0, 0.05);
    @media (prefers-color-scheme: dark) {
        background: #28292c;
        box-shadow: 1px 1px 0 0 #404040;
    }
    position: fixed;
    width: 100%;
}

.tool {
    position: fixed;
    left: 20px;
    @media (min-width: 512px) {
        display: none;
    }
}

.user-wrapper {
    position: fixed;
    right: 20px;
}

.set_lang {
    display: inline;
    margin-right: 25px;
}
</style>
