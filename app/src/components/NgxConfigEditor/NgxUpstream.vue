<script setup lang="ts">
import type { Component } from 'vue'
import { ClusterOutlined, MoreOutlined, PlusOutlined } from '@antdv-next/icons'
import { breakpointsAntDesign, useBreakpoints } from '@vueuse/core'
import { Modal } from 'antdv-next'
import { DirectiveEditor, useNgxConfigStore } from '.'
import ConvertUpstreamModal from './ConvertUpstreamModal.vue'
import { siteUpstreamContextKey } from './siteUpstreamContext'
import {
  hasUpstreamServerAddress,
  isUpstreamServerEnabled,
  setUpstreamServerEnabled,
} from './upstreamServer'
import UseUpstreamGroupModal from './UseUpstreamGroupModal.vue'

const props = withDefaults(defineProps<{
  context?: 'http' | 'stream'
}>(), {
  context: 'http',
})

const [modal, ContextHolder] = Modal.useModal()

// Upstream groups are http upstreams, so both group actions are http only.
const isHttp = computed(() => props.context === 'http')

// Only the editor of an existing site provides this; converting needs a site
// file on the server.
const siteContext = inject(siteUpstreamContextKey, undefined)

const isUseGroupOpen = ref(false)

const convertTarget = ref('')
const isConvertOpen = ref(false)

function openConvert(name: string) {
  convertTarget.value = name
  isConvertOpen.value = true
}

async function onConverted() {
  // The block left the site file on the server; load the file again so a
  // later save does not write the stale block back.
  await siteContext?.reload()
}

const ngxConfigStore = useNgxConfigStore()
const { ngxConfig } = storeToRefs(ngxConfigStore)

const currentUpstreamIdx = ref('0')

const upstreamTabItems = computed(() => ngxConfig.value.upstreams?.map((upstream, index) => ({
  key: String(index),
  label: `Upstream ${upstream.name}`,
  upstream,
})) ?? [])

async function addUpstream() {
  if (!ngxConfig.value.upstreams)
    ngxConfig.value.upstreams = []

  ngxConfig.value.upstreams?.push({
    name: '',
    comments: '',
    directives: [],
  })

  rename(ngxConfig.value.upstreams.length - 1, true)
}

function removeUpstream(index: number) {
  modal.confirm({
    title: $gettext('Do you want to remove this upstream?'),
    mask: false,
    centered: true,
    okText: $gettext('OK'),
    cancelText: $gettext('Cancel'),
    onOk() {
      ngxConfig.value.upstreams?.splice(index, 1)
      currentUpstreamIdx.value = String(index > 1 ? index - 1 : 0)
    },
  })
}

function getUpstreamMenuItems(index: number) {
  const items = [
    {
      key: 'rename',
      label: $gettext('Rename'),
      onClick: () => rename(index),
    },
    {
      key: 'delete',
      label: $gettext('Delete'),
      onClick: () => removeUpstream(index),
    },
  ]
  const name = ngxConfig.value.upstreams?.[index]?.name
  if (siteContext && isHttp.value && name) {
    items.push({
      key: 'convert',
      label: $gettext('Convert to Shared Group'),
      onClick: () => openConvert(name),
    })
  }
  return items
}

// The tab bar actions go through the prop: this antdv-next version rendered an
// empty extra area for a #rightExtra slot holding two buttons.
// On a phone the buttons keep only their icons, so the upstream tabs are not
// squeezed out of the bar.
const Button = resolveComponent('AButton')
const isNarrow = useBreakpoints(breakpointsAntDesign).smaller('sm')

function tabBarButton(label: string, icon: Component, onClick: () => void, testId?: string) {
  return h(Button, {
    'type': 'link',
    'size': 'small',
    'title': isNarrow.value ? label : undefined,
    'aria-label': label,
    'data-testid': testId,
    onClick,
  }, { icon: () => h(icon), default: isNarrow.value ? undefined : () => label })
}

const tabBarExtra = computed(() => ({
  right: h('div', { class: 'upstream-tab-actions' }, [
    isHttp.value
      ? tabBarButton($gettext('Use Existing Group'), ClusterOutlined, () => { isUseGroupOpen.value = true }, 'use-upstream-group')
      : null,
    tabBarButton($gettext('Add'), PlusOutlined, addUpstream),
  ]),
}))

const open = ref(false)
const renameIdx = ref(-1)
const buffer = ref('')
// The name dialog also opens for a new upstream; only then it points to the
// existing groups, which is what most people adding an upstream look for.
const isNaming = ref(false)

function rename(idx: number, isNew = false) {
  open.value = true
  isNaming.value = isNew
  renameIdx.value = idx
  buffer.value = ngxConfig.value.upstreams?.[renameIdx.value].name ?? ''
}

function renameOK() {
  if (ngxConfig.value.upstreams?.[renameIdx.value])
    ngxConfig.value.upstreams[renameIdx.value].name = buffer.value
  open.value = false
}
</script>

<template>
  <div>
    <ContextHolder />
    <ATabs
      v-if="ngxConfig.upstreams && ngxConfig.upstreams.length > 0"
      v-model:active-key="currentUpstreamIdx"
      :items="upstreamTabItems"
      :tab-bar-extra-content="tabBarExtra"
    >
      <template #labelRender="{ item, index }">
        Upstream {{ item.upstream.name }}
        <ADropdown>
          <MoreOutlined />
          <template #popupRender>
            <AMenu :items="getUpstreamMenuItems(index)" />
          </template>
        </ADropdown>
      </template>

      <template #contentRender="{ item }">
        <div class="tab-content">
          <AFlex
            v-if="siteContext && isHttp && item.upstream.name"
            wrap
            gap="small"
            align="center"
            justify="space-between"
            class="mb-3"
          >
            <span class="text-gray-500 dark:text-gray-400">
              {{ $gettext('Defined in this site only.') }}
            </span>
            <AButton
              size="small"
              :data-testid="`convert-upstream-${item.upstream.name}`"
              @click="openConvert(item.upstream.name)"
            >
              <ClusterOutlined />
              {{ $gettext('Convert to Shared Group') }}
            </AButton>
          </AFlex>
          <DirectiveEditor v-model:directives="item.upstream.directives">
            <template #directiveSuffix="{ directive }">
              <div
                v-if="directive.directive === 'server'"
                class="upstream-server-toggle"
                @click.stop
              >
                <span>{{ $gettext('Enabled') }}</span>
                <ASwitch
                  :checked="isUpstreamServerEnabled(directive)"
                  :disabled="!hasUpstreamServerAddress(directive)"
                  :aria-label="$gettext('Enabled')"
                  @change="isEnabled => setUpstreamServerEnabled(directive, isEnabled)"
                />
              </div>
            </template>
          </DirectiveEditor>
        </div>
      </template>
    </ATabs>
    <div v-else class="empty-state">
      <AEmpty
        :description="$gettext('No upstreams configured')"
        class="mb-6"
      >
        <template #image>
          <div class="text-6xl mb-4 text-gray-300">
            ⚖️
          </div>
        </template>
      </AEmpty>
      <AFlex
        wrap
        gap="small"
        justify="center"
      >
        <AButton
          v-if="isHttp"
          data-testid="use-upstream-group"
          @click="isUseGroupOpen = true"
        >
          <ClusterOutlined />
          {{ $gettext('Use Existing Group') }}
        </AButton>
        <AButton
          type="primary"
          @click="addUpstream"
        >
          <PlusOutlined />
          {{ $gettext('Add Upstream') }}
        </AButton>
      </AFlex>
    </div>

    <AModal
      v-model:open="open"
      :title="$gettext('Upstream Name')"
      centered
      @ok="renameOK"
    >
      <AForm layout="vertical">
        <AFormItem :label="$gettext('Name')">
          <AInput v-model:value="buffer" />
        </AFormItem>
      </AForm>
      <p
        v-if="isNaming && isHttp"
        class="mb-0 text-gray-500 dark:text-gray-400"
        data-testid="upstream-name-group-hint"
      >
        {{ $gettext('This adds an upstream block to this site only. To proxy to an upstream group shared with other sites, use "Use Existing Group" instead.') }}
      </p>
    </AModal>

    <UseUpstreamGroupModal
      v-if="isHttp"
      v-model:open="isUseGroupOpen"
    />

    <ConvertUpstreamModal
      v-if="siteContext && isHttp"
      v-model:open="isConvertOpen"
      :site="siteContext.siteName.value"
      :upstream="convertTarget"
      :context="siteContext"
      @converted="onConverted"
    />
  </div>
</template>

<style scoped lang="less">
.empty-state {
  @apply px-8 text-center;
  min-height: 200px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
}

.upstream-tab-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.upstream-server-toggle {
  display: flex;
  gap: 8px;
  align-items: center;
  white-space: nowrap;
}
</style>
