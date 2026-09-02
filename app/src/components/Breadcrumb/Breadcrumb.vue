<script setup lang="ts">
import type { Bread } from '@/components/Breadcrumb/types'
import { useBreadcrumbs } from '@/composables/useBreadcrumbs'

const route = useRoute()
const router = useRouter()

const computedBreadList = computed(() => {
  const result: Bread[] = []

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

const breadList = useBreadcrumbs()

const breadcrumbItems = computed(() => breadList.value.map(item => ({
  key: item.name,
  path: item.path,
})))

const getBread = (route: unknown, routes: readonly unknown[]) => breadList.value[routes.indexOf(route)]

onMounted(() => {
  breadList.value = computedBreadList.value
})

watch(route, () => {
  breadList.value = computedBreadList.value
})
</script>

<template>
  <ABreadcrumb class="breadcrumb" :items="breadcrumbItems">
    <template #itemRender="{ route: breadcrumbRoute, routes }">
      <RouterLink
        v-if="routes.indexOf(breadcrumbRoute) === 0 || !getBread(breadcrumbRoute, routes)?.hasChildren && routes.indexOf(breadcrumbRoute) !== routes.length - 1"
        :to="{ path: getBread(breadcrumbRoute, routes)?.path === '' ? '/' : getBread(breadcrumbRoute, routes)?.path, query: getBread(breadcrumbRoute, routes)?.query }"
      >
        {{ getBread(breadcrumbRoute, routes)?.translatedName() }}
      </RouterLink>
      <span v-else-if="getBread(breadcrumbRoute, routes)?.hasChildren">{{ getBread(breadcrumbRoute, routes)?.translatedName() }}</span>
      <span v-else>{{ getBread(breadcrumbRoute, routes)?.translatedName() }}</span>
    </template>
  </ABreadcrumb>
</template>

<style scoped>
</style>
