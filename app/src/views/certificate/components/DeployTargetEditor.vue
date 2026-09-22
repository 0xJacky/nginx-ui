<script setup lang="ts">
import type { CertDeployTarget } from '@/api/cert_deploy'
import { ExperimentOutlined } from '@antdv-next/icons'
import { testDeployTarget } from '@/api/cert_deploy'
import PluginConfigForm from '@/components/PluginConfigForm'
import {
  certificateOptions,
  deployKinds,
  findDeployKind,
  loadCertificateOptions,
  loadDeployKinds,
  sanitizeDeployConfig,
} from '../deployTargets'

// The kind, form values and certificate binding of a deploy target, with a
// dry run that checks the target without changing anything.
const target = defineModel<CertDeployTarget>({ required: true })

const { message } = useGlobalApp()
const testing = ref(false)

onMounted(() => {
  loadDeployKinds(true)
  loadCertificateOptions()
  target.value.cert_id ??= 0
  if (!target.value.config)
    target.value.config = {}
})

const kind = computed(() => findDeployKind(target.value.kind))

// A target whose plugin is gone keeps its stored kind selectable, so it is
// not changed by accident.
const kindOptions = computed(() => {
  const options = deployKinds.value.map(item => ({ label: item.name, value: item.kind }))
  if (target.value.kind && !options.some(option => option.value === target.value.kind))
    options.push({ label: target.value.kind, value: target.value.kind })
  return options
})

const certOptions = computed(() => [
  { label: $gettext('All certificates'), value: 0 },
  ...certificateOptions.value.map(item => ({ label: item.name, value: item.id })),
])

const config = computed<Record<string, string>>({
  get: () => target.value.config ?? {},
  set: value => {
    target.value.config = value
  },
})

// Keep only the values the selected kind declares.
watch(() => target.value.kind, value => {
  if (findDeployKind(value))
    target.value.config = sanitizeDeployConfig(value, target.value.config)
})

async function handleDryRun() {
  const missing = kind.value?.fields.find(field => field.required && !config.value[field.key])
  if (missing) {
    message.warning($gettext('Please fill in %{field}', { field: missing.display_name }))
    return
  }

  testing.value = true
  try {
    const res = await testDeployTarget({
      kind: target.value.kind,
      config: config.value,
      cert_id: target.value.cert_id ?? 0,
    })
    message.success(res.message || $gettext('Dry run successful'))
  }
  catch {
    // The request layer shows the error.
  }
  finally {
    testing.value = false
  }
}
</script>

<template>
  <div>
    <AAlert
      v-if="!deployKinds.length"
      class="mb-4"
      type="info"
      show-icon
      :message="$gettext('No enabled plugin offers a deploy target. Install and enable a plugin with the cert.deploy capability first.')"
    />
    <AFormItem required :label="$gettext('Target Type')">
      <ASelect
        v-model:value="target.kind"
        :options="kindOptions"
        :placeholder="$gettext('Select a target type')"
      />
    </AFormItem>
    <AAlert
      v-if="target.kind && !kind"
      class="mb-4"
      type="warning"
      show-icon
      :message="$gettext('The plugin that provides this target type is not enabled. Pushes fail until it is enabled again.')"
    />

    <PluginConfigForm
      v-if="kind"
      v-model="config"
      :fields="kind.fields"
    />

    <AFormItem
      :label="$gettext('Certificate')"
      :extra="$gettext('The certificate pushed to this target after every issuance and renewal.')"
    >
      <ASelect
        v-model:value="target.cert_id"
        :options="certOptions"
        show-search
        option-filter-prop="label"
      />
    </AFormItem>

    <AFormItem v-if="kind">
      <AButton
        type="primary"
        ghost
        :loading="testing"
        @click="handleDryRun"
      >
        <template #icon>
          <ExperimentOutlined />
        </template>
        {{ $gettext('Dry Run') }}
      </AButton>
    </AFormItem>
  </div>
</template>
