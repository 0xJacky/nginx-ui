<script setup lang="ts">
import { useGettext } from 'vue3-gettext'
import ObtainCert from '@/views/domain/cert/components/ObtainCert.vue'
import type { NgxDirective } from '@/api/ngx'

const props = defineProps<{
  enabled: boolean
}>()

const emit = defineEmits(['callback', 'update:enabled'])

const { $gettext } = useGettext()
const issuing_cert = ref(false)
const obtain_cert = ref()
const directivesMap = inject('directivesMap') as Record<string, NgxDirective[]>

const enabled = computed({
  get() {
    return props.enabled
  },
  set(value) {
    emit('update:enabled', value)
  },
})

const no_server_name = computed(() => {
  if (!directivesMap.server_name)
    return true

  return directivesMap.server_name.length === 0
})

provide('no_server_name', no_server_name)
provide('props', props)
provide('issuing_cert', issuing_cert)

watch(no_server_name, () => emit('update:enabled', false))

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

<style lang="less">
.issue-cert-log-container {
  height: 320px;
  overflow: scroll;
  background-color: #f3f3f3;
  border-radius: 4px;
  margin-top: 15px;
  padding: 10px;

  p {
    font-size: 12px;
    line-height: 1.3;
  }
}
</style>

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
