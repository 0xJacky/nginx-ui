<script setup lang="ts">
import ObtainCert from '@/views/domain/cert/components/ObtainCert.vue'
import type { NgxDirective } from '@/api/ngx'

export interface Props {
  configName: string
}

const props = defineProps<Props>()

const issuing_cert = ref(false)
const obtain_cert = ref()
const directivesMap = inject('directivesMap') as Ref<Record<string, NgxDirective[]>>

const enabled = defineModel<boolean>('enabled', {
  default: () => false,
})

const no_server_name = computed(() => {
  if (!directivesMap.value.server_name)
    return true

  return directivesMap.value.server_name.length === 0
})

provide('no_server_name', no_server_name)
provide('props', props)
provide('issuing_cert', issuing_cert)

watch(no_server_name, () => {
  enabled.value = false
})

const update = ref(0)

async function onchange() {
  update.value++
  await nextTick(() => {
    obtain_cert.value.toggle(enabled.value)
  })
}
</script>

<template>
  <ObtainCert
    ref="obtain_cert"
    :key="update"
    @update:auto_cert="r => enabled = r"
  />
  <div class="issue-cert">
    <AFormItem :label="$gettext('Encrypt website with Let\'s Encrypt')">
      <ASwitch
        :loading="issuing_cert"
        :checked="enabled"
        :disabled="no_server_name"
        @change="onchange"
      />
    </AFormItem>
  </div>
</template>

<style lang="less" scoped>
.ant-tag {
  margin: 0;
}

.issue-cert {
  margin: 15px 0;
}

.switch-wrapper {
  position: relative;

  .text {
    position: absolute;
    top: 50%;
    transform: translateY(-50%);
    margin-left: 10px;
  }
}
</style>
