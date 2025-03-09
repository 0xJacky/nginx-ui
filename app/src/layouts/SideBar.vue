<script setup lang="ts">
import type { IconComponentProps } from '@ant-design/icons-vue/es/components/Icon'
import type { AntdIconType } from '@ant-design/icons-vue/lib/components/AntdIcon'
import type { Key } from 'ant-design-vue/es/_util/type'
import type { ComputedRef, Ref } from 'vue'
import EnvIndicator from '@/components/EnvIndicator/EnvIndicator.vue'
import Logo from '@/components/Logo/Logo.vue'
import { routes } from '@/routes'

const route = useRoute()

const openKeys = ref([openSub()])

const selectedKey = ref([route.name]) as Ref<Key[]>

function openSub() {
  const path = route.path
  const lastSepIndex = path.lastIndexOf('/')

  return path.substring(1, lastSepIndex)
}

watch(route, () => {
  selectedKey.value = [route.name as Key]

  const sub = openSub()
  const p = openKeys.value.indexOf(sub)
  if (p === -1)
    openKeys.value = [sub]
})

const sidebars = computed(() => {
  return routes[0].children
})

interface Meta {
  icon: AntdIconType
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

const visible: ComputedRef<Sidebar[]> = computed(() => {
  const res: Sidebar[] = [];

  (sidebars.value || []).forEach(s => {
    if (s.meta && ((typeof s.meta.hiddenInSidebar === 'boolean' && s.meta.hiddenInSidebar)
      || (typeof s.meta.hiddenInSidebar === 'function' && s.meta.hiddenInSidebar()))) {
      return
    }

    const t: Sidebar = {
      path: s.path,
      name: s.name as string,
      meta: s.meta as unknown as Meta,
      children: [],
    };

    (s.children || []).forEach(c => {
      if (c.meta && ((typeof c.meta.hiddenInSidebar === 'boolean' && c.meta.hiddenInSidebar)
        || (typeof c.meta.hiddenInSidebar === 'function' && c.meta.hiddenInSidebar()))) {
        return
      }

      t.children.push((c as unknown as Sidebar))
    })
    res.push(t)
  })

  return res
})
</script>

<template>
  <div class="sidebar">
    <Logo />

    <AMenu
      v-model:open-keys="openKeys"
      v-model:selected-keys="selectedKey"
      mode="inline"
    >
      <EnvIndicator />

      <template v-for="s in visible">
        <AMenuItem
          v-if="s.children.length === 0 || s.meta.hideChildren"
          :key="s.name"
          @click="$router.push(`/${s.path}`).catch(() => {})"
        >
          <Component :is="s.meta.icon as IconComponentProps" />
          <span>{{ s.meta?.name() }}</span>
        </AMenuItem>

        <ASubMenu
          v-else
          :key="s.path"
        >
          <template #title>
            <Component :is="s.meta.icon as IconComponentProps" />
            <span>{{ s?.meta?.name() }}</span>
          </template>
          <AMenuItem
            v-for="child in s.children"
            :key="child.name"
          >
            <RouterLink :to="`/${s.path}/${child.path}`">
              {{ child?.meta?.name() }}
            </RouterLink>
          </AMenuItem>
        </ASubMenu>
      </template>
    </AMenu>
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

.ant-menu-inline, .ant-menu-vertical, .ant-menu-vertical-left {
  border-right: unset;
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
