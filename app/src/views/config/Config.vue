<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
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

const get_params = computed(() => {
  return {
    dir: basePath.value,
  }
})

const update = ref(1)

watch(get_params, () => {
  update.value++
})

const inspect_config = ref()

watch(route, () => {
  inspect_config.value?.test()
})
</script>

<template>
  <ACard :title="$gettext('Configurations')">
    <InspectConfig ref="inspect_config" />
    <StdTable
      :key="update"
      ref="table"
      :api="api"
      :columns="configColumns"
      disable-delete
      disable_search
      disabled-view
      row-key="name"
      :get_params="get_params"
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
