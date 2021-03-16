<template>
    <div class="logo">
        <img :src="logo"/>
        <div class="text">Nginx UI</div>
        <div class="clear"></div>
    </div>
</template>

<script>
export default {
    name: 'Logo',
    data() {
        return {
            logo: require('@/assets/img/logo.png')
        }
    }
}
</script>

<style lang="less" scoped>
.logo {
    margin: 8px 18px;
    border-bottom: 1px solid #e8e8e8;
    transition: all 0.3s;
    height: 56px;
    width: 80%;
    overflow: hidden;
    display: inline-block;

    img {
        height: 46px;
        float: left;
    }

    .text {
        float: left;
        font-weight: 300;
        font-size: 22px;
        line-height: 48px;
        height: 48px;
        display: inline-block;
        margin-left: 13px;
    }
}
</style>
