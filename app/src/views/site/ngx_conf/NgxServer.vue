<script setup lang="ts">
import type { CertificateInfo } from '@/api/cert'
import type { NgxConfig, NgxDirective } from '@/api/ngx'
import Cert from '@/views/site/cert/Cert.vue'
import ConfigTemplate from '@/views/site/ngx_conf/config_template/ConfigTemplate.vue'
import DirectiveEditor from '@/views/site/ngx_conf/directive/DirectiveEditor.vue'
import LocationEditor from '@/views/site/ngx_conf/LocationEditor.vue'
import LogEntry from '@/views/site/ngx_conf/LogEntry.vue'
import { MoreOutlined, PlusOutlined } from '@ant-design/icons-vue'
import { Modal } from 'ant-design-vue'

withDefaults(defineProps<{
  enabled: boolean
  certInfo?: {
    [key: number]: CertificateInfo[]
  }
  context?: 'http' | 'stream'
}>(), {
  context: 'http',
})

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

const autoCert = defineModel<boolean>('autoCert', { default: false })

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
    <ContextHolder />
    <ATabs v-model:active-key="current_server_index">
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
          <Cert
            v-if="current_support_ssl"
            v-model:enabled="autoCert"
            v-model:current_server_directives="ngx_config.servers[current_server_index].directives"
            class="mb-4"
            :site-enabled="enabled"
            :config-name="ngx_config.name"
            :cert-info="certInfo?.[k]"
            :current-server-index="current_server_index"
          />

          <template v-if="v.comments">
            <h3>{{ $gettext('Comments') }}</h3>
            <ATextarea
              v-model:value="v.comments"
              :bordered="false"
            />
          </template>
          <DirectiveEditor />
          <br>
          <ConfigTemplate
            v-if="context === 'http'"
            :current-server-index="current_server_index"
          />
          <br>
          <LocationEditor
            v-if="context === 'http'"
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
