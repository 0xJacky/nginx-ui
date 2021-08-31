<template>
    <div class="ProseMirror" v-html="html"></div>
</template>

<script>
export default {
    name: "RichText",
    props: ['html']
}
</script>


<style lang="less">
@import "style";
</style>
