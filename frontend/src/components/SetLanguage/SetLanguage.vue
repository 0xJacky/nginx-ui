<template>
    <div>
        <a-select v-model="current" size="small" style="width: 50px">
            <a-select-option v-for="(language, key) in $language.available" :value="key" :key="key">
                {{ language }}
            </a-select-option>
        </a-select>
    </div>
</template>

<script>
export default {
    name: 'SetLanguage',
    data() {
        return {
            current: this.$language.current,
        }
    },
    watch: {
        current() {
            this.$store.commit('set_language', this.current)
            this.$nextTick(() => {
                location.reload()
            })
        }
    },
}
</script>

<style lang="less" scoped>

</style>
