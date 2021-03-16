import Vue from 'vue'

let cache = {}

const scrollPosition = {
    // 保存滚动条位置
    save(path) {
        cache[path] = document.documentElement.scrollTop || document.body.scrollTop
    },

    // 重置滚动条位置
    get() {
        const path = this.$route.path
        Vue.prototype.$nextTick(() => {
            document.documentElement.scrollTop = document.body.scrollTop = cache[path] || 0
        })
    },

    // 设置滚动条到顶部
    goTop() {
        Vue.prototype.$nextTick(() => {
            document.documentElement.scrollTop = document.body.scrollTop = 0
        })
    }
}

export default scrollPosition
