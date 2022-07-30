<script setup lang="ts">
import {computed, reactive, ref, watch} from "vue";
import {useRoute} from "vue-router"
import {useGettext} from "vue3-gettext";

const {$gettext} = useGettext()

interface bread {
    name: string
    path: string
}

const name = ref('')
const route = useRoute()

const breadList = computed(() => {
    let _breadList: bread[] = []

    name.value = (route.name || '').toString()

    route.matched.forEach(item => {
        //item.name !== 'index' && this.breadList.push(item)
        _breadList.push({
            name: (item.name || '').toString(),
            path: item.path
        })
    })

    return _breadList
})


</script>

<template>
    <a-breadcrumb class="breadcrumb">
        <a-breadcrumb-item v-for="(item, index) in breadList" :key="item.name">
            <router-link
                v-if="item.name !== name && index !== 1"
                :to="{ path: item.path === '' ? '/' : item.path }"
            >{{ $gettext(item.name) }}
            </router-link>
            <span v-else>{{ $gettext(item.name) }}</span>
        </a-breadcrumb-item>
    </a-breadcrumb>
</template>

<style scoped>
</style>
