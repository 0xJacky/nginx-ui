<script setup lang="ts">
import type { CheckedType } from '@/types'
import { Modal } from 'antdv-next'
import template from '@/api/template'
import { useSiteEditorStore } from '@/views/site/site_edit/components/SiteEditor/store'
import { ensureHTTPChallengeLocation } from '../../composables/useHTTPChallenge'
import { buildTLSServerFromHTTPServer } from '../../composables/useHTTPSRedirect'

// The classic "Enable TLS" switch for sites the HTTPS card does not cover
// (e.g. a disabled site). Using an existing certificate is one of the HTTPS
// card's methods, so there is no separate "own certificate" action here.

const [modal, ContextHolder] = Modal.useModal()

const editorStore = useSiteEditorStore()
const { ngxConfig, curServerIdx, hasServers } = storeToRefs(editorStore)

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
        ensureHTTPChallengeLocation(first, r.locations!)

        await nextTick()
      })
      await editorStore.save()

      changeTLS(status)
    },
  })
}

function changeTLS(status: CheckedType) {
  if (status) {
    // Copy servers[0] into a 443 server without its HTTPS redirect, which
    // would otherwise make HTTPS redirect to itself.
    ngxConfig.value.servers.push(buildTLSServerFromHTTPServer(ngxConfig.value.servers[0]))

    curServerIdx.value = ngxConfig.value.servers.length - 1
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
