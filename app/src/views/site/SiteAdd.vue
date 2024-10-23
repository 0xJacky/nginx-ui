<script setup lang="ts">
import { message } from 'ant-design-vue'
import DirectiveEditor from '@/views/site/ngx_conf/directive/DirectiveEditor.vue'
import LocationEditor from '@/views/site/ngx_conf/LocationEditor.vue'
import NgxConfigEditor from '@/views/site/ngx_conf/NgxConfigEditor.vue'
import domain from '@/api/domain'
import type { NgxConfig } from '@/api/ngx'
import ngx from '@/api/ngx'

const ngx_config: NgxConfig = reactive({
  name: '',
  servers: [{
    directives: [],
    locations: [],
  }],
})

const current_step = ref(0)

const enabled = ref(true)

const auto_cert = ref(false)

onMounted(() => {
  init()
})

function init() {
  domain.get_template().then(r => {
    Object.assign(ngx_config, r.tokenized)
  })
}

async function save() {
  return ngx.build_config(ngx_config).then(r => {
    domain.save(ngx_config.name, { name: ngx_config.name, content: r.content, overwrite: true }).then(() => {
      message.success($gettext('Saved successfully'))

      domain.enable(ngx_config.name).then(() => {
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

function goto_modify() {
  router.push(`/sites/${ngx_config.name}`)
}

function create_another() {
  router.go(0)
}

const has_server_name = computed(() => {
  const servers = ngx_config.servers

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
  current_step.value++
}

const ngx_directives = computed(() => {
  return ngx_config.servers[0].directives
})

provide('save_config', save)
provide('ngx_directives', ngx_directives)
provide('ngx_config', ngx_config)
</script>

<template>
  <ACard :title="$gettext('Add Site')">
    <div class="domain-add-container">
      <ASteps
        :current="current_step"
        size="small"
      >
        <AStep :title="$gettext('Base information')" />
        <AStep :title="$gettext('Configure SSL')" />
        <AStep :title="$gettext('Finished')" />
      </ASteps>
      <template v-if="current_step === 0">
        <AForm layout="vertical">
          <AFormItem :label="$gettext('Configuration Name')">
            <AInput v-model:value="ngx_config.name" />
          </AFormItem>
        </AForm>

        <DirectiveEditor />
        <br>
        <LocationEditor
          :locations="ngx_config.servers[0].locations"
          :current-server-index="0"
        />
        <br>
        <AAlert
          v-if="!has_server_name"
          :message="$gettext('Warning')"
          type="warning"
          show-icon
        >
          <template #description>
            <span>{{ $gettext('server_name parameter is required') }}</span>
          </template>
        </AAlert>
        <br>
      </template>

      <template v-else-if="current_step === 1">
        <NgxConfigEditor
          v-model:auto-cert="auto_cert"
          :enabled="enabled"
        />

        <br>
      </template>

      <ASpace v-if="current_step < 2">
        <AButton
          type="primary"
          :disabled="!ngx_config.name || !has_server_name"
          @click="next"
        >
          {{ $gettext('Next') }}
        </AButton>
      </ASpace>
      <AResult
        v-else-if="current_step === 2"
        status="success"
        :title="$gettext('Domain Config Created Successfully')"
      >
        <template #extra>
          <AButton
            type="primary"
            @click="goto_modify"
          >
            {{ $gettext('Modify Config') }}
          </AButton>
          <AButton @click="create_another">
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
