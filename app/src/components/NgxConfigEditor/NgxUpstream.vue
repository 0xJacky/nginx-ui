<script setup lang="ts">
import type ReconnectingWebSocket from 'reconnecting-websocket'
import type { NgxDirective } from '@/api/ngx'
import type { UpstreamStatus } from '@/api/upstream'
import { MoreOutlined, PlusOutlined } from '@ant-design/icons-vue'
import { Modal } from 'ant-design-vue'
import { throttle } from 'lodash'
import upstream from '@/api/upstream'
import { DirectiveEditor, Server, useNgxConfigStore } from '.'

const [modal, ContextHolder] = Modal.useModal()

const ngxConfigStore = useNgxConfigStore()
const { ngxConfig } = storeToRefs(ngxConfigStore)

const currentUpstreamIdx = ref(0)

async function addUpstream() {
  if (!ngxConfig.value.upstreams)
    ngxConfig.value.upstreams = []

  ngxConfig.value.upstreams?.push({
    name: '',
    comments: '',
    directives: [],
  })

  rename(ngxConfig.value.upstreams.length - 1)
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
      currentUpstreamIdx.value = (index > 1 ? index - 1 : 0)
    },
  })
}

const curUptreamDirectives = computed(() => {
  return ngxConfig.value.upstreams?.[currentUpstreamIdx.value]?.directives
})

const open = ref(false)
const renameIdx = ref(-1)
const buffer = ref('')

function rename(idx: number) {
  open.value = true
  renameIdx.value = idx
  buffer.value = ngxConfig.value.upstreams?.[renameIdx.value].name ?? ''
}

function renameOK() {
  if (ngxConfig.value.upstreams?.[renameIdx.value])
    ngxConfig.value.upstreams[renameIdx.value].name = buffer.value
  open.value = false
}

const availabilityResult = ref({}) as Ref<Record<string, UpstreamStatus>>
const websocket = shallowRef<ReconnectingWebSocket | WebSocket>()

function availabilityTest() {
  const sockets: string[] = []
  for (const u of ngxConfig.value.upstreams ?? []) {
    for (const d of u.directives ?? []) {
      if (d.directive === Server)
        sockets.push(d.params.split(' ')[0])
    }
  }

  if (sockets.length > 0) {
    websocket.value = upstream.availability_test()
    websocket.value.onopen = () => {
      websocket.value!.send(JSON.stringify(sockets))
    }
    websocket.value.onmessage = (e: MessageEvent) => {
      availabilityResult.value = JSON.parse(e.data)
    }
  }
}

onMounted(() => {
  availabilityTest()
})

onBeforeUnmount(() => {
  websocket.value?.close()
})

async function _restartTest() {
  websocket.value?.close()
  availabilityTest()
}

const restartTest = throttle(_restartTest, 5000)

watch(curUptreamDirectives, () => {
  restartTest()
}, { deep: true })

function getAvailabilityResult(directive: NgxDirective) {
  const params = directive.params.split(' ')
  return availabilityResult.value[params?.[0]]
}
</script>

<template>
  <div>
    <ContextHolder />
    <ATabs
      v-if="ngxConfig.upstreams && ngxConfig.upstreams.length > 0"
      v-model:active-key="currentUpstreamIdx"
    >
      <ATabPane
        v-for="(v, k) in ngxConfig.upstreams"
        :key="k"
      >
        <template #tab>
          Upstream {{ v.name }}
          <ADropdown>
            <MoreOutlined />
            <template #overlay>
              <AMenu>
                <AMenuItem>
                  <a @click="rename(k)">{{ $gettext('Rename') }}</a>
                </AMenuItem>
                <AMenuItem>
                  <a @click="removeUpstream(k)">{{ $gettext('Delete') }}</a>
                </AMenuItem>
              </AMenu>
            </template>
          </ADropdown>
        </template>

        <div class="tab-content">
          <DirectiveEditor v-model:directives="v.directives">
            <template #directiveSuffix="{ directive }: {directive: NgxDirective}">
              <template v-if="directive.directive === Server">
                <template v-if="getAvailabilityResult(directive)?.online">
                  <ABadge color="green" />
                  {{ getAvailabilityResult(directive)?.latency?.toFixed(2) }}ms
                </template>
                <template v-else>
                  <ABadge color="red" />
                  {{ $gettext('Offline') }}
                </template>
              </template>
            </template>
          </DirectiveEditor>
        </div>
      </ATabPane>

      <template #rightExtra>
        <AButton
          type="link"
          size="small"
          @click="addUpstream"
        >
          <PlusOutlined />
          {{ $gettext('Add') }}
        </AButton>
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
      <div class="text-center">
        <AButton
          type="primary"
          @click="addUpstream"
        >
          <PlusOutlined />
          {{ $gettext('Add Upstream') }}
        </AButton>
      </div>
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
    </AModal>
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
