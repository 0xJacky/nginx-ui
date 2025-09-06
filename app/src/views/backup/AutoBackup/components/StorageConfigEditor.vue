<script setup lang="ts">
import type { AutoBackup } from '@/api/backup'

import { CheckCircleOutlined, LoadingOutlined } from '@ant-design/icons-vue'
import { testS3Connection } from '@/api/backup'

const modelValue = defineModel<AutoBackup>({ default: reactive({
  storage_type: 'local',
}) as AutoBackup })
const { message } = useGlobalApp()

const isLocalStorage = computed(() => modelValue.value.storage_type === 'local')
const isS3Storage = computed(() => modelValue.value.storage_type === 's3')
const isTestingS3 = ref(false)

onMounted(() => {
  if (!modelValue.value.storage_type)
    modelValue.value.storage_type = 'local'
})

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
        :options="[{ label: $gettext('Local'), value: 'local' },
                   { label: $gettext('S3'), value: 's3' }]"
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
  </div>
</template>

<style scoped lang="less">
</style>
