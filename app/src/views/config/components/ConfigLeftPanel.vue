<script setup lang="ts">
import type { Config } from '@/api/config'
import { HistoryOutlined } from '@ant-design/icons-vue'
import { trim, trimEnd } from 'lodash'
import config from '@/api/config'
import ngx from '@/api/ngx'
import CodeEditor from '@/components/CodeEditor'
import { ConfigHistory } from '@/components/ConfigHistory'
import FooterToolbar from '@/components/FooterToolbar'
import { useBreadcrumbs } from '@/composables/useBreadcrumbs'
import InspectConfig from '@/views/config/InspectConfig.vue'

const route = useRoute()
const router = useRouter()
const { message } = useGlobalApp()

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

const modifiedAt = ref('')
const nginxConfigBase = ref('')
const loading = ref(true)

const newPath = computed(() => {
  // Decode and display after combining paths
  const path = [nginxConfigBase.value, basePath.value, data.value.name]
    .filter(v => v)
    .join('/')
  return path
})

const relativePath = computed(() => (basePath.value ? `${basePath.value}/${route.params.name.toString()}` : route.params.name.toString()))
const breadcrumbs = useBreadcrumbs()

// Use Vue 3.4+ useTemplateRef for InspectConfig component
const inspectConfigRef = useTemplateRef<InstanceType<typeof InspectConfig>>('inspectConfig')

// Expose data for right panel
defineExpose({
  data,
  addMode,
  newPath,
  modifiedAt,
  origName,
  loading,
})

async function init() {
  const { name } = route.params

  data.value.name = name?.[name?.length - 1] ?? ''
  origName.value = data.value.name

  if (!addMode.value) {
    config.getItem(relativePath.value).then(r => {
      data.value = r
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
  loading.value = false
}

onMounted(async () => {
  await config.get_base_path().then(r => {
    nginxConfigBase.value = r.base_path
  })
  await init()
})

function save() {
  const payload = {
    name: addMode.value ? data.value.name : undefined,
    base_dir: addMode.value ? basePath.value : undefined,
    content: data.value.content,
    sync_node_ids: data.value.sync_node_ids,
    sync_overwrite: data.value.sync_overwrite,
  }

  const api = addMode.value
    ? config.createItem(payload)
    : config.updateItem(relativePath.value, payload)

  api.then(r => {
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
}

function formatCode() {
  ngx.format_code(data.value.content).then(r => {
    data.value.content = r.content
    message.success($gettext('Format successfully'))
  })
}

function goBack() {
  // Keep orignal path with encoding state
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
  <ACard
    :title="addMode ? $gettext('Add Configuration') : $gettext('Edit Configuration')"
    :bordered="false" :loading
  >
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
      class="mb-0!"
      banner
    />

    <CodeEditor
      v-model:content="data.content"
      no-border-radius
    />

    <FooterToolbar>
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
    </FooterToolbar>

    <ConfigHistory
      v-model:visible="showHistory"
      v-model:current-content="data.content"
      :filepath="data.filepath"
    />
  </ACard>
</template>

<style lang="less" scoped>
:deep(.ant-card-body) {
  max-height: calc(100vh - 260px);
  overflow-y: scroll;
  padding: 0;
}
</style>
