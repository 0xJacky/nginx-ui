<script setup lang="ts">
import { message } from 'ant-design-vue'
import ngx from '@/api/ngx'
import site from '@/api/site'
import NgxConfigEditor, { DirectiveEditor, LocationEditor, useNgxConfigStore } from '@/components/NgxConfigEditor'
import { ConfigStatus } from '@/constants'
import Cert from '../site_edit/components/Cert'
import EnableTLS from '../site_edit/components/EnableTLS'
import { useSiteEditorStore } from '../site_edit/components/SiteEditor/store'

const currentStep = ref(0)

onMounted(() => {
  init()
})

const ngxConfigStore = useNgxConfigStore()
const editorStore = useSiteEditorStore()
const { ngxConfig, curServerDirectives, curServerLocations } = storeToRefs(ngxConfigStore)
const { curSupportSSL } = storeToRefs(editorStore)

function init() {
  site.get_default_template().then(r => {
    ngxConfig.value = r.tokenized
  })
}

async function save() {
  return ngx.build_config(ngxConfig.value).then(r => {
    site.updateItem(ngxConfig.value.name, { name: ngxConfig.value.name, content: r.content, overwrite: true }).then(() => {
      message.success($gettext('Saved successfully'))

      site.enable(ngxConfig.value.name).then(() => {
        message.success($gettext('Enabled successfully'))
        window.scroll({ top: 0, left: 0, behavior: 'smooth' })
      }).catch(e => {
        message.error(e.message ?? $gettext('Enable failed'), 5)
      })
    }).catch(e => {
      message.error($gettext('Save error %{msg}', { msg: $gettext(e.message) ?? '' }), 5)
    })
  })
}

const router = useRouter()

function gotoModify() {
  router.push(`/sites/${ngxConfig.value.name}`)
}

function createAnother() {
  router.go(0)
}

const hasServerName = computed(() => {
  const servers = ngxConfig.value.servers

  for (const server of Object.values(servers)) {
    for (const directive of Object.values(server.directives!)) {
      if (directive.directive === 'server_name' && directive.params.trim() !== '')
        return true
    }
  }

  return false
})

async function next() {
  await save()
  currentStep.value++
}
</script>

<template>
  <ACard :title="$gettext('Add Site')">
    <div class="domain-add-container">
      <ASteps
        :current="currentStep"
        size="small"
      >
        <AStep :title="$gettext('Base information')" />
        <AStep :title="$gettext('Configure SSL')" />
        <AStep :title="$gettext('Finished')" />
      </ASteps>
      <div v-if="currentStep === 0" class="mb-6">
        <AForm layout="vertical">
          <AFormItem :label="$gettext('Configuration Name')">
            <AInput v-model:value="ngxConfig.name" />
          </AFormItem>
        </AForm>

        <AAlert
          v-if="!hasServerName"
          type="warning"
          class="mb-4"
          show-icon
          :message="$gettext('The parameter of server_name is required')"
        />

        <DirectiveEditor
          v-model:directives="curServerDirectives"
          class="mb-4"
        />
        <LocationEditor
          v-model:locations="curServerLocations"
          :current-server-index="0"
        />
      </div>

      <template v-else-if="currentStep === 1">
        <EnableTLS />

        <NgxConfigEditor>
          <template v-if="curSupportSSL" #tab-content>
            <Cert
              class="mb-4"
              :site-status="ConfigStatus.Enabled"
              :config-name="ngxConfig.name"
            />
          </template>
        </NgxConfigEditor>

        <br>
      </template>

      <ASpace v-if="currentStep < 2">
        <AButton
          type="primary"
          :disabled="!ngxConfig.name || !hasServerName"
          @click="next"
        >
          {{ $gettext('Next') }}
        </AButton>
      </ASpace>
      <AResult
        v-else-if="currentStep === 2"
        status="success"
        :title="$gettext('Site Config Created Successfully')"
      >
        <template #extra>
          <AButton
            type="primary"
            @click="gotoModify"
          >
            {{ $gettext('Modify Config') }}
          </AButton>
          <AButton @click="createAnother">
            {{ $gettext('Create Another') }}
          </AButton>
        </template>
      </AResult>
    </div>
  </ACard>
</template>

<style lang="less" scoped>
.ant-steps {
  padding: 10px 0 20px 0;
}

.domain-add-container {
  max-width: 800px;
  margin: 0 auto
}
</style>
