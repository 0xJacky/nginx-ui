<script setup lang="ts">
import { MoreOutlined, PlusOutlined } from '@antdv-next/icons'
import { Modal } from 'antdv-next'
import { DirectiveEditor, Http, LocationEditor, LogEntry, useNgxConfigStore } from '.'

withDefaults(defineProps<{
  context?: 'http' | 'stream'
}>(), {
  context: 'http',
})

const [modal, ContextHolder] = Modal.useModal()
const ngxConfigStore = useNgxConfigStore()
const { ngxConfig, curServerIdx } = storeToRefs(ngxConfigStore)

const route = useRoute()
const name = computed(() => route.params.name) as ComputedRef<string>

const router = useRouter()

const serversLength = computed(() => {
  return ngxConfig.value.servers?.length ?? 0
})

const hasServers = computed(() => {
  return serversLength.value > 0
})

const activeServerKey = computed({
  get: () => curServerIdx.value.toString(),
  set: value => {
    curServerIdx.value = Number(value)
  },
})

// The tab content reads its server from the store by index, never from the
// item: a tab pane keeps the slot content of its first render, so an item's
// server would stay the object loaded back then. Once a save or reload
// replaces the config, edits made there would land on that detached copy,
// stay visible, and never reach the saved file.
const serverTabItems = computed(() => ngxConfig.value.servers?.map((_, index) => ({
  key: String(index),
  label: `Server ${index + 1}`,
})) ?? [])

watch(serversLength, () => {
  // An empty config, e.g. while another site is loading, must not park the tab
  // index on -1.
  if (curServerIdx.value >= serversLength.value)
    curServerIdx.value = Math.max(serversLength.value - 1, 0)
  else if (curServerIdx.value < 0)
    curServerIdx.value = 0
})

watch(curServerIdx, () => {
  router.push({
    query: {
      server_idx: curServerIdx.value.toString(),
    },
  })
})

function addServer() {
  if (!ngxConfig.value.servers)
    ngxConfig.value.servers = []

  ngxConfig.value.servers.push({
    comments: '',
    locations: [],
    directives: [],
  })
}

function removeServer(index: number) {
  modal.confirm({
    title: $gettext('Do you want to remove this server?'),
    mask: false,
    centered: true,
    okText: $gettext('OK'),
    cancelText: $gettext('Cancel'),
    onOk() {
      ngxConfig.value.servers?.splice(index, 1)
      curServerIdx.value = (index > 1 ? index - 1 : 0)
    },
  })
}

function getServerMenuItems(index: number) {
  return [
    {
      key: 'delete',
      label: $gettext('Delete'),
      onClick: () => removeServer(index),
    },
  ]
}
</script>

<template>
  <div>
    <ContextHolder />

    <!-- Empty State -->
    <div v-if="!hasServers" class="empty-state">
      <AEmpty
        :description="$gettext('No servers configured')"
        class="mb-6"
      >
        <template #image>
          <div class="text-6xl mb-4 text-gray-300">
            🖥️
          </div>
        </template>
      </AEmpty>
      <div class="text-center">
        <AButton
          type="primary"
          @click="addServer"
        >
          <PlusOutlined />
          {{ $gettext('Add Server') }}
        </AButton>
      </div>
    </div>

    <!-- Server Tabs -->
    <ATabs
      v-else
      v-model:active-key="activeServerKey"
      :items="serverTabItems"
    >
      <template #labelRender="{ index }">
        Server {{ index + 1 }}
        <ADropdown>
          <MoreOutlined />
          <template #popupRender>
            <AMenu :items="getServerMenuItems(index)" />
          </template>
        </ADropdown>
      </template>

      <template #contentRender="{ index }">
        <LogEntry class="mb-4" :ngx-config :cur-server-idx :name :context />

        <div v-if="ngxConfig.servers[index]" class="tab-content">
          <slot name="tab-content" :tab-idx="index" />

          <template v-if="ngxConfig.servers[index].comments">
            <h3>{{ $gettext('Comments') }}</h3>
            <ATextarea
              v-model:value="ngxConfig.servers[index].comments"
              variant="borderless"
            />
          </template>
          <DirectiveEditor v-model:directives="ngxConfig.servers[index].directives" class="mb-4" />
          <LocationEditor
            v-if="context === Http"
            v-model:locations="ngxConfig.servers[index].locations"
          >
            <template #label-extra="{ index: locationIdx }">
              <slot name="location-label" :server-idx="index" :location-idx="locationIdx" />
            </template>
            <template #form-extra="{ index: locationIdx }">
              <slot name="location-form" :server-idx="index" :location-idx="locationIdx" />
            </template>
          </LocationEditor>
        </div>
      </template>

      <template #rightExtra>
        <AButton
          type="link"
          size="small"
          @click="addServer"
        >
          <PlusOutlined />
          {{ $gettext('Add') }}
        </AButton>
      </template>
    </ATabs>
  </div>
</template>

<style scoped lang="less">
.empty-state {
  @apply px-8 text-center;
  min-height: 400px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
}
</style>
