<script setup lang="ts">
import type { Component } from 'vue'
import type { NgxModule } from '@/api/ngx'
import { RouterLink } from 'vue-router'
import ngx from '@/api/ngx'
import Logo from '@/components/Logo'
import NodeIndicator from '@/components/NodeIndicator'
import { useGlobalStore } from '@/pinia/moudule/global'
import { routes } from '@/routes'

const route = useRoute()

const openKeys = ref([openSub()])

const selectedKey = ref([route.name as string])

function openSub() {
  if (route.matched.length <= 2)
    return ''

  return route.matched[route.matched.length - 2].name as string
}

watch(route, () => {
  selectedKey.value = [route.name as string]

  const sub = openSub()
  const p = openKeys.value.indexOf(sub)
  if (p === -1)
    openKeys.value = [sub]
})

const sidebars = computed(() => {
  return routes[0].children
})

interface Meta {
  icon: Component
  hiddenInSidebar: boolean
  hideChildren: boolean
  name: () => string
}

interface Sidebar {
  path: string
  name: string
  meta: Meta
  children: Sidebar[]
}

const globalStore = useGlobalStore()
const { modules, modulesMap } = storeToRefs(globalStore)

onMounted(() => {
  ngx.get_modules().then(r => {
    modules.value = r
    modulesMap.value = r.reduce((acc, m) => {
      acc[m.name] = m
      return acc
    }, {} as Record<string, NgxModule>)
  })
})

const visible: ComputedRef<Sidebar[]> = computed(() => {
  const res: Sidebar[] = [];

  (sidebars.value || []).forEach(s => {
    if (s.meta && ((typeof s.meta.hiddenInSidebar === 'boolean' && s.meta.hiddenInSidebar)
      || (typeof s.meta.hiddenInSidebar === 'function' && s.meta.hiddenInSidebar()))) {
      return
    }

    if (s.meta && s.meta.modules && s.meta.modules?.length > 0
      && !s.meta.modules.every(m => modulesMap.value[m]?.loaded)) {
      return
    }

    const t: Sidebar = {
      path: s.path,
      name: s.name as string,
      meta: {
        ...s.meta,
        icon: s.meta?.icon ? markRaw(s.meta.icon as Component) : undefined,
      } as unknown as Meta,
      children: [],
    };

    (s.children || []).forEach(c => {
      if (c.meta && ((typeof c.meta.hiddenInSidebar === 'boolean' && c.meta.hiddenInSidebar)
        || (typeof c.meta.hiddenInSidebar === 'function' && c.meta.hiddenInSidebar()))) {
        return
      }

      if (c.meta && c.meta.modules && c.meta.modules?.length > 0
        && !c.meta.modules.every(m => modulesMap.value[m]?.loaded)) {
        return
      }

      t.children.push((c as unknown as Sidebar))
    })
    res.push(t)
  })

  return res
})

const router = useRouter()

const menuItems = computed(() => {
  return visible.value.map(s => {
    if (s.children.length === 0 || s.meta.hideChildren) {
      return {
        key: s.name,
        icon: s.meta.icon,
        label: s.meta?.name(),
        onClick: () => router.push(`/${s.path}`).catch(() => {}),
      }
    }

    return {
      key: s.name,
      icon: s.meta.icon,
      label: s?.meta?.name(),
      children: s.children.map(child => ({
        key: child.name,
        label: h(RouterLink, { to: `/${s.path}/${child.path}` }, () => child?.meta?.name()),
      })),
    }
  })
})
</script>

<template>
  <div class="sidebar">
    <Logo />

    <NodeIndicator />

    <AMenu
      v-model:open-keys="openKeys"
      v-model:selected-keys="selectedKey"
      mode="inline"
      :items="menuItems"
      :styles="{ root: { borderRight: 'unset' } }"
    />
  </div>
</template>

<style lang="less">
.sidebar {
  position: sticky;
  top: 0;

  .logo {
    display: inline-flex;
    justify-content: center;
    align-items: center;

    img {
      margin-left: -18px;
    }
  }
}

.ant-layout-sider-collapsed .logo {
  overflow: hidden;
}

.ant-layout-sider-collapsed {
  .logo {
    img {
      margin-left: 0;
    }

    .text {
      display: none;
    }
  }
}
</style>
