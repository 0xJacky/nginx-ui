<script setup lang="ts">
import { DesktopOutlined, HomeOutlined, LogoutOutlined, MenuUnfoldOutlined } from '@antdv-next/icons'
import { useElementSize } from '@vueuse/core'
import auth from '@/api/auth'
import Breadcrumb from '@/components/Breadcrumb'
import NginxControl from '@/components/NginxControl'
import Notification from '@/components/Notification'
import ProcessingStatus from '@/components/ProcessingStatus'
import RecoveryCodeMigrationWarning from '@/components/RecoveryCodeMigrationWarning'
import { SelfCheckHeaderBanner } from '@/components/SelfCheck'
import SetLanguage from '@/components/SetLanguage'
import SwitchAppearance from '@/components/SwitchAppearance'
import { isWorkspacePane } from '@/lib/workspace/env'
import { useSettingsStore } from '@/pinia'

const props = defineProps<{
  /** The sidebar pill is not showing the node name (collapsed or in the drawer). */
  showNode?: boolean
  /**
   * Phone width: the menu button replaces the sidebar, and the icons leave no
   * room for the breadcrumb, which BaseLayout then renders below the header.
   */
  isMobile?: boolean
}>()

const emit = defineEmits<{
  clickUnFold: [void]
}>()

// Horizontal gap between header items; keep in sync with `.header` below.
const ITEM_GAP = 16

const router = useRouter()
const { message } = useGlobalApp()

function logout() {
  auth.logout().then(() => {
    message.success($gettext('Logout successful'))
  }).then(() => {
    router.push('/login')
  })
}

const headerRef = useTemplateRef('headerRef') as Ref<HTMLElement>
const breadcrumbSlotRef = useTemplateRef('breadcrumbSlotRef')
const breadcrumbRef = useTemplateRef('breadcrumbRef')
const toolRef = useTemplateRef('toolRef')
const userWrapperRef = useTemplateRef('userWrapperRef')
const selfCheckRef = useTemplateRef('selfCheckRef')
const recoveryRef = useTemplateRef('recoveryRef')

// Inside a workspace pane the header is a slim bar: theme, language, home,
// logout and the workspace entry live in the workspace top bar instead.
const isWorkspace = isWorkspacePane

const settingsStore = useSettingsStore()
// The workspace opens (or brings forward) a tab of the node shown here.
const workspaceLink = computed(() => ({ path: '/workspace', query: { l: String(settingsStore.node.id) } }))

// The breadcrumb is sized to its content, so this is the room it wants rather
// than the room it got.
const { width: breadcrumbWidth } = useElementSize(breadcrumbRef)
const { width: headerWidth } = useElementSize(headerRef)
const { width: userWrapperWidth } = useElementSize(userWrapperRef)
const { width: selfCheckWidth } = useElementSize(selfCheckRef)
const { width: breadcrumbSlotWidth } = useElementSize(breadcrumbSlotRef)
const { width: toolWidth } = useElementSize(toolRef)
const { width: recoveryWidth } = useElementSize(recoveryRef)

// What is left for the banners once the breadcrumb and the icons are placed:
// a banner sits between the two, one gap on each side. The banners collapse to
// icons instead of squeezing the breadcrumb.
const bannerSpace = computed(() => {
  const toolSpace = props.isMobile ? toolWidth.value + ITEM_GAP : 0
  return headerWidth.value - toolSpace - breadcrumbWidth.value - userWrapperWidth.value - ITEM_GAP * 2
})
const recoveryBannerSpace = computed(() => bannerSpace.value - (selfCheckWidth.value ? selfCheckWidth.value + ITEM_GAP : 0))

// A full alert is centered in the header, like the page title bar of a desktop
// window. Both sides then grow from zero so the banners sit in the middle, and
// the breadcrumb keeps all the room it can get, pushing the banners off center
// rather than being clipped. A collapsed icon stays next to the header icons.
const isBannerCentered = computed(() => !!(selfCheckRef.value?.isExpanded || recoveryRef.value?.isExpanded))
const breadcrumbMinWidth = computed(() => {
  const toolSpace = props.isMobile ? toolWidth.value + ITEM_GAP : 0
  const bannerCount = (selfCheckWidth.value ? 1 : 0) + (recoveryWidth.value ? 1 : 0)
  const rest = headerWidth.value - toolSpace - userWrapperWidth.value
    - selfCheckWidth.value - recoveryWidth.value - ITEM_GAP * (bannerCount + 1)
  return Math.max(0, Math.min(breadcrumbWidth.value, rest))
})

const isBreadcrumbClipped = computed(() => breadcrumbWidth.value > breadcrumbSlotWidth.value + 0.5)
</script>

<template>
  <div ref="headerRef" class="header" :class="{ 'in-pane': isWorkspace }">
    <span v-if="isMobile" ref="toolRef" class="tool">
      <MenuUnfoldOutlined @click="emit('clickUnFold')" />
    </span>

    <div
      ref="breadcrumbSlotRef"
      class="breadcrumb-slot"
      :class="{ overflowing: isBreadcrumbClipped, centering: isBannerCentered }"
      :style="isBannerCentered ? { minWidth: `${breadcrumbMinWidth}px` } : undefined"
    >
      <div v-if="!isMobile" ref="breadcrumbRef" class="breadcrumb-measure">
        <Breadcrumb :show-node="showNode" />
      </div>
    </div>

    <SelfCheckHeaderBanner
      ref="selfCheckRef"
      class="header-banner"
      :available-width="bannerSpace"
    />

    <RecoveryCodeMigrationWarning
      ref="recoveryRef"
      class="header-banner"
      :available-width="recoveryBannerSpace"
    />

    <div class="user-slot" :class="{ centering: isBannerCentered }">
      <ASpace
        ref="userWrapperRef"
        class="user-wrapper"
        :size="isMobile || isWorkspace ? 16 : 24"
      >
        <template v-if="!isWorkspace">
          <SetLanguage class="set_lang" />

          <SwitchAppearance />

          <div class="workspace-entry">
            <RouterLink :to="workspaceLink">
              <ATooltip :title="$gettext('Workspace')">
                <DesktopOutlined />
              </ATooltip>
            </RouterLink>
          </div>

          <ProcessingStatus />
        </template>

        <Notification :header-ref="headerRef" />

        <NginxControl />

        <template v-if="!isWorkspace">
          <a href="/">
            <HomeOutlined />
          </a>

          <a @click="logout">
            <LogoutOutlined />
          </a>
        </template>
      </ASpace>
    </div>
  </div>
</template>

<style lang="less" scoped>
.header {
  height: 64px;
  padding: 0 28px 0 24px;
  background: transparent;
  box-shadow: 0 0 20px 0 rgba(0, 0, 0, 0.05);
  width: 100%;
  position: relative;
  display: flex;
  align-items: center;
  gap: 16px;

  a {
    color: #000000;
  }

  @media (max-width: 600px) {
    padding: 0 20px;
  }

  // Keep in sync with the pane header height below.
  &.in-pane {
    height: 40px;
    padding: 0 16px;
    gap: 12px;
  }
}

.dark {
  .header {
    box-shadow: 1px 1px 0 0 #404040;

    a {
      color: #fafafa;
    }
  }
}

.tool {
  flex: none;
  display: flex;
}

.breadcrumb-slot {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  display: flex;
  align-items: center;
  height: 100%;

  // The trail is clipped, not wrapped; fade the cut edge so it reads as more.
  &.overflowing {
    mask-image: linear-gradient(to right, #000 calc(100% - 32px), transparent);
  }

  &.centering {
    flex: 1 1 0;
  }
}

.breadcrumb-measure {
  flex: none;
}

.header-banner {
  flex: none;
}

.workspace-entry {
  @media (max-width: 600px) {
    display: none;
  }
}

.user-slot {
  flex: none;
  display: flex;
  justify-content: flex-end;
  margin-left: auto;

  &.centering {
    flex: 1 1 0;
  }
}

.user-wrapper {
  flex: none;
}

.set_lang {
  display: inline;
}
</style>

<style lang="less">
// Workspace panes (see lib/workspace/env.ts) trade the 64px header for a slim
// bar. BaseLayout owns the Layout.Header element, so its height is set here,
// together with the sidebar logo that lines up with it.
@pane-header-height: 40px;

html.workspace-pane {
  .ant-layout-header {
    height: @pane-header-height;
    line-height: @pane-header-height;
  }

  .sidebar .logo {
    height: @pane-header-height;

    img {
      height: 28px;
    }

    p.text {
      font-size: 18px;
      line-height: @pane-header-height;
      height: @pane-header-height;
    }
  }
}

// A split pane without focus: its header takes the workspace background and
// fades, like an inactive window's title bar, so the focused pane stands out
// without covering either page. Overrides the header colours set in App.vue.
html.workspace-pane.workspace-pane-inactive {
  .ant-layout-header {
    background-color: #f0f2f5 !important;
    transition: background-color 0.15s;

    .header > * {
      opacity: 0.5;
      transition: opacity 0.15s;
    }
  }

  .dark .ant-layout-header {
    background-color: #000000 !important;
  }
}
</style>
