<script setup lang="ts">
import type { CheckedType } from '@/types'
import { Modal } from 'ant-design-vue'
import template from '@/api/template'
import { useSiteEditorStore } from '@/views/site/site_edit/components/SiteEditor/store'

const [modal, ContextHolder] = Modal.useModal()

const editorStore = useSiteEditorStore()
const { ngxConfig, curServerIdx, curDirectivesMap, hasServers } = storeToRefs(editorStore)

function confirmChangeTLS(status: CheckedType) {
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
        const first = ngxConfig.value.servers[0]
        if (!first.locations)
          first.locations = []
        else
          first.locations = first.locations.filter(l => !l.path.includes('/.well-known/acme-challenge'))

        await nextTick()

        first.locations?.push(...r.locations!)
      })
      await editorStore.save()

      changeTLS(status)
    },
  })
}

function changeTLS(status: CheckedType) {
  if (status) {
    // deep copy servers[0] to servers[1]
    const server = JSON.parse(JSON.stringify(ngxConfig.value.servers[0]))

    ngxConfig.value.servers.push(server)

    curServerIdx.value = 1

    const servers = ngxConfig.value.servers

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

    const serverNameIdx = curDirectivesMap.value?.server_name?.[0].idx ?? 0

    if (!curDirectivesMap.value.ssl_certificate) {
      servers?.[1]?.directives?.splice(serverNameIdx + 1, 0, {
        directive: 'ssl_certificate',
        params: '',
      })
    }

    setTimeout(() => {
      if (!curDirectivesMap.value.ssl_certificate_key) {
        servers?.[1]?.directives?.splice(serverNameIdx + 2, 0, {
          directive: 'ssl_certificate_key',
          params: '',
        })
      }
    }, 100)
  }
  else {
    // remove servers[1]
    curServerIdx.value = 0
    if (ngxConfig.value.servers.length === 2)
      ngxConfig.value.servers.splice(1, 1)
  }
}

const supportSSL = computed(() => {
  const servers = ngxConfig.value.servers
  for (const server_key in servers) {
    for (const k in servers[server_key].directives) {
      const v = servers?.[server_key]?.directives?.[Number.parseInt(k)]
      if (v?.directive === 'listen' && v?.params?.indexOf('ssl') > 0)
        return true
    }
  }

  return false
})
</script>

<template>
  <div v-if="hasServers" class="px-6">
    <ContextHolder />

    <AFormItem
      v-if="!supportSSL"
      :label="$gettext('Enable TLS')"
    >
      <ASwitch class="<sm:ml-2" @change="confirmChangeTLS" />
    </AFormItem>
  </div>
</template>
