<script lang="ts" setup>
import CodeEditor from '@/components/CodeEditor'
import ConfigHistory from '@/components/ConfigHistory'
import FooterToolBar from '@/components/FooterToolbar'
import NgxConfigEditor from '@/components/NgxConfigEditor'
import { HistoryOutlined } from '@ant-design/icons-vue'
import { useStreamEditorStore } from '../store'

const router = useRouter()

const store = useStreamEditorStore()
const { name, enabled, configText, filepath, saving, parseErrorStatus, parseErrorMessage, advanceMode } = storeToRefs(store)
const showHistory = ref(false)
</script>

<template>
  <ACard class="mb-4" :bordered="false">
    <template #title>
      <span style="margin-right: 10px">{{ $gettext('Edit %{n}', { n: name }) }}</span>
      <ATag
        v-if="enabled"
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

    <Transition name="slide-fade">
      <div
        v-if="advanceMode"
        key="advance"
      >
        <div
          v-if="parseErrorStatus"
          class="mb-4"
        >
          <AAlert
            :message="$gettext('Nginx Configuration Parse Error')"
            :description="parseErrorMessage"
            type="error"
            show-icon
          />
        </div>
        <div>
          <CodeEditor v-model:content="configText" />
        </div>
      </div>

      <div
        v-else
        key="basic"
        class="domain-edit-container"
      >
        <NgxConfigEditor
          :enabled="enabled"
          context="stream"
        />
      </div>
    </Transition>

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
</style>
