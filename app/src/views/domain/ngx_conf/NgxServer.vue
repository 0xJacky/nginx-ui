<script setup lang="ts">

import { MoreOutlined, PlusOutlined } from '@ant-design/icons-vue'
import { useGettext } from 'vue3-gettext'
import type { ComputedRef, Ref } from 'vue'
import Modal from 'ant-design-vue/lib/modal'
import LogEntry from '@/views/domain/ngx_conf/LogEntry.vue'
import ConfigTemplate from '@/views/domain/ngx_conf/config_template/ConfigTemplate.vue'
import LocationEditor from '@/views/domain/ngx_conf/LocationEditor.vue'
import Cert from '@/views/domain/cert/Cert.vue'
import DirectiveEditor from '@/views/domain/ngx_conf/directive/DirectiveEditor.vue'
import type { NgxConfig, NgxDirective } from '@/api/ngx'
import type { CertificateInfo } from '@/api/cert'

const props = defineProps<{
  autoCert: boolean
  enabled: boolean
  certInfo?: {
    [key: number]: CertificateInfo
  }
}>()

const emit = defineEmits(['callback', 'update:autoCert'])

const { $gettext } = useGettext()

const [modal, ContextHolder] = Modal.useModal()

const current_server_index = inject('current_server_index') as Ref<number>
const route = useRoute()
const name = computed(() => route.params.name) as ComputedRef<string>

const ngx_config = inject('ngx_config') as NgxConfig

const directivesMap = inject('directivesMap') as ComputedRef<Record<string, NgxDirective[]>>

const current_support_ssl = computed(() => {
  if (directivesMap.value.listen) {
    for (const v of directivesMap.value.listen) {
      if (v?.params.indexOf('ssl') > 0)
        return true
    }
  }

  return false
})

const autoCertRef = computed({
  get() {
    return props.autoCert
  },
  set(value) {
    emit('update:autoCert', value)
  },
})

const router = useRouter()

const servers_length = computed(() => {
  return ngx_config.servers.length
})

watch(servers_length, () => {
  if (current_server_index.value >= servers_length.value)
    current_server_index.value = servers_length.value - 1
  else if (current_server_index.value < 0)
    current_server_index.value = 0
})

watch(current_server_index, () => {
  router.push({
    query: {
      server_idx: current_server_index.value.toString(),
    },
  })
})

function add_server() {
  ngx_config.servers.push({
    comments: '',
    locations: [],
    directives: [],
  })
}

function remove_server(index: number) {
  modal.confirm({
    title: $gettext('Do you want to remove this server?'),
    mask: false,
    centered: true,
    okText: $gettext('OK'),
    cancelText: $gettext('Cancel'),
    onOk() {
      ngx_config?.servers?.splice(index, 1)
      current_server_index.value = (index > 1 ? index - 1 : 0)
    },
  })
}

const ngx_directives = computed(() => {
  return ngx_config?.servers?.[current_server_index.value]?.directives
})

provide('ngx_directives', ngx_directives)
</script>

<template>
  <div>
    <h2>Server</h2>
    <ContextHolder />
    <ATabs v-model:activeKey="current_server_index">
      <ATabPane
        v-for="(v, k) in ngx_config.servers"
        :key="k"
      >
        <template #tab>
          Server {{ k + 1 }}
          <ADropdown>
            <MoreOutlined />
            <template #overlay>
              <AMenu>
                <AMenuItem>
                  <a @click="remove_server(k)">{{ $gettext('Delete') }}</a>
                </AMenuItem>
              </AMenu>
            </template>
          </ADropdown>
        </template>
        <LogEntry
          :ngx-config="ngx_config"
          :current-server-idx="current_server_index"
          :name="name"
        />

        <div class="tab-content">
          <template v-if="current_support_ssl && enabled">
            <Cert
              v-if="current_support_ssl"
              v-model:enabled="autoCertRef"
              :config-name="ngx_config.name"
              :cert-info="certInfo?.[k]"
              :current-server-index="current_server_index"
              @callback="$emit('callback')"
            />
          </template>

          <template v-if="v.comments">
            <h3>{{ $gettext('Comments') }}</h3>
            <ATextarea
              v-model:value="v.comments"
              :bordered="false"
            />
          </template>
          <DirectiveEditor />
          <br>
          <ConfigTemplate :current-server-index="current_server_index" />
          <br>
          <LocationEditor
            :current-server-index="current_server_index"
            :locations="v.locations"
          />
        </div>
      </ATabPane>

      <template #rightExtra>
        <AButton
          type="link"
          size="small"
          @click="add_server"
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
