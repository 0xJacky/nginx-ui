<script setup lang="ts">
import { MoreOutlined, PlusOutlined } from '@ant-design/icons-vue'
import { Modal } from 'ant-design-vue'
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

watch(serversLength, () => {
  if (curServerIdx.value >= serversLength.value)
    curServerIdx.value = serversLength.value - 1
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
    <ATabs v-else v-model:active-key="curServerIdx">
      <ATabPane
        v-for="(v, k) in ngxConfig.servers"
        :key="k"
      >
        <template #tab>
          Server {{ k + 1 }}
          <ADropdown>
            <MoreOutlined />
            <template #overlay>
              <AMenu>
                <AMenuItem>
                  <a @click="removeServer(k)">{{ $gettext('Delete') }}</a>
                </AMenuItem>
              </AMenu>
            </template>
          </ADropdown>
        </template>

        <LogEntry class="mb-4" :ngx-config :cur-server-idx :name />

        <div class="tab-content">
          <slot name="tab-content" :tab-idx="k" />

          <template v-if="v.comments">
            <h3>{{ $gettext('Comments') }}</h3>
            <ATextarea
              v-model:value="v.comments"
              :bordered="false"
            />
          </template>
          <DirectiveEditor v-model:directives="v.directives" class="mb-4" />
          <LocationEditor
            v-if="context === Http"
            v-model:locations="v.locations"
          />
        </div>
      </ATabPane>

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
