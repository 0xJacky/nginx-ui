<script setup lang="ts">
interface bread {
  name: string
  translatedName: () => string
  path: string
  hasChildren?: boolean
}

const name = ref()
const route = useRoute()
const router = useRouter()

const breadList = computed(() => {
  const result: bread[] = []

  name.value = route.meta.name

  route.matched.forEach(item => {
    if (item.meta?.lastRouteName) {
      const lastRoute = router.resolve({ name: item.meta.lastRouteName })

      result.push({
        name: lastRoute.name as string,
        translatedName: lastRoute.meta.name as never as () => string,
        path: lastRoute.path,
      })
    }

    result.push({
      name: item.name as string,
      translatedName: item.meta.name as never as () => string,
      path: item.path,
      hasChildren: item.children?.length > 0,
    })
  })

  return result
})
</script>

<template>
  <ABreadcrumb class="breadcrumb">
    <ABreadcrumbItem
      v-for="(item, index) in breadList"
      :key="item.name"
    >
      <RouterLink
        v-if="index === 0 || !item.hasChildren && index !== breadList.length - 1"
        :to="{ path: item.path === '' ? '/' : item.path }"
      >
        {{ item.translatedName() }}
      </RouterLink>
      <span v-else-if="item.hasChildren">{{ item.translatedName() }}</span>
      <span v-else>{{ item.translatedName() }}</span>
    </ABreadcrumbItem>
  </ABreadcrumb>
</template>

<style scoped>
</style>
