<script setup lang="ts">
import type { Bread } from '@/components/Breadcrumb/types'
import { useBreadcrumbs } from '@/composables/useBreadcrumbs'

const route = useRoute()
const router = useRouter()

const computedBreadList = computed(() => {
  const result: Bread[] = []

  const pushBread = (bread: Bread) => {
    const last = result[result.length - 1]
    const isSameQuery = JSON.stringify(last?.query ?? null) === JSON.stringify(bread.query ?? null)
    if (last && last.path === bread.path && isSameQuery)
      return

    result.push(bread)
  }

  route.matched.forEach(item => {
    if (item.meta?.lastRouteName) {
      const lastRoute = router.resolve({ name: item.meta.lastRouteName })

      pushBread({
        name: lastRoute.name as string,
        translatedName: lastRoute.meta.name as never as () => string,
        path: lastRoute.path,
      })
    }

    pushBread({
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
        v-if="routes.indexOf(breadcrumbRoute) !== routes.length - 1 && getBread(breadcrumbRoute, routes)?.path"
        :to="{ path: getBread(breadcrumbRoute, routes)?.path === '' ? '/' : getBread(breadcrumbRoute, routes)?.path, query: getBread(breadcrumbRoute, routes)?.query }"
      >
        {{ getBread(breadcrumbRoute, routes)?.translatedName() }}
      </RouterLink>
      <span v-else>{{ getBread(breadcrumbRoute, routes)?.translatedName() }}</span>
    </template>
  </ABreadcrumb>
</template>

<style scoped>
</style>
