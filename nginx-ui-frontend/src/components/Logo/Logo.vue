<template>
    <div class="logo">
        <p class="text">Nginx UI</p>
        <div class="clear"></div>
    </div>
</template>

<script>
export default {
    name: 'Logo'
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
        font-weight: 300;
        font-size: 23px;
        line-height: 48px;
        height: 48px;
        text-align: center;
    }
}
</style>
