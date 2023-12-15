<script setup lang="ts">
import { useGettext } from 'vue3-gettext'
import Modal from 'ant-design-vue/lib/modal'
import type { ComputedRef } from 'vue'
import CodeEditor from '@/components/CodeEditor/CodeEditor.vue'
import template from '@/api/template'
import type { NgxConfig, NgxDirective } from '@/api/ngx'
import type { CertificateInfo } from '@/api/cert'
import NgxServer from '@/views/domain/ngx_conf/NgxServer.vue'
import NgxUpstream from '@/views/domain/ngx_conf/NgxUpstream.vue'

const props = withDefaults(defineProps<{
  autoCert?: boolean
  enabled: boolean
  certInfo?: Record<number, CertificateInfo>
  context?: 'http' | 'stream'
}>(), {
  autoCert: false,
  enabled: false,
  context: 'http',
})

const emit = defineEmits(['callback', 'update:autoCert'])

const { $gettext } = useGettext()

const save_config = inject('save_config') as () => Promise<void>

const [modal, ContextHolder] = Modal.useModal()

const current_server_index = ref(0)

provide('current_server_index', current_server_index)

const route = useRoute()

onMounted(() => {
  current_server_index.value = Number.parseInt((route.query?.server_idx ?? 0) as string)
})

const ngx_config = inject('ngx_config') as NgxConfig

function confirm_change_tls(status: boolean) {
  modal.confirm({
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
      await save_config()

      change_tls(status)
    },
  })
}

const current_server_directives = computed(() => {
  return ngx_config.servers?.[current_server_index.value]?.directives
})

provide('current_server_directives', current_server_directives)

const directivesMap: ComputedRef<Record<string, NgxDirective[]>> = computed(() => {
  const map: Record<string, NgxDirective[]> = {}

  current_server_directives.value?.forEach((v, k) => {
    v.idx = k
    if (map[v.directive])
      map[v.directive].push(v)
    else
      map[v.directive] = [v]
  })

  return map
})

// eslint-disable-next-line sonarjs/cognitive-complexity
function change_tls(status: boolean) {
  if (status) {
    // deep copy servers[0] to servers[1]
    const server = JSON.parse(JSON.stringify(ngx_config.servers[0]))

    ngx_config.servers.push(server)

    current_server_index.value = 1

    const servers = ngx_config.servers

    let i = 0
    while (i < (servers?.[1].directives?.length ?? 0)) {
      const v = servers?.[1]?.directives?.[i]
      if (v?.directive === 'listen')
        servers[1]?.directives?.splice(i, 1)
      else
        i++
    }

    servers?.[1]?.directives?.splice(0, 0, {
      directive: 'listen',
      params: '443 ssl',
    }, {
      directive: 'listen',
      params: '[::]:443 ssl',
    })

    const server_name_idx = directivesMap.value?.server_name?.[0].idx ?? 0

    if (!directivesMap.value.ssl_certificate) {
      servers?.[1]?.directives?.splice(server_name_idx + 1, 0, {
        directive: 'ssl_certificate',
        params: '',
      })
    }

    setTimeout(() => {
      if (!directivesMap.value.ssl_certificate_key) {
        servers?.[1]?.directives?.splice(server_name_idx + 2, 0, {
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

const support_ssl = computed(() => {
  const servers = ngx_config.servers
  for (const server_key in servers) {
    for (const k in servers[server_key].directives) {
      const v = servers?.[server_key]?.directives?.[Number.parseInt(k)]
      if (v?.directive === 'listen' && v?.params?.indexOf('ssl') > 0)
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

provide('directivesMap', directivesMap)

const activeKey = ref(['3'])
</script>

<template>
  <div>
    <ContextHolder />
    <AFormItem
      v-if="!support_ssl && context === 'http'"
      :label="$gettext('Enable TLS')"
    >
      <ASwitch @change="confirm_change_tls" />
    </AFormItem>

    <ACollapse
      v-model:activeKey="activeKey"
      ghost
    >
      <ACollapsePanel
        key="1"
        :header="$gettext('Custom')"
      >
        <div class="mb-4">
          <CodeEditor
            v-model:content="ngx_config.custom"
            default-height="150px"
          />
        </div>
      </ACollapsePanel>
      <ACollapsePanel
        key="2"
        header="Upstream"
      >
        <NgxUpstream />
      </ACollapsePanel>
      <ACollapsePanel
        key="3"
        header="Server"
      >
        <NgxServer
          v-model:auto-cert="autoCertRef"
          :enabled="enabled"
          :cert-info="certInfo"
          :context="context"
        />
      </ACollapsePanel>
    </ACollapse>
  </div>
</template>

<style lang="less" scoped>
:deep(.ant-tabs-tab-btn) {
  margin-left: 16px;
}
</style>
