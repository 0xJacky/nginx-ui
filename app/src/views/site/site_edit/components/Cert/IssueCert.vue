<script setup lang="ts">
import { Modal } from 'ant-design-vue'
import template from '@/api/template'
import { useGlobalStore } from '@/pinia'
import { useSiteEditorStore } from '@/views/site/site_edit/components/SiteEditor/store'
import ObtainCert from './ObtainCert.vue'

const editorStore = useSiteEditorStore()
const { ngxConfig, issuingCert, curServer, curDirectivesMap, autoCert } = storeToRefs(editorStore)

const [modal, ContextHolder] = Modal.useModal()

const obtainCert = useTemplateRef('obtainCert')

const noServerName = computed(() => {
  if (!curDirectivesMap.value.server_name)
    return true

  return curDirectivesMap.value.server_name.length === 0
})

watch(noServerName, () => {
  autoCert.value = false
})

const update = ref(0)

async function onchange() {
  update.value++
  await nextTick()

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
        if (!curServer.value.locations)
          curServer.value.locations = []
        else
          curServer.value.locations = curServer.value.locations.filter(l => !l.path.includes('/.well-known/acme-challenge'))

        await nextTick()

        curServer.value.locations.push(...r.locations!)
      })
      await editorStore.save()

      await nextTick()

      obtainCert.value!.toggle(autoCert.value)
    },
  })
}

const globalStore = useGlobalStore()
const { processingStatus } = storeToRefs(globalStore)
</script>

<template>
  <div>
    <ContextHolder />
    <ObtainCert
      ref="obtainCert"
      :key="update"
      v-model:auto-cert="autoCert"
      :no-server-name="noServerName"
      :config-name="ngxConfig.name"
    />
    <div class="issue-cert">
      <AFormItem :label="$gettext('Encrypt website with Let\'s Encrypt')">
        <ASwitch
          :loading="issuingCert"
          :checked="autoCert"
          :disabled="noServerName || processingStatus.auto_cert_processing"
          @change="onchange"
        />
        <span v-if="processingStatus.auto_cert_processing" class="ml-4">
          {{ $gettext('AutoCert is running, please wait...') }}
        </span>
      </AFormItem>
    </div>
  </div>
</template>

<style lang="less" scoped>
.ant-tag {
  margin: 0;
}

.issue-cert {
  margin: 15px 0;
}

.switch-wrapper {
  position: relative;

  .text {
    position: absolute;
    top: 50%;
    transform: translateY(-50%);
    margin-left: 10px;
  }
}
</style>
