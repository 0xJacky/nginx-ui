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
    >
      <ACollapsePanel
        key="1"
        :header="$gettext('Custom')"
      >
        <div class="mb-4">
          <CodeEditor
            v-model:content="ngxConfig.custom"
            default-height="150px"
          />
        </div>
      </ACollapsePanel>
      <ACollapsePanel
        key="2"
        header="Upstream"
      >
        <NgxUpstream />
      </ACollapsePanel>
      <ACollapsePanel
        key="3"
        header="Server"
      >
        <NgxServer :context>
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
      </ACollapsePanel>
    </ACollapse>
  </div>
</template>

<style lang="less" scoped>
:deep(.ant-tabs-tab-btn) {
  margin-left: 16px;
}
</style>
