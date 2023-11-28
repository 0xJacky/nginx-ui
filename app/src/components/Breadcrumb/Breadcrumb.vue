<script setup lang="ts">
import { useRoute } from 'vue-router'

interface bread {
  name: () => string
  path: string
}

const name = ref()
const route = useRoute()

const breadList = computed(() => {
  const _breadList: bread[] = []

  name.value = route.name

  route.matched.forEach(item => {
    // item.name !== 'index' && this.breadList.push(item)
    _breadList.push({
      name: item.name as () => string,
      path: item.path,
    })
  })

  return _breadList
})

</script>

<template>
  <ABreadcrumb class="breadcrumb">
    <ABreadcrumbItem
      v-for="(item, index) in breadList"
      :key="item.name"
    >
      <RouterLink
        v-if="item.name !== name && index !== 1"
        :to="{ path: item.path === '' ? '/' : item.path }"
      >
        {{ item.name() }}
      </RouterLink>
      <span v-else>{{ item.name() }}</span>
    </ABreadcrumbItem>
  </ABreadcrumb>
</template>

<style scoped>
</style>
