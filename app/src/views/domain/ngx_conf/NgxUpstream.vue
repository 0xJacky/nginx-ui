<script setup lang="ts">
import { MoreOutlined, PlusOutlined } from '@ant-design/icons-vue'
import { useGettext } from 'vue3-gettext'
import Modal from 'ant-design-vue/lib/modal'
import type { NgxConfig } from '@/api/ngx'
import DirectiveEditor from '@/views/domain/ngx_conf/directive/DirectiveEditor.vue'

const { $gettext } = useGettext()

const [modal, ContextHolder] = Modal.useModal()

const ngx_config = inject('ngx_config') as NgxConfig
const current_upstream_index = ref(0)
function add_upstream() {
  ngx_config.upstreams?.push({
    name: '',
    comments: '',
    directives: [],
  })
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
</script>

<template>
  <div>
    <h2>Upstream</h2>
    <ContextHolder />
    <ATabs v-model:activeKey="current_upstream_index">
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
                  <a @click="remove_upstream(k)">{{ $gettext('Delete') }}</a>
                </AMenuItem>
              </AMenu>
            </template>
          </ADropdown>
        </template>

        <div class="tab-content">
          <div class="mb-4">
            <h2>{{ $gettext('Name') }}</h2>
            <AInput v-model:value="v.name" />
          </div>

          <div class="mb-4">
            <h2>{{ $gettext('Comments') }}</h2>
            <ATextarea v-model:value="v.comments" />
          </div>

          <DirectiveEditor />
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
  </div>
</template>

<style scoped lang="less">

</style>
