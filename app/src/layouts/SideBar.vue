<script setup lang="ts">
import { useRoute } from 'vue-router'
import type { ComputedRef } from 'vue'
import { computed, ref, watch } from 'vue'
import type { AntdIconType } from '@ant-design/icons-vue/lib/components/AntdIcon'
import Logo from '@/components/Logo/Logo.vue'
import { routes } from '@/routes'
import EnvIndicator from '@/components/EnvIndicator/EnvIndicator.vue'

const route = useRoute()

const openKeys = [openSub()]

const selectedKey = ref([route.name])

function openSub() {
  const path = route.path
  const lastSepIndex = path.lastIndexOf('/')

  return path.substring(1, lastSepIndex)
}

watch(route, () => {
  selectedKey.value = [route.name]

  const sub = openSub()
  const p = openKeys.indexOf(sub)
  if (p === -1)
    openKeys.push(sub)
})

const sidebars = computed(() => {
  return routes[0].children
})

interface meta {
  icon: AntdIconType
  hiddenInSidebar: boolean
  hideChildren: boolean
}

interface sidebar {
  path: string
  name: () => string
  meta: meta
  children: sidebar[]
}

const visible: ComputedRef<sidebar[]> = computed(() => {
  const res: sidebar[] = [];

  (sidebars.value || []).forEach(s => {
    if (s.meta && s.meta.hiddenInSidebar)
      return

    const t: sidebar = {
      path: s.path,
      name: s.name,
      meta: s.meta as meta,
      children: [],
    };

    (s.children || []).forEach(c => {
      if (c.meta && c.meta.hiddenInSidebar)
        return

      t.children.push((c as sidebar))
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
      v-model:openKeys="openKeys"
      v-model:selectedKeys="selectedKey"
      :open-keys="openKeys"
      mode="inline"
    >
      <EnvIndicator />

      <template v-for="s in visible">
        <AMenuItem
          v-if="s.children.length === 0 || s.meta.hideChildren"
          :key="s.name"
          @click="$router.push(`/${s.path}`).catch(() => {})"
        >
          <component :is="s.meta.icon" />
          <span>{{ s.name() }}</span>
        </AMenuItem>

        <ASubMenu
          v-else
          :key="s.path"
        >
          <template #title>
            <component :is="s.meta.icon" />
            <span>{{ s.name() }}</span>
          </template>
          <AMenuItem
            v-for="child in s.children"
            :key="child.name"
          >
            <RouterLink :to="`/${s.path}/${child.path}`">
              {{ child.name() }}
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
