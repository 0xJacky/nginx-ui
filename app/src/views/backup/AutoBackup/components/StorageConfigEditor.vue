<script setup lang="ts">
import type { AutoBackup, StorageType } from '@/api/backup'

import { CheckCircleOutlined, LoadingOutlined } from '@antdv-next/icons'
import { testPluginStorage, testS3Connection } from '@/api/backup'
import PluginConfigForm from '@/components/PluginConfigForm'
import { findPluginBackend, isPluginStorageType, loadPluginBackends, pluginBackends, sanitizeStorageConfig } from '../pluginStorage'

const modelValue = defineModel<AutoBackup>({
  default: () => ({
    storage_type: 'local',
  }) as AutoBackup,
})
const { message } = useGlobalApp()

const isLocalStorage = computed(() => modelValue.value.storage_type === 'local')
const isS3Storage = computed(() => modelValue.value.storage_type === 's3')
const isPluginStorage = computed(() => isPluginStorageType(modelValue.value.storage_type))
const pluginBackend = computed(() => findPluginBackend(modelValue.value.storage_type))
const isTestingS3 = ref(false)
const isTestingPlugin = ref(false)

// The built-in storage first, then what plugins offer. A task whose plugin is
// gone keeps its stored type selectable, so it is not changed by accident.
const storageOptions = computed(() => {
  const options: { label: string, value: StorageType }[] = [
    { label: $gettext('Local'), value: 'local' },
    { label: $gettext('S3'), value: 's3' },
  ]
  for (const backend of pluginBackends.value)
    options.push({ label: backend.name, value: backend.type as StorageType })
  const current = modelValue.value.storage_type
  if (isPluginStorageType(current) && !options.some(option => option.value === current))
    options.push({ label: current, value: current })
  return options
})

const storageConfig = computed<Record<string, string>>({
  get: () => modelValue.value.storage_config ?? {},
  set: value => {
    modelValue.value.storage_config = value
  },
})

onMounted(() => {
  if (!modelValue.value.storage_type)
    modelValue.value.storage_type = 'local'
  loadPluginBackends(true)
})

// Keep only the values the selected backend declares.
watch(() => modelValue.value.storage_type, type => {
  if (isPluginStorageType(type) && findPluginBackend(type))
    modelValue.value.storage_config = sanitizeStorageConfig(type, modelValue.value.storage_config)
})

async function handleTestPluginStorage() {
  const backend = pluginBackend.value
  const missing = backend?.fields.find(field => field.required && !storageConfig.value[field.key])
  if (missing) {
    message.warning($gettext('Please fill in %{field}', { field: missing.display_name }))
    return
  }

  isTestingPlugin.value = true
  try {
    const res = await testPluginStorage(modelValue.value)
    message.success($gettext('Storage test successful, %{count} stored backups found', { count: res.stored ?? 0 }))
  }
  catch {
    // The request layer shows the error.
  }
  finally {
    isTestingPlugin.value = false
  }
}

async function handleTestS3Connection() {
  if (!modelValue.value.s3_bucket || !modelValue.value.s3_access_key_id || !modelValue.value.s3_secret_access_key) {
    message.warning($gettext('Please fill in required S3 configuration fields'))
    return
  }

  isTestingS3.value = true
  try {
    await testS3Connection(modelValue.value)
    message.success($gettext('S3 connection test successful'))
  }
  // eslint-disable-next-line ts/no-explicit-any
  catch (error: any) {
    const errorMessage = error?.response?.data?.error || error?.message || $gettext('S3 connection test failed')
    message.error(errorMessage)
  }
  finally {
    isTestingS3.value = false
  }
}
</script>

<template>
  <div>
    <AFormItem required :label="$gettext('Storage Type')">
      <ASelect
        v-model:value="modelValue.storage_type"
        :options="storageOptions"
      />
    </AFormItem>
    <AFormItem
      v-if="isLocalStorage"
      :label="$gettext('Storage Path')"
      name="storage_path"
      :rules="[{ required: true, message: $gettext('Storage path is required') }]"
    >
      <AInput
        v-model:value="modelValue.storage_path"
        :placeholder="isS3Storage ? $gettext('S3 path (e.g., backups/)') : $gettext('Local path (e.g., /var/backups)')"
      />
    </AFormItem>

    <template v-else-if="isS3Storage">
      <AFormItem
        :label="$gettext('S3 Endpoint')"
        name="s3_endpoint"
        :rules="[{ required: true, message: $gettext('S3 endpoint is required') }]"
      >
        <AInput
          v-model:value="modelValue.s3_endpoint"
          :placeholder="$gettext('S3 endpoint URL')"
        />
      </AFormItem>

      <AFormItem
        :label="$gettext('S3 Access Key ID')"
        name="s3_access_key_id"
        :rules="[{ required: true, message: $gettext('S3 access key ID is required') }]"
      >
        <AInput
          v-model:value="modelValue.s3_access_key_id"
          :placeholder="$gettext('S3 access key ID')"
        />
      </AFormItem>

      <AFormItem
        :label="$gettext('S3 Secret Access Key')"
        name="s3_secret_access_key"
        :rules="[{ required: true, message: $gettext('S3 secret access key is required') }]"
      >
        <AInputPassword
          v-model:value="modelValue.s3_secret_access_key"
          :placeholder="$gettext('S3 secret access key')"
        />
      </AFormItem>

      <AFormItem
        :label="$gettext('S3 Bucket')"
        name="s3_bucket"
        :rules="[{ required: true, message: $gettext('S3 bucket is required') }]"
      >
        <AInput
          v-model:value="modelValue.s3_bucket"
          :placeholder="$gettext('S3 bucket name')"
        />
      </AFormItem>

      <AFormItem
        :label="$gettext('S3 Region')"
        name="s3_region"
      >
        <AInput
          v-model:value="modelValue.s3_region"
          :placeholder="$gettext('S3 region (e.g., us-east-1)')"
        />
      </AFormItem>

      <AFormItem
        :label="$gettext('Storage Path')"
        name="storage_path"
        :rules="[{ required: true, message: $gettext('Storage path is required') }]"
      >
        <AInput
          v-model:value="modelValue.storage_path"
          :placeholder="$gettext('S3 path (e.g., backups/)')"
        />
      </AFormItem>

      <AFormItem>
        <AButton
          type="primary"
          ghost
          :loading="isTestingS3"
          @click="handleTestS3Connection"
        >
          <template #icon>
            <CheckCircleOutlined v-if="!isTestingS3" />
            <LoadingOutlined v-else />
          </template>
          {{ $gettext('Test S3 Connection') }}
        </AButton>
      </AFormItem>
    </template>

    <template v-else-if="isPluginStorage">
      <AAlert
        v-if="!pluginBackend"
        class="mb-4"
        type="warning"
        show-icon
        :message="$gettext('The plugin that provides this storage is not enabled. Backups fail until it is enabled again.')"
      />
      <PluginConfigForm
        v-if="pluginBackend"
        v-model="storageConfig"
        :fields="pluginBackend.fields"
      />

      <AFormItem
        :label="$gettext('Key Prefix')"
        name="storage_path"
        :extra="$gettext('Backups are stored under this prefix, for example nginx-ui/daily. Use / for the root.')"
        :rules="[{ required: true, message: $gettext('Key prefix is required') }]"
      >
        <AInput
          v-model:value="modelValue.storage_path"
          :placeholder="$gettext('nginx-ui/backups')"
        />
      </AFormItem>

      <AFormItem
        :label="$gettext('Keep Latest Backups')"
        :extra="$gettext('Older backups in this storage are deleted after each run. 0 keeps every backup.')"
      >
        <AInputNumber
          v-model:value="modelValue.retention_count"
          :min="0"
          :precision="0"
          class="w-full"
        />
      </AFormItem>

      <AFormItem v-if="pluginBackend">
        <AButton
          type="primary"
          ghost
          :loading="isTestingPlugin"
          @click="handleTestPluginStorage"
        >
          <template #icon>
            <CheckCircleOutlined v-if="!isTestingPlugin" />
            <LoadingOutlined v-else />
          </template>
          {{ $gettext('Test Storage') }}
        </AButton>
      </AFormItem>
    </template>
  </div>
</template>

<style scoped lang="less">
</style>
