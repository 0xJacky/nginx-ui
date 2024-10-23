<script setup lang="ts">
import config from '@/api/config'
import FooterToolBar from '@/components/FooterToolbar/FooterToolBar.vue'
import StdTable from '@/components/StdDesign/StdDataDisplay/StdTable.vue'
import { useBreadcrumbs } from '@/composables/useBreadcrumbs'
import Mkdir from '@/views/config/components/Mkdir.vue'
import Rename from '@/views/config/components/Rename.vue'
import configColumns from '@/views/config/configColumns'
import InspectConfig from '@/views/config/InspectConfig.vue'
import { $gettext } from '../../gettext'

const table = ref()
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

const refInspectConfig = ref()
const breadcrumbs = useBreadcrumbs()

function updateBreadcrumbs() {
  const filteredPath = basePath.value
    .split('/')
    .filter(v => v)

  const path = filteredPath.map((v, k) => {
    let dir = v

    if (k > 0) {
      dir = filteredPath.slice(0, k).join('/')
      dir += `/${v}`
    }

    return {
      name: 'Manage Configs',
      translatedName: () => v,
      path: '/config',
      query: {
        dir,
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
  router.push({
    path: '/config',
    query: {
      dir: `${basePath.value.split('/').slice(0, -2).join('/')}` || undefined,
    },
  })
}

const refMkdir = ref()
const refRename = ref()
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
        @click="() => refMkdir.open(basePath)"
      >
        {{ $gettext('Create Folder') }}
      </AButton>
    </template>
    <InspectConfig ref="refInspectConfig" />
    <StdTable
      :key="update"
      ref="table"
      :api="config"
      :columns="configColumns"
      disable-delete
      disable-view
      row-key="name"
      :get-params="getParams"
      disable-query-params
      disable-modify
    >
      <template #actions="{ record }">
        <AButton
          type="link"
          size="small"
          @click="() => {
            if (!record.is_dir) {
              $router.push({
                path: `/config/${basePath}${record.name}/edit`,
              })
            }
            else {
              $router.push({
                query: {
                  dir: basePath + record.name,
                },
              })
            }
          }"
        >
          {{ $gettext('Modify') }}
        </AButton>
        <ADivider type="vertical" />
        <AButton
          type="link"
          size="small"
          @click="() => refRename.open(basePath, record.name, record.is_dir)"
        >
          {{ $gettext('Rename') }}
        </AButton>
      </template>
    </StdTable>
    <Mkdir
      ref="refMkdir"
      @created="() => table.get_list()"
    />
    <Rename
      ref="refRename"
      @renamed="() => table.get_list()"
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
