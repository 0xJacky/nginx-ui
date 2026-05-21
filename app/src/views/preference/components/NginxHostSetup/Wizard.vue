<script setup lang="ts">
import type { SetupParams } from '@/api/host_setup'
import { storeToRefs } from 'pinia'
import { ref, watch } from 'vue'
import useSystemSettingsStore from '../../store'
import Step1 from './steps/Step1AuthMethod.vue'
import Step2a from './steps/Step2aContainer.vue'
import Step2b from './steps/Step2bHost.vue'
import Step3 from './steps/Step3Connection.vue'
import Step4 from './steps/Step4Verify.vue'
import Step5 from './steps/Step5HostIdentity.vue'

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

const systemSettingsStore = useSystemSettingsStore()
const { data } = storeToRefs(systemSettingsStore)

// Write wizard params into the global settings store so the existing
// "Save" button in Preference.vue's FooterToolBar persists them.
function saveToSettings() {
  data.value.nginx.host_address = params.value.host_address
  data.value.nginx.host_user = params.value.host_user
  data.value.nginx.host_systemd_unit_name = params.value.systemd_unit
  data.value.nginx.host_systemctl_path = params.value.systemctl_path
  data.value.nginx.host_config_dir = params.value.host_config_dir
  data.value.nginx.host_log_dir = params.value.host_log_dir
}

function next() {
  if (current.value < 5)
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
      <AStep :title="$gettext('Host Identity')" />
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
      <Step5 :params="params" />
    </div>
    <div v-else-if="current === 5">
      <Step4 />
    </div>

    <div class="mt-6 flex justify-between">
      <AButton :disabled="current === 0" @click="prev">
        {{ $gettext('Previous') }}
      </AButton>
      <AButton v-if="current < 5" type="primary" @click="next">
        {{ $gettext('Next') }}
      </AButton>
      <AButton v-if="current === 5" type="primary" @click="saveToSettings">
        {{ $gettext('Save configuration') }}
      </AButton>
    </div>
  </ACard>
</template>
