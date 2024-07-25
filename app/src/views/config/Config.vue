<script setup lang="ts">
import StdTable from '@/components/StdDesign/StdDataDisplay/StdTable.vue'
import config from '@/api/config'
import configColumns from '@/views/config/config'
import FooterToolBar from '@/components/FooterToolbar/FooterToolBar.vue'
import InspectConfig from '@/views/config/InspectConfig.vue'

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

watch(route, () => {
  refInspectConfig.value?.test()
})

function goBack() {
  router.push({
    path: '/config',
    query: {
      dir: `${basePath.value.split('/').slice(0, -2).join('/')}` || undefined,
    },
  })
}
</script>

<template>
  <ACard :title="$gettext('Configurations')">
    <template #extra>
      <a
        @click="router.push({
          path: '/config/add',
          query: { basePath: basePath || undefined },
        })"
      >{{ $gettext('Add') }}</a>
    </template>
    <InspectConfig ref="refInspectConfig" />
    <StdTable
      :key="update"
      ref="table"
      :api="config"
      :columns="configColumns"
      disable-delete
      disable-search
      disable-view
      row-key="name"
      :get-params="getParams"
      disable-query-params
      @click-edit="(r, row) => {
        if (!row.is_dir) {
          $router.push({
            path: `/config/${basePath}${r}/edit`,
          })
        }
        else {
          $router.push({
            query: {
              dir: basePath + r,
            },
          })
        }
      }"
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
