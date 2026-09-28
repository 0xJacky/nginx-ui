<script setup lang="ts">
import CodeEditor from '@/components/CodeEditor'
import { NginxStatusAlert, NgxServer, NgxUpstream, useNgxConfigStore } from '.'

withDefaults(defineProps<{
  context?: 'http' | 'stream'
}>(), {
  context: 'http',
})

const ngxConfigStore = useNgxConfigStore()
const { ngxConfig, curServerIdx } = storeToRefs(ngxConfigStore)

const route = useRoute()

onMounted(() => {
  // Only restore the tab from the URL when it names one; otherwise keep the
  // index the caller selected (e.g. the TLS server in the add-site wizard).
  // setNgxConfig() clamps the index again once a config is loaded.
  const serverIdx = Number(route.query?.server_idx)
  const serverCount = ngxConfig.value.servers?.length ?? 0
  const isValidIdx = route.query?.server_idx !== undefined
    && Number.isInteger(serverIdx)
    && serverIdx >= 0
    && (serverCount === 0 || serverIdx < serverCount)
  if (isValidIdx)
    curServerIdx.value = serverIdx
})

const activeKey = ref(['3'])
</script>

<template>
  <div>
    <NginxStatusAlert />

    <ACollapse
      v-model:active-key="activeKey"
      ghost
      :items="[
        { key: '1', label: $gettext('Custom') },
        { key: '2', label: 'Upstream' },
        { key: '3', label: 'Server' },
      ]"
    >
      <template #contentRender="{ item }">
        <div
          v-if="item.key === '1'"
          class="mb-4"
        >
          <CodeEditor
            v-model:content="ngxConfig.custom"
            default-height="150px"
          />
        </div>
        <NgxUpstream
          v-else-if="item.key === '2'"
          :context
        />
        <NgxServer
          v-else-if="item.key === '3'"
          :context
        >
          <template
            v-for="(_, key) in $slots"
            :key="key"
            #[key]="slotProps"
          >
            <slot
              :name="key"
              v-bind="slotProps"
            />
          </template>
        </NgxServer>
      </template>
    </ACollapse>
  </div>
</template>

<style lang="less" scoped>
:deep(.ant-tabs-tab-btn) {
  margin-left: 16px;
}
</style>
