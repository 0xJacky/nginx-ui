<script setup lang="ts">
import {computed, ref} from 'vue'
import {useRoute} from 'vue-router'

interface bread {
  name: any
  path: string
}

const name = ref()
const route = useRoute()

const breadList = computed(() => {
  let _breadList: bread[] = []

  name.value = route.name

  route.matched.forEach(item => {
    //item.name !== 'index' && this.breadList.push(item)
    _breadList.push({
      name: item.name,
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
      >{{ item.name() }}
      </router-link>
      <span v-else>{{ item.name() }}</span>
    </a-breadcrumb-item>
  </a-breadcrumb>
</template>

<style scoped>
</style>
