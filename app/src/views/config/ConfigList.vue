<script setup lang="ts">
import { StdTable } from '@uozi-admin/curd'
import config from '@/api/config'
import FooterToolBar from '@/components/FooterToolbar'
import { useBreadcrumbs } from '@/composables/useBreadcrumbs'
import { isProtectedPath } from '@/views/config/configUtils'
import Delete from './components/Delete.vue'
import Mkdir from './components/Mkdir.vue'
import Rename from './components/Rename.vue'
import configColumns from './configColumns'
import InspectConfig from './InspectConfig.vue'

const table = useTemplateRef('table')
const route = useRoute()
const router = useRouter()

const basePath = computed(() => {
  let dir = route?.query?.dir ?? ''
  if (dir)
    dir += '/'

  return dir as string
})

const getParams = computed(() => {
  return {
    dir: basePath.value,
  }
})

const update = ref(1)

watch(getParams, () => {
  update.value++
})

const refInspectConfig = useTemplateRef('refInspectConfig')
const breadcrumbs = useBreadcrumbs()

function updateBreadcrumbs() {
  const filteredPath = basePath.value
    .split('/')
    .filter(v => v)

  let accumulatedPath = ''
  const path = filteredPath.map((segment, index) => {
    const decodedSegment = decodeURIComponent(segment)

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
  }, ...path]
}

onMounted(() => {
  updateBreadcrumbs()
})

watch(route, () => {
  refInspectConfig.value?.test()
  updateBreadcrumbs()
})

function goBack() {
  const pathSegments = basePath.value.split('/').slice(0, -2)
  const encodedPath = pathSegments.length > 0 ? pathSegments.join('/') : ''

  router.push({
    path: '/config',
    query: {
      dir: encodedPath || undefined,
    },
  })
}

const refMkdir = useTemplateRef('refMkdir')
const refRename = useTemplateRef('refRename')
const refDelete = useTemplateRef('refDelete')

// Check if a file/directory is protected
function isProtected(name: string) {
  return isProtectedPath(name)
}
</script>

<template>
  <ACard :title="$gettext('Configurations')">
    <template #extra>
      <AButton
        v-if="basePath"
        type="link"
        size="small"
        @click="goBack"
      >
        {{ $gettext('Back') }}
      </AButton>
      <AButton
        type="link"
        size="small"
        @click="router.push({
          path: '/config/add',
          query: { basePath: basePath || undefined },
        })"
      >
        {{ $gettext('Create File') }}
      </AButton>
      <AButton
        type="link"
        size="small"
        @click="() => refMkdir?.open(basePath)"
      >
        {{ $gettext('Create Folder') }}
      </AButton>
    </template>
    <InspectConfig ref="refInspectConfig" />
    <StdTable
      :key="update"
      ref="table"
      :get-list-api="config.getList"
      :columns="configColumns"
      disable-delete
      disable-view
      row-key="name"
      :custom-query-params="getParams"
      disable-router-query
      disable-edit
      :scroll-x="880"
    >
      <template #beforeActions="{ record }">
        <AButton
          type="link"
          size="small"
          @click="() => {
            if (!record.is_dir) {
              router.push({
                path: `/config/${encodeURIComponent(record.name)}/edit`,
                query: {
                  basePath,
                },
              })
            }
            else {
              let encodedPath = '';
              if (basePath) {
                encodedPath = basePath;
              }
              encodedPath += encodeURIComponent(record.name);

              router.push({
                query: {
                  dir: encodedPath,
                },
              })
            }
          }"
        >
          {{ $gettext('Modify') }}
        </AButton>
        <AButton
          v-if="!isProtected(record.name)"
          type="link"
          size="small"
          @click="() => refRename?.open(basePath, record.name, record.is_dir)"
        >
          {{ $gettext('Rename') }}
        </AButton>
        <AButton
          v-if="!isProtected(record.name)"
          type="link"
          size="small"
          danger
          @click="() => refDelete?.open(basePath, record.name, record.is_dir)"
        >
          {{ $gettext('Delete') }}
        </AButton>
      </template>
    </StdTable>
    <Mkdir
      ref="refMkdir"
      @created="() => table?.refresh()"
    />
    <Rename
      ref="refRename"
      @renamed="() => table?.refresh()"
    />
    <Delete
      ref="refDelete"
      @deleted="() => table?.refresh()"
    />
    <FooterToolBar v-if="basePath">
      <AButton @click="goBack">
        {{ $gettext('Back') }}
      </AButton>
    </FooterToolBar>
  </ACard>
</template>

<style scoped>

</style>
