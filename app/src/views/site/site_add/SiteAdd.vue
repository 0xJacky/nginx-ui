<script setup lang="ts">
import type { NgxConfig } from '@/api/ngx'
import ngx from '@/api/ngx'
import site from '@/api/site'
import DirectiveEditor from '@/views/site/ngx_conf/directive/DirectiveEditor.vue'
import LocationEditor from '@/views/site/ngx_conf/LocationEditor.vue'
import NgxConfigEditor from '@/views/site/ngx_conf/NgxConfigEditor.vue'
import { message } from 'ant-design-vue'

const ngxConfig: NgxConfig = reactive({
  name: '',
  servers: [{
    directives: [],
    locations: [],
  }],
})

const currentStep = ref(0)

const enabled = ref(true)

const autoCert = ref(false)

onMounted(() => {
  init()
})

function init() {
  site.get_default_template().then(r => {
    Object.assign(ngxConfig, r.tokenized)
  })
}

async function save() {
  return ngx.build_config(ngxConfig).then(r => {
    site.save(ngxConfig.name, { name: ngxConfig.name, content: r.content, overwrite: true }).then(() => {
      message.success($gettext('Saved successfully'))

      site.enable(ngxConfig.name).then(() => {
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
  router.push(`/sites/${ngxConfig.name}`)
}

function createAnother() {
  router.go(0)
}

const hasServerName = computed(() => {
  const servers = ngxConfig.servers

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

const ngxDirectives = computed(() => {
  return ngxConfig.servers[0].directives
})

provide('save_config', save)
provide('ngx_directives', ngxDirectives)
provide('ngx_config', ngxConfig)
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
      <template v-if="currentStep === 0">
        <AForm layout="vertical">
          <AFormItem :label="$gettext('Configuration Name')">
            <AInput v-model:value="ngxConfig.name" />
          </AFormItem>
        </AForm>

        <DirectiveEditor />
        <br>
        <LocationEditor
          :locations="ngxConfig.servers[0].locations"
          :current-server-index="0"
        />
        <br>
        <AAlert
          v-if="!hasServerName"
          :message="$gettext('Warning')"
          type="warning"
          show-icon
        >
          <template #description>
            <span>{{ $gettext('The parameter of server_name is required') }}</span>
          </template>
        </AAlert>
        <br>
      </template>

      <template v-else-if="currentStep === 1">
        <NgxConfigEditor
          v-model:auto-cert="autoCert"
          :enabled="enabled"
        />

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
