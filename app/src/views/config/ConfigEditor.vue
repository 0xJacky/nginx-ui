<script setup lang="ts">
import type { Ref } from 'vue'
import type { Config } from '@/api/config'
import type { ChatComplicationMessage } from '@/api/openai'
import { HistoryOutlined, InfoCircleOutlined } from '@ant-design/icons-vue'
import { message } from 'ant-design-vue'
import { trim, trimEnd } from 'lodash'
import config from '@/api/config'
import ngx from '@/api/ngx'
import ChatGPT from '@/components/ChatGPT/ChatGPT.vue'
import CodeEditor from '@/components/CodeEditor/CodeEditor.vue'
import { ConfigHistory } from '@/components/ConfigHistory'
import FooterToolBar from '@/components/FooterToolbar/FooterToolBar.vue'
import NodeSelector from '@/components/NodeSelector/NodeSelector.vue'
import { useBreadcrumbs } from '@/composables/useBreadcrumbs'
import { formatDateTime } from '@/lib/helper'
import { useSettingsStore } from '@/pinia'
import ConfigName from '@/views/config/components/ConfigName.vue'
import InspectConfig from '@/views/config/InspectConfig.vue'

const settings = useSettingsStore()
const route = useRoute()
const router = useRouter()

// eslint-disable-next-line vue/require-typed-ref
const refForm = ref()
const origName = ref('')
const addMode = computed(() => !route.params.name)

const showHistory = ref(false)
const basePath = computed(() => {
  if (route.query.basePath)
    return trim(route?.query?.basePath?.toString(), '/')
  else if (typeof route.params.name === 'object')
    return (route.params.name as string[]).slice(0, -1).join('/')
  else
    return ''
})

const data = ref({
  name: '',
  content: '',
  filepath: '',
  sync_node_ids: [] as number[],
  sync_overwrite: false,
} as Config)

const historyChatgptRecord = ref([]) as Ref<ChatComplicationMessage[]>
const activeKey = ref(['basic', 'deploy', 'chatgpt'])
const modifiedAt = ref('')
const nginxConfigBase = ref('')

const newPath = computed(() => {
  // Decode and display after combining paths
  const path = [nginxConfigBase.value, basePath.value, data.value.name]
    .filter(v => v)
    .join('/')
  return path
})

const relativePath = computed(() => (basePath.value ? `${basePath.value}/${route.params.name}` : route.params.name) as string)
const breadcrumbs = useBreadcrumbs()

// Use Vue 3.4+ useTemplateRef for InspectConfig component
const inspectConfigRef = useTemplateRef<InstanceType<typeof InspectConfig>>('inspectConfig')

async function init() {
  const { name } = route.params

  data.value.name = name?.[name?.length - 1] ?? ''
  origName.value = data.value.name
  if (!addMode.value) {
    config.getItem(relativePath.value).then(r => {
      data.value = r
      historyChatgptRecord.value = r.chatgpt_messages
      modifiedAt.value = r.modified_at

      const filteredPath = trimEnd(data.value.filepath
        .replaceAll(`${nginxConfigBase.value}/`, ''), data.value.name)
        .split('/')
        .filter(v => v)

      // Build accumulated path to maintain original encoding state
      let accumulatedPath = ''
      const path = filteredPath.map((segment, index) => {
        // Decode for display
        const decodedSegment = decodeURIComponent(segment)

        // Accumulated path keeps original encoding state
        if (index === 0) {
          accumulatedPath = segment
        }
        else {
          accumulatedPath = `${accumulatedPath}/${segment}`
        }

        return {
          name: 'Manage Configs',
          translatedName: () => decodedSegment,
          path: '/config',
          query: {
            dir: accumulatedPath,
          },
          hasChildren: false,
        }
      })

      breadcrumbs.value = [{
        name: 'Dashboard',
        translatedName: () => $gettext('Dashboard'),
        path: '/dashboard',
        hasChildren: false,
      }, {
        name: 'Manage Configs',
        translatedName: () => $gettext('Manage Configs'),
        path: '/config',
        hasChildren: false,
      }, ...path, {
        name: 'Edit Config',
        translatedName: () => origName.value,
        hasChildren: false,
      }]
    })
  }
  else {
    data.value.content = ''
    historyChatgptRecord.value = []
    data.value.filepath = ''

    const pathSegments = basePath.value
      .split('/')
      .filter(v => v)

    // Build accumulated path
    let accumulatedPath = ''
    const path = pathSegments.map((segment, index) => {
      // Decode for display
      const decodedSegment = decodeURIComponent(segment)

      // Accumulated path keeps original encoding state
      if (index === 0) {
        accumulatedPath = segment
      }
      else {
        accumulatedPath = `${accumulatedPath}/${segment}`
      }

      return {
        name: 'Manage Configs',
        translatedName: () => decodedSegment,
        path: '/config',
        query: {
          dir: accumulatedPath,
        },
        hasChildren: false,
      }
    })

    breadcrumbs.value = [{
      name: 'Dashboard',
      translatedName: () => $gettext('Dashboard'),
      path: '/dashboard',
      hasChildren: false,
    }, {
      name: 'Manage Configs',
      translatedName: () => $gettext('Manage Configs'),
      path: '/config',
      hasChildren: false,
    }, ...path, {
      name: 'Add Config',
      translatedName: () => $gettext('Add Configuration'),
      hasChildren: false,
    }]
  }
}

onMounted(async () => {
  await config.get_base_path().then(r => {
    nginxConfigBase.value = r.base_path
  })
  await init()
})

function save() {
  refForm.value?.validate().then(() => {
    config.save(addMode.value ? undefined : relativePath.value, {
      name: addMode.value ? data.value.name : undefined,
      base_dir: addMode.value ? basePath.value : undefined,
      content: data.value.content,
      sync_node_ids: data.value.sync_node_ids,
      sync_overwrite: data.value.sync_overwrite,
    }).then(r => {
      data.value.content = r.content
      message.success($gettext('Saved successfully'))

      if (addMode.value) {
        router.push({
          path: `/config/${data.value.name}/edit`,
          query: {
            basePath: basePath.value,
          },
        })
      }
      else {
        data.value = r
        // Run test after saving to verify configuration
        inspectConfigRef.value?.test()
      }
    })
  })
}

function formatCode() {
  ngx.format_code(data.value.content).then(r => {
    data.value.content = r.content
    message.success($gettext('Format successfully'))
  })
}

function goBack() {
  // Keep original path with encoding state
  const encodedPath = basePath.value || ''

  router.push({
    path: '/config',
    query: {
      dir: encodedPath || undefined,
    },
  })
}

function openHistory() {
  showHistory.value = true
}
</script>

<template>
  <ARow :gutter="16">
    <ACol
      :xs="24"
      :sm="24"
      :md="18"
    >
      <ACard :title="addMode ? $gettext('Add Configuration') : $gettext('Edit Configuration')">
        <template #extra>
          <AButton
            v-if="!addMode && data.filepath"
            type="link"
            @click="openHistory"
          >
            <template #icon>
              <HistoryOutlined />
            </template>
            {{ $gettext('History') }}
          </AButton>
        </template>

        <InspectConfig
          v-show="!addMode"
          ref="inspectConfig"
        />
        <CodeEditor v-model:content="data.content" />
        <FooterToolBar>
          <ASpace>
            <AButton @click="goBack">
              {{ $gettext('Back') }}
            </AButton>
            <AButton @click="formatCode">
              {{ $gettext('Format Code') }}
            </AButton>
            <AButton
              type="primary"
              @click="save"
            >
              {{ $gettext('Save') }}
            </AButton>
          </ASpace>
        </FooterToolBar>
      </ACard>
    </ACol>

    <ACol
      :xs="24"
      :sm="24"
      :md="6"
    >
      <ACard class="col-right">
        <ACollapse
          v-model:active-key="activeKey"
          ghost
        >
          <ACollapsePanel
            key="basic"
            :header="$gettext('Basic')"
          >
            <AForm
              ref="refForm"
              layout="vertical"
              :model="data"
              :rules="{
                name: [
                  { required: true, message: $gettext('Please input a filename') },
                  { pattern: /^[^\\/]+$/, message: $gettext('Invalid filename') },
                ],
              }"
            >
              <AFormItem
                name="name"
                :label="$gettext('Name')"
              >
                <AInput v-if="addMode" v-model:value="data.name" />
                <ConfigName v-else :name="data.name" :dir="data.dir" />
              </AFormItem>
              <AFormItem
                v-if="!addMode"
                :label="$gettext('Path')"
              >
                {{ decodeURIComponent(data.filepath) }}
              </AFormItem>
              <AFormItem
                v-show="data.name !== origName"
                :label="addMode ? $gettext('New Path') : $gettext('Changed Path')"
                required
              >
                {{ decodeURIComponent(newPath) }}
              </AFormItem>
              <AFormItem
                v-if="!addMode"
                :label="$gettext('Updated at')"
              >
                {{ formatDateTime(modifiedAt) }}
              </AFormItem>
            </AForm>
          </ACollapsePanel>
          <ACollapsePanel
            v-if="!settings.is_remote"
            key="deploy"
            :header="$gettext('Deploy')"
          >
            <NodeSelector
              v-model:target="data.sync_node_ids"
              hidden-local
            />
            <div class="node-deploy-control">
              <div class="overwrite">
                <ACheckbox v-model:checked="data.sync_overwrite">
                  {{ $gettext('Overwrite') }}
                </ACheckbox>
                <ATooltip placement="bottom">
                  <template #title>
                    {{ $gettext('Overwrite exist file') }}
                  </template>
                  <InfoCircleOutlined />
                </ATooltip>
              </div>
            </div>
          </ACollapsePanel>
          <ACollapsePanel
            key="chatgpt"
            header="ChatGPT"
          >
            <ChatGPT
              v-model:history-messages="historyChatgptRecord"
              :content="data.content"
              :path="data.filepath"
            />
          </ACollapsePanel>
        </ACollapse>
      </ACard>
    </ACol>

    <ConfigHistory
      v-model:visible="showHistory"
      v-model:current-content="data.content"
      :filepath="data.filepath"
    />
  </ARow>
</template>

<style lang="less" scoped>
.col-right {
  position: sticky;
  top: 78px;

  :deep(.ant-card-body) {
    max-height: 100vh;
    overflow-y: scroll;
  }
}

:deep(.ant-collapse-ghost > .ant-collapse-item > .ant-collapse-content > .ant-collapse-content-box) {
  padding: 0;
}

:deep(.ant-collapse > .ant-collapse-item > .ant-collapse-header) {
  padding: 0 0 10px 0;
}

.overwrite {
  margin-right: 15px;

  span {
    color: #9b9b9b;
  }
}

.node-deploy-control {
  display: flex;
  justify-content: flex-end;
  margin-top: 10px;
  align-items: center;
}
</style>
