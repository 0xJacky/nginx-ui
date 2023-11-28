<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'
import { useGettext } from 'vue3-gettext'
import { MoreOutlined, PlusOutlined } from '@ant-design/icons-vue'
import Modal from 'ant-design-vue/lib/modal'
import type { ComputedRef, Ref } from 'vue'
import { provide } from 'vue'
import DirectiveEditor from '@/views/domain/ngx_conf/directive/DirectiveEditor.vue'
import LocationEditor from '@/views/domain/ngx_conf/LocationEditor.vue'
import Cert from '@/views/domain/cert/Cert.vue'
import LogEntry from '@/views/domain/ngx_conf/LogEntry.vue'
import ConfigTemplate from '@/views/domain/ngx_conf/config_template/ConfigTemplate.vue'
import CodeEditor from '@/components/CodeEditor/CodeEditor.vue'
import template from '@/api/template'
import type { NgxConfig, NgxDirective } from '@/api/ngx'
import type { CertificateInfo } from '@/api/cert'

const props = defineProps<{
  autoCert: boolean
  enabled: boolean
  certInfo?: {
    [key: number]: CertificateInfo
  }
}>()

const emit = defineEmits(['callback', 'update:auto_cert'])

const { $gettext } = useGettext()

const save_site_config = inject('save_site_config')!

const route = useRoute()

const current_server_index = ref(0)
const name = ref(route.params.name) as Ref<string>

const ngx_config = inject('ngx_config') as NgxConfig

function confirm_change_tls(status: boolean) {
  Modal.confirm({
    title: $gettext('Do you want to enable TLS?'),
    content: $gettext('To make sure the certification auto-renewal can work normally, '
      + 'we need to add a location which can proxy the request from authority to backend, '
      + 'and we need to save this file and reload the Nginx. Are you sure you want to continue?'),
    mask: false,
    centered: true,
    okText: $gettext('OK'),
    cancelText: $gettext('Cancel'),
    async onOk() {
      await template.get_block('letsencrypt.conf').then(async r => {
        const first = ngx_config.servers[0]
        if (!first.locations)
          first.locations = []
        else
          first.locations = first.locations.filter(l => l.path !== '/.well-known/acme-challenge')

        first.locations.push(...r.locations)
      })
      await save_site_config()

      change_tls(status)
    },
  })
}

const current_server_directives = computed(() => {
  return ngx_config.servers?.[current_server_index.value]?.directives
})

const directivesMap: ComputedRef<Record<string, NgxDirective[]>> = computed(() => {
  const map = {}

  current_server_directives.value?.forEach((v, k) => {
    v.idx = k
    if (map[v.directive])
      map[v.directive].push(v)
    else
      map[v.directive] = [v]
  })

  return map
})

function change_tls(status: boolean) {
  if (status) {
    // deep copy servers[0] to servers[1]
    const server = JSON.parse(JSON.stringify(ngx_config.servers[0]))

    ngx_config.servers.push(server)

    current_server_index.value = 1

    const servers = ngx_config.servers

    let i = 0
    while (i < servers[1].directives.length) {
      const v = servers[1].directives[i]
      if (v.directive === 'listen')
        servers[1].directives.splice(i, 1)
      else
        i++
    }

    servers[1].directives.splice(0, 0, {
      directive: 'listen',
      params: '443 ssl',
    }, {
      directive: 'listen',
      params: '[::]:443 ssl',
    }, {
      directive: 'http2',
      params: 'on',
    })

    const server_name = directivesMap.value.server_name[0]

    if (!directivesMap.value.ssl_certificate) {
      servers[1].directives.splice(server_name.idx + 1, 0, {
        directive: 'ssl_certificate',
        params: '',
      })
    }

    setTimeout(() => {
      if (!directivesMap.value.ssl_certificate_key) {
        servers[1].directives.splice(server_name.idx + 2, 0, {
          directive: 'ssl_certificate_key',
          params: '',
        })
      }
    }, 100)
  }
  else {
    // remove servers[1]
    current_server_index.value = 0
    if (ngx_config.servers.length === 2)
      ngx_config.servers.splice(1, 1)
  }
}

provide('current_server_directives', current_server_directives)

const support_ssl = computed(() => {
  const servers = ngx_config.servers
  for (const server_key in servers) {
    for (const k in servers[server_key].directives) {
      const v = servers[server_key].directives[k]
      if (v.directive === 'listen' && v.params.indexOf('ssl') > 0)
        return true
    }
  }

  return false
})

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
    emit('update:auto_cert', value)
  },
})

onMounted(() => {
  current_server_index.value = Number.parseInt((route.query?.server_idx ?? 0) as string)
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
  Modal.confirm({
    title: $gettext('Do you want to remove this server?'),
    mask: false,
    centered: true,
    okText: $gettext('OK'),
    cancelText: $gettext('Cancel'),
    onOk() {
      ngx_config?.servers?.splice(index, 1)
    },
  })
}

const ngx_directives = computed(() => {
  return ngx_config?.servers?.[current_server_index.value]?.directives
})

provide('ngx_directives', ngx_directives)
provide('directivesMap', directivesMap)
</script>

<template>
  <div>
    <AFormItem
      v-if="!support_ssl"
      :label="$gettext('Enable TLS')"
    >
      <ASwitch @change="confirm_change_tls" />
    </AFormItem>

    <h2>{{ $gettext('Custom') }}</h2>
    <CodeEditor
      v-model:content="ngx_config.custom"
      default-height="150px"
    />

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

<style lang="less" scoped>
:deep(.ant-tabs-tab-btn) {
  margin-left: 16px;
}
</style>
