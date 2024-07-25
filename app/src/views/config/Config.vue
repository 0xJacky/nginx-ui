<script setup lang="ts">
import StdTable from '@/components/StdDesign/StdDataDisplay/StdTable.vue'
import config from '@/api/config'
import configColumns from '@/views/config/config'
import FooterToolBar from '@/components/FooterToolbar/FooterToolBar.vue'
import router from '@/routes'
import InspectConfig from '@/views/config/InspectConfig.vue'

const api = config

const table = ref(null)
const route = useRoute()

const basePath = computed(() => {
  let dir = route?.query?.dir ?? ''
  if (dir)
    dir += '/'

  return dir
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
</script>

<template>
  <ACard :title="$gettext('Configurations')">
    <InspectConfig ref="refInspectConfig" />
    <StdTable
      :key="update"
      ref="table"
      :api="api"
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
      <AButton @click="router.go(-1)">
        {{ $gettext('Back') }}
      </AButton>
    </FooterToolBar>
  </ACard>
</template>

<style scoped>

</style>
