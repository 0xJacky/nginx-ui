<script setup lang="ts">
import type { SetupParams } from '@/api/host_setup'
import { ref, watch } from 'vue'
import Step1 from './steps/Step1AuthMethod.vue'
import Step2a from './steps/Step2aContainer.vue'
import Step2b from './steps/Step2bHost.vue'
import Step3 from './steps/Step3Connection.vue'
import Step4 from './steps/Step4Verify.vue'

const current = ref(0)
const authMethod = ref<'key' | 'password'>('key')
const publicKey = ref('')

const params = ref<SetupParams>({
  host_address: 'host.docker.internal:22',
  host_user: 'nginxui',
  systemd_unit: 'nginx.service',
  systemctl_path: '/bin/systemctl',
  nginx_sbin_path: '/usr/sbin/nginx',
  host_config_dir: '/etc/nginx',
  host_log_dir: '/var/log/nginx',
  use_generated_key: true,
  public_key_open_ssh: '',
})

watch(publicKey, v => {
  params.value.public_key_open_ssh = v
})

function next() {
  if (current.value < 4)
    current.value++
}
function prev() {
  if (current.value > 0)
    current.value--
}
</script>

<template>
  <ACard :title="$gettext('Host SSH setup wizard')">
    <ASteps :current="current" size="small" class="mb-4">
      <AStep :title="$gettext('Auth')" />
      <AStep :title="$gettext('Container')" />
      <AStep :title="$gettext('Host')" />
      <AStep :title="$gettext('Connection')" />
      <AStep :title="$gettext('Verify')" />
    </ASteps>

    <div v-if="current === 0">
      <Step1 v-model:auth-method="authMethod" v-model:public-key="publicKey" />
    </div>
    <div v-else-if="current === 1">
      <Step2a :params="params" />
    </div>
    <div v-else-if="current === 2">
      <Step2b :params="params" />
    </div>
    <div v-else-if="current === 3">
      <Step3 v-model:params="params" />
    </div>
    <div v-else-if="current === 4">
      <Step4 />
    </div>

    <div class="mt-6 flex justify-between">
      <AButton :disabled="current === 0" @click="prev">
        {{ $gettext('Previous') }}
      </AButton>
      <AButton v-if="current < 4" type="primary" @click="next">
        {{ $gettext('Next') }}
      </AButton>
    </div>
  </ACard>
</template>
