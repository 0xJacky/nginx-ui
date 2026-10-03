<script setup lang="ts">
import type { Component } from 'vue'
import type { RouteRecordRaw } from 'vue-router'
import type { NgxModule } from '@/api/ngx'
import { AppstoreOutlined } from '@antdv-next/icons'
import { RouterLink } from 'vue-router'
import ngx from '@/api/ngx'
import Logo from '@/components/Logo'
import NodeIndicator from '@/components/NodeIndicator'
import PluginSlot from '@/components/PluginSlot'
import { useGlobalStore } from '@/pinia/moudule/global'
import { usePluginStore } from '@/plugin'
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
  /** Set for plugin entries, whose path is already relative to the layout. */
  to?: string
  order?: number
}

const globalStore = useGlobalStore()
const { modules, modulesMap } = storeToRefs(globalStore)

const pluginStore = usePluginStore()

onMounted(() => {
  ngx.get_modules().then(r => {
    modules.value = r
    modulesMap.value = r.reduce((acc, m) => {
      acc[m.name] = m
      return acc
    }, {} as Record<string, NgxModule>)
  })
})

function isHidden(record?: RouteRecordRaw): boolean {
  const meta = record?.meta
  if (!meta)
    return false

  if (typeof meta.hiddenInSidebar === 'boolean' && meta.hiddenInSidebar)
    return true

  if (typeof meta.hiddenInSidebar === 'function' && meta.hiddenInSidebar())
    return true

  return false
}

function isModuleMissing(record?: RouteRecordRaw): boolean {
  const required = record?.meta?.modules
  if (!required?.length)
    return false

  return !required.every(m => modulesMap.value[m]?.loaded)
}

function toSidebar(record: RouteRecordRaw): Sidebar {
  return {
    path: record.path,
    name: record.name as string,
    meta: {
      ...record.meta,
      icon: record.meta?.icon ? markRaw(record.meta.icon as Component) : undefined,
    } as unknown as Meta,
    children: [],
  }
}

/** Static routes of the authenticated layout, filtered the same way as before. */
const staticSidebars = computed<Sidebar[]>(() => {
  const res: Sidebar[] = []

  for (const s of routes[0].children ?? []) {
    if (isHidden(s) || isModuleMissing(s))
      continue

    const entry = toSidebar(s)

    for (const c of s.children ?? []) {
      if (isHidden(c) || isModuleMissing(c))
        continue

      entry.children.push(toSidebar(c))
    }

    res.push(entry)
  }

  return res
})

/** Plugin routes, turned into sidebar entries of their own. */
function pluginSidebar(record: RouteRecordRaw): Sidebar {
  const entry = toSidebar(record)

  entry.to = `/${record.path.replace(/^\//, '')}`
  entry.order = record.meta?.pluginOrder ?? 0
  entry.meta.icon = entry.meta.icon ?? markRaw(AppstoreOutlined)

  return entry
}

/**
 * Static entries merged with whatever the loaded plugins contributed. A plugin
 * route with a `parent` is nested under that top level entry, everything else
 * becomes a new top level item.
 */
const visible = computed<Sidebar[]>(() => {
  const res = staticSidebars.value.map(entry => ({ ...entry, children: [...entry.children] }))

  for (const record of pluginStore.routes) {
    if (isHidden(record))
      continue

    const entry = pluginSidebar(record)
    const parentName = record.meta?.pluginParent
    const parent = parentName ? res.find(item => item.name === parentName) : undefined

    if (parent)
      parent.children.push(entry)
    else
      res.push(entry)
  }

  for (const entry of res) {
    entry.children.sort((a, b) => (a.order ?? 0) - (b.order ?? 0))
  }

  return res
})

const router = useRouter()

function childLink(parent: Sidebar, child: Sidebar) {
  return child.to ?? `/${parent.path}/${child.path}`
}

const menuItems = computed(() => {
  return visible.value.map(s => {
    if (s.children.length === 0 || s.meta.hideChildren) {
      const target = s.to ?? `/${s.path}`

      return {
        key: s.name,
        icon: s.meta.icon,
        label: s.meta?.name(),
        onClick: () => router.push(target).catch(() => {}),
      }
    }

    return {
      key: s.name,
      icon: s.meta.icon,
      label: s?.meta?.name(),
      children: s.children.map(child => ({
        key: child.name,
        label: h(RouterLink, { to: childLink(s, child) }, () => child?.meta?.name()),
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

    <div class="sidebar-footer">
      <PluginSlot name="sidebar.footer" />
    </div>
  </div>
</template>

<style lang="less">
.sidebar {
  position: sticky;
  top: 0;

  .sidebar-footer {
    flex: none;
  }

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
