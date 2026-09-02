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
  curServerIdx.value = Number.parseInt((route.query?.server_idx ?? 0) as string)
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
