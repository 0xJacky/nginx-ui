<script lang="ts" setup>
import { HistoryOutlined, LoadingOutlined } from '@ant-design/icons-vue'
import CodeEditor from '@/components/CodeEditor'
import ConfigHistory from '@/components/ConfigHistory'
import FooterToolBar from '@/components/FooterToolbar'
import NgxConfigEditor from '@/components/NgxConfigEditor'
import { ConfigStatus } from '@/constants'
import { useStreamEditorStore } from '../store'

const router = useRouter()

const store = useStreamEditorStore()
const { name, status, configText, filepath, saving, parseErrorStatus, parseErrorMessage, advanceMode, loading } = storeToRefs(store)
const showHistory = ref(false)
</script>

<template>
  <ASpin :spinning="loading" :indicator="LoadingOutlined">
    <ACard class="mb-4" :bordered="false">
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

      <div class="card-body">
        <Transition name="slide-fade">
          <div
            v-if="advanceMode"
            key="advance"
          >
            <div v-if="parseErrorStatus">
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
            <NgxConfigEditor
              :enabled="status === ConfigStatus.Enabled"
              context="stream"
            />
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
            @click="store.save"
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
