<script setup lang="ts">
import { MoreOutlined, PlusOutlined } from '@ant-design/icons-vue'
import { Modal } from 'ant-design-vue'
import _ from 'lodash'
import type { NgxConfig, NgxDirective } from '@/api/ngx'
import DirectiveEditor from '@/views/domain/ngx_conf/directive/DirectiveEditor.vue'
import type { UpstreamStatus } from '@/api/upstream'
import upstream from '@/api/upstream'

const [modal, ContextHolder] = Modal.useModal()

const ngx_config = inject('ngx_config') as NgxConfig
const current_upstream_index = ref(0)
async function add_upstream() {
  if (!ngx_config.upstreams)
    ngx_config.upstreams = []

  ngx_config.upstreams?.push({
    name: '',
    comments: '',
    directives: [],
  })

  rename(ngx_config.upstreams.length - 1)
}

function remove_upstream(index: number) {
  modal.confirm({
    title: $gettext('Do you want to remove this upstream?'),
    mask: false,
    centered: true,
    okText: $gettext('OK'),
    cancelText: $gettext('Cancel'),
    onOk() {
      ngx_config?.upstreams?.splice(index, 1)
      current_upstream_index.value = (index > 1 ? index - 1 : 0)
    },
  })
}

const ngx_directives = computed(() => {
  return ngx_config?.upstreams?.[current_upstream_index.value]?.directives
})

provide('ngx_directives', ngx_directives)

const open = ref(false)
const renameIdx = ref(-1)
const buffer = ref('')

function rename(idx: number) {
  open.value = true
  renameIdx.value = idx
  buffer.value = ngx_config?.upstreams?.[renameIdx.value].name ?? ''
}

function ok() {
  if (ngx_config?.upstreams?.[renameIdx.value])
    ngx_config.upstreams[renameIdx.value].name = buffer.value
  open.value = false
}

const availabilityResult = ref({}) as Ref<Record<string, UpstreamStatus>>
const websocket = ref()
function availability_test() {
  const sockets: string[] = []
  for (const u of ngx_config.upstreams ?? []) {
    for (const d of u.directives ?? []) {
      if (d.directive === 'server')
        sockets.push(d.params.split(' ')[0])
    }
  }

  if (sockets.length > 0) {
    websocket.value = upstream.availability_test()
    websocket.value.onopen = () => {
      websocket.value.send(JSON.stringify(sockets))
    }
    websocket.value.onmessage = (e: MessageEvent) => {
      availabilityResult.value = JSON.parse(e.data)
    }
  }
}

onMounted(() => {
  availability_test()
})

onBeforeUnmount(() => {
  websocket.value?.close()
})

async function _restartTest() {
  websocket.value?.close()
  availability_test()
}

const restartTest = _.throttle(_restartTest, 5000)

watch(ngx_directives, () => {
  restartTest()
}, { deep: true })
</script>

<template>
  <div>
    <ContextHolder />
    <ATabs
      v-if="ngx_config.upstreams && ngx_config.upstreams.length > 0"
      v-model:active-key="current_upstream_index"
    >
      <ATabPane
        v-for="(v, k) in ngx_config.upstreams"
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
                  <a @click="remove_upstream(k)">{{ $gettext('Delete') }}</a>
                </AMenuItem>
              </AMenu>
            </template>
          </ADropdown>
        </template>

        <div class="tab-content">
          <DirectiveEditor>
            <template #directiveSuffix="{ directive }: {directive: NgxDirective}">
              <template v-if="availabilityResult[directive.params]?.online">
                <ABadge color="green" />
                {{ availabilityResult[directive.params]?.latency.toFixed(2) }}ms
              </template>
            </template>
          </DirectiveEditor>
        </div>
      </ATabPane>

      <template #rightExtra>
        <AButton
          type="link"
          size="small"
          @click="add_upstream"
        >
          <PlusOutlined />
          {{ $gettext('Add') }}
        </AButton>
      </template>
    </ATabs>
    <div v-else>
      <AEmpty />
      <div class="flex justify-center">
        <AButton
          type="primary"
          @click="add_upstream"
        >
          {{ $gettext('Create') }}
        </AButton>
      </div>
    </div>

    <AModal
      v-model:open="open"
      :title="$gettext('Upstream Name')"
      centered
      @ok="ok"
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

</style>
