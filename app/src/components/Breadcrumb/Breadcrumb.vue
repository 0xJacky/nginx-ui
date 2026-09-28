<script setup lang="ts">
import type { RouteLocationRaw } from 'vue-router'
import type { CrumbEntry } from './crumbs'
import type { Bread } from '@/components/Breadcrumb/types'
import { DatabaseOutlined, DownOutlined, EllipsisOutlined } from '@antdv-next/icons'
import { storeToRefs } from 'pinia'
import NodeSwitcher from '@/components/NodeSwitcher'
import { useBreadcrumbs } from '@/composables/useBreadcrumbs'
import { useSettingsStore } from '@/pinia'
import { collapseCrumbs, withoutHome } from './crumbs'

const props = defineProps<{
  /** Lead with the current node, for when the sidebar pill cannot show its name. */
  showNode?: boolean
  /** Only the current page, for phone-width headers. */
  compact?: boolean
}>()

interface CrumbView {
  kind: 'link' | 'text' | 'collapsed'
  label: string
  to?: RouteLocationRaw
  hidden?: Bread[]
}

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

onMounted(() => {
  breadList.value = computedBreadList.value
})

watch(route, () => {
  breadList.value = computedBreadList.value
})

function linkTo(bread: Bread): RouteLocationRaw {
  return { path: bread.path || '/', query: bread.query }
}

const entries = computed<CrumbEntry[]>(() => {
  const breads = withoutHome(breadList.value)
  if (props.compact)
    return breads.slice(-1).map(bread => ({ type: 'item', bread }))

  return collapseCrumbs(breads)
})

const views = computed<CrumbView[]>(() => entries.value.map((entry, index) => {
  if (entry.type === 'collapsed')
    return { kind: 'collapsed', label: '', hidden: entry.breads }

  const { bread } = entry
  const isLast = index === entries.value.length - 1
  if (!isLast && bread.path)
    return { kind: 'link', label: bread.translatedName(), to: linkTo(bread) }

  return { kind: 'text', label: bread.translatedName() }
}))

const breadcrumbItems = computed(() => views.value.map((_, index) => ({ key: String(index) })))

function viewOf(item: { key?: string | number }) {
  return views.value[Number(item.key)]
}

function hiddenMenu(view: CrumbView) {
  return {
    items: (view.hidden ?? []).map((bread, index) => ({ key: String(index), label: bread.translatedName() })),
  }
}

function openHidden(view: CrumbView, key: string | number) {
  const bread = view.hidden?.[Number(key)]
  if (bread)
    router.push(linkTo(bread))
}

const settingsStore = useSettingsStore()
const { node, server_name } = storeToRefs(settingsStore)
const isSwitcherOpen = ref(false)
const isLocal = computed(() => node.value.id === 0)
const nodeName = computed(() => isLocal.value ? (server_name.value || $gettext('Local')) : node.value.name)
</script>

<template>
  <div class="header-breadcrumb">
    <template v-if="showNode">
      <NodeSwitcher v-model:open="isSwitcherOpen">
        <button
          type="button"
          class="node-crumb"
          :class="{ remote: !isLocal }"
          :aria-expanded="isSwitcherOpen"
        >
          <DatabaseOutlined />
          <span class="node-crumb-name">{{ nodeName }}</span>
          <DownOutlined class="node-crumb-chevron" />
        </button>
      </NodeSwitcher>
      <span v-if="views.length" class="node-crumb-separator">/</span>
    </template>

    <ABreadcrumb :items="breadcrumbItems">
      <template #itemRender="{ route: item }">
        <ADropdown
          v-if="viewOf(item)?.kind === 'collapsed'"
          :menu="hiddenMenu(viewOf(item))"
          :trigger="['click']"
          @menu-click="({ key }) => openHidden(viewOf(item), key)"
        >
          <a class="collapsed-crumb" :aria-label="$gettext('Expand')"><EllipsisOutlined /></a>
        </ADropdown>
        <RouterLink v-else-if="viewOf(item)?.kind === 'link'" :to="viewOf(item).to!">
          {{ viewOf(item).label }}
        </RouterLink>
        <span v-else>{{ viewOf(item)?.label }}</span>
      </template>
    </ABreadcrumb>
  </div>
</template>

<style scoped lang="less">
// Sized to its content so the header can measure how much room the trail
// really needs; the header clips it when space runs out.
.header-breadcrumb {
  display: flex;
  align-items: center;
  width: max-content;
}

.node-crumb {
  display: flex;
  align-items: center;
  gap: 6px;
  height: 24px;
  padding: 0 10px;
  border-radius: 12px;
  border: 1px solid #91d5ff;
  background: #e6f7ff;
  color: #096dd9;
  font: inherit;
  font-size: 13px;
  cursor: pointer;
  transition: border-color 0.2s;

  &:hover {
    border-color: #1677ff;
  }

  &:focus-visible {
    outline: 2px solid #1677ff;
    outline-offset: 1px;
  }

  &.remote {
    border-color: #ffd591;
    background: #fff7e6;
    color: #d46b08;

    &:hover {
      border-color: #fa8c16;
    }
  }

  > .anticon {
    display: flex;
  }

  .node-crumb-name {
    max-width: 160px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    line-height: 20px;
  }

  .node-crumb-chevron {
    font-size: 10px;
    opacity: 0.7;
  }
}

// Matches the Breadcrumb separator's spacing and colour.
.node-crumb-separator {
  margin-inline: 8px;
  color: rgba(0, 0, 0, 0.45);
}

.collapsed-crumb {
  display: inline-flex;
  align-items: center;
}

.dark {
  .node-crumb {
    border-color: #545454;
    background: transparent;
    color: #bebebe;

    &:hover {
      border-color: #8c8c8c;
    }

    &.remote {
      border-color: rgba(216, 150, 20, 0.5);
      color: #d89614;

      &:hover {
        border-color: #d89614;
      }
    }
  }

  .node-crumb-separator {
    color: rgba(255, 255, 255, 0.45);
  }
}
</style>
