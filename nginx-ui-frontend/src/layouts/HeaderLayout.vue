<template>
    <div class="header">
        <div class="tool">
            <a-icon type="menu-unfold" @click="$emit('clickUnFold')"/>
        </div>
        <div class="user-wrapper">
            <a href="/index.html">
                <a-icon type="home"/>
            </a>
        </div>
    </div>
</template>

<script>
export default {
    name: 'HeaderComponent',
    components: {},
}
</script>

<style lang="less" scoped>

.header {
    height: 64px;
    padding: 0 20px 0 0;
    background: #fff;
    box-shadow: 0 0 20px 0 rgba(0, 0, 0, 0.05);
    border-bottom: 1px solid #e8e8e8;
    @media (prefers-color-scheme: dark) {
        background: #28292c;
        box-shadow: 0 1px 4px rgba(0, 21, 41, 0.08);
        border-bottom: unset;
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
</style>
