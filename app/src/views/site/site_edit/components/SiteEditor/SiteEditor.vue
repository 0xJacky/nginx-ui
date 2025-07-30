<script setup lang="ts">
import { HistoryOutlined } from '@ant-design/icons-vue'
import { message } from 'ant-design-vue'
import CodeEditor from '@/components/CodeEditor/CodeEditor.vue'
import ConfigHistory from '@/components/ConfigHistory'
import FooterToolBar from '@/components/FooterToolbar'
import NgxConfigEditor from '@/components/NgxConfigEditor'
import UpstreamCards from '@/components/UpstreamCards/UpstreamCards.vue'
import { ConfigStatus } from '@/constants'
import Cert from '@/views/site/site_edit/components/Cert'
import EnableTLS from '@/views/site/site_edit/components/EnableTLS'
import { useSiteEditorStore } from './store'

const route = useRoute()

const name = computed(() => decodeURIComponent(route.params?.name?.toString() ?? ''))

const editorStore = useSiteEditorStore()
const {
  data,
  parseErrorStatus,
  parseErrorMessage,
  filepath,
  configText,
  loading,
  saving,
  certInfoMap,
  advanceMode,
  curSupportSSL,
} = storeToRefs(editorStore)

// Get upstream targets from backend API data
const upstreamTargets = computed(() => {
  return data.value.proxy_targets || []
})

const showHistory = ref(false)

onMounted(() => {
  editorStore.init(name.value)
})

async function save() {
  try {
    await editorStore.save()
    message.success($gettext('Saved successfully'))
  }
  catch {
    // do nothing
  }
}
</script>

<template>
  <ACard class="site-edit-container" :bordered="false">
    <template #title>
      <span style="margin-right: 10px">{{ $gettext('Edit %{n}', { n: name }) }}</span>
      <ATag
        v-if="data.status === ConfigStatus.Enabled"
        color="blue"
      >
        {{ $gettext('Enabled') }}
      </ATag>
      <ATag
        v-else-if="data.status === ConfigStatus.Disabled"
        color="red"
      >
        {{ $gettext('Disabled') }}
      </ATag>
      <ATag
        v-else-if="data.status === ConfigStatus.Maintenance"
        color="orange"
      >
        {{ $gettext('Maintenance') }}
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
              :loading="loading"
              @change="editorStore.handleModeChange"
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

    <div class="card-body">
      <Transition name="slide-fade">
        <div
          v-if="advanceMode"
          key="advance"
        >
          <div
            v-if="parseErrorStatus"
            class="parse-error-alert-wrapper"
          >
            <AAlert
              banner
              :message="$gettext('Nginx Configuration Parse Error')"
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
          <EnableTLS />

          <!-- Upstream Cards Display -->
          <UpstreamCards
            :targets="upstreamTargets"
            :env-group-id="data.env_group_id"
          />

          <NgxConfigEditor
            :cert-info="certInfoMap"
            :status="data.status"
          >
            <template #tab-content="{ tabIdx }">
              <Cert
                v-if="curSupportSSL"
                class="mb-4"
                :site-status="data.status"
                :config-name="name"
                :cert-info="certInfoMap?.[tabIdx]"
              />
            </template>
          </NgxConfigEditor>
        </div>
      </Transition>
    </div>

    <FooterToolBar>
      <ASpace>
        <AButton @click="$router.push('/sites/list')">
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

    <ConfigHistory
      v-model:visible="showHistory"
      v-model:current-content="configText"
      :filepath="filepath"
    />
  </ACard>
</template>

<style lang="less" scoped>
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
  padding: 24px 0;
}

.site-edit-container {
  height: 100%;
  :deep(.ant-card-body) {
    max-height: 100%;
    overflow-y: scroll;
    padding: 0;
  }
}

.domain-edit-container {
  max-width: 800px;
  margin: 0 auto;
}

.slide-fade-enter-active {
  transition: all .3s ease-in-out;
}

.slide-fade-leave-active {
  transition: all .3s cubic-bezier(1.0, 0.5, 0.8, 1.0);
}

.slide-fade-enter-from, .slide-fade-enter-to, .slide-fade-leave-to {
  transform: translateX(10px);
  opacity: 0;
}

:deep(.tab-content) {
  padding-bottom: 24px;
}
</style>
