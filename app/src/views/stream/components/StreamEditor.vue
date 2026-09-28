<script lang="ts" setup>
import { HistoryOutlined, LoadingOutlined } from '@antdv-next/icons'
import { AccessControlCard, useAccessControlStore } from '@/components/AccessControl'
import CodeEditor from '@/components/CodeEditor'
import ConfigHistory from '@/components/ConfigHistory'
import FooterToolBar from '@/components/FooterToolbar'
import InspectConfig from '@/components/InspectConfig'
import NgxConfigEditor from '@/components/NgxConfigEditor'
import UpstreamCards from '@/components/UpstreamCards/UpstreamCards.vue'
import { ConfigStatus } from '@/constants'
import { useStreamEditorStore } from '../store'

const router = useRouter()
const { message } = App.useApp()

const store = useStreamEditorStore()
const { name, status, configText, filepath, saving, parseErrorStatus, parseErrorMessage, advanceMode, loading, data } = storeToRefs(store)
const showHistory = ref(false)

// Use Vue 3.4+ useTemplateRef for InspectConfig component
const inspectConfigRef = useTemplateRef<InstanceType<typeof InspectConfig>>('inspectConfig')

// Get upstream targets from backend API data
const upstreamTargets = computed(() => {
  return data.value.proxy_targets || []
})

// The access control card and the right panel read the access state of the
// open stream from this store.
const accessControl = useAccessControlStore()
watch(advanceMode, value => {
  accessControl.advanced = value
}, { immediate: true })
onMounted(() => accessControl.activate())
onBeforeUnmount(() => accessControl.deactivate())

async function save() {
  try {
    await store.save()
    message.success($gettext('Saved successfully'))
    // Run test after saving to verify configuration
    inspectConfigRef.value?.test()
  }
  catch {
    // do nothing
  }
}
</script>

<template>
  <ASpin :spinning="loading" :indicator="h(LoadingOutlined)">
    <ACard class="mb-4" variant="borderless">
      <template #title>
        <span style="margin-right: 10px">{{ $gettext('Edit %{n}', { n: name }) }}</span>
        <ATag
          v-if="status === ConfigStatus.Enabled"
          color="blue"
        >
          {{ $gettext('Enabled') }}
        </ATag>
        <ATag
          v-else
          color="orange"
        >
          {{ $gettext('Disabled') }}
        </ATag>
      </template>
      <template #extra>
        <ASpace>
          <AButton
            v-if="filepath"
            type="link"
            @click="showHistory = true"
          >
            <template #icon>
              <HistoryOutlined />
            </template>
            {{ $gettext('History') }}
          </AButton>
          <div class="mode-switch">
            <div class="switch">
              <ASwitch
                size="small"
                :disabled="parseErrorStatus"
                :checked="advanceMode"
                @change="store.handleModeChange"
              />
            </div>
            <template v-if="advanceMode">
              <div>{{ $gettext('Advance Mode') }}</div>
            </template>
            <template v-else>
              <div>{{ $gettext('Basic Mode') }}</div>
            </template>
          </div>
        </ASpace>
      </template>

      <InspectConfig
        ref="inspectConfig"
        class="mb-0!"
        banner
        :namespace-id="data.namespace_id"
      />

      <div class="card-body">
        <Transition name="slide-fade">
          <div
            v-if="advanceMode"
            key="advance"
          >
            <div v-if="parseErrorStatus">
              <AAlert
                banner
                :title="$gettext('Nginx Configuration Parse Error')"
                :description="parseErrorMessage"
                type="error"
                show-icon
              />
            </div>
            <div>
              <CodeEditor
                v-model:content="configText"
                no-border-radius
              />
            </div>
          </div>

          <div
            v-else
            key="basic"
            class="domain-edit-container"
          >
            <!-- Upstream Cards Display -->
            <UpstreamCards
              :targets="upstreamTargets"
              :namespace-id="data.namespace_id"
            />

            <NgxConfigEditor
              :enabled="status === ConfigStatus.Enabled"
              context="stream"
            >
              <template #tab-content="{ tabIdx }">
                <AccessControlCard :server-idx="tabIdx" context="stream" />
              </template>
            </NgxConfigEditor>
          </div>
        </Transition>
      </div>

      <ConfigHistory
        v-model:visible="showHistory"
        v-model:current-content="configText"
        :filepath="filepath"
      />

      <FooterToolBar>
        <ASpace>
          <AButton @click="router.push('/streams')">
            {{ $gettext('Back') }}
          </AButton>
          <AButton
            type="primary"
            :loading="saving"
            @click="save"
          >
            {{ $gettext('Save') }}
          </AButton>
        </ASpace>
      </FooterToolBar>
    </ACard>
  </ASpin>
</template>

<style scoped lang="less">
.mode-switch {
  display: flex;

  .switch {
    display: flex;
    align-items: center;
    margin-right: 5px;
  }
}

.domain-edit-container {
  max-width: 800px;
  margin: 0 auto;
}

:deep(.tab-content) {
  padding-bottom: 24px;
}
</style>
