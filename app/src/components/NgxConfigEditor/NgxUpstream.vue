<script setup lang="ts">
import { MoreOutlined, PlusOutlined } from '@antdv-next/icons'
import { Modal } from 'antdv-next'
import { DirectiveEditor, useNgxConfigStore } from '.'

const [modal, ContextHolder] = Modal.useModal()

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
      currentUpstreamIdx.value = String(index > 1 ? index - 1 : 0)
    },
  })
}

function getUpstreamMenuItems(index: number) {
  return [
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
}

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
</script>

<template>
  <div>
    <ContextHolder />
    <ATabs
      v-if="ngxConfig.upstreams && ngxConfig.upstreams.length > 0"
      v-model:active-key="currentUpstreamIdx"
      :items="upstreamTabItems"
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
          <DirectiveEditor v-model:directives="item.upstream.directives" />
        </div>
      </template>

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
  min-height: 200px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
}
</style>
