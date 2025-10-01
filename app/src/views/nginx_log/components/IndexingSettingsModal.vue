<script setup lang="tsx">
import { CheckCircleOutlined, CloseCircleOutlined, HeartOutlined, InfoCircleOutlined, MailOutlined, ThunderboltOutlined, WarningOutlined } from '@ant-design/icons-vue'

interface Props {
  loading?: boolean
}

interface Emits {
  (e: 'confirm'): void
  (e: 'cancel'): void
}

withDefaults(defineProps<Props>(), {
  loading: false,
})

const emit = defineEmits<Emits>()

const visible = defineModel<boolean>('visible', { required: true })

const systemRequirements = [
  {
    title: $gettext('CPU'),
    requirement: $gettext('1 core minimum'),
    recommended: $gettext('2+ cores recommended'),
    icon: <CheckCircleOutlined class="text-green-500" />,
  },
  {
    title: $gettext('Memory'),
    requirement: $gettext('1GB RAM minimum'),
    recommended: $gettext('4GB+ RAM recommended'),
    icon: <CheckCircleOutlined class="text-green-500" />,
  },
  {
    title: $gettext('Storage'),
    requirement: $gettext('At least 20GB available disk space'),
    recommended: $gettext('SSD storage for better I/O performance'),
    icon: <CheckCircleOutlined class="text-green-500" />,
  },
]

const performanceStats = [
  {
    metric: $gettext('Production Pipeline'),
    value: '~10,000/sec',
    description: $gettext('Complete indexing with search capabilities'),
  },
  {
    metric: $gettext('Parser Performance'),
    value: '~932K/sec',
    description: $gettext('SIMD-optimized stream processing'),
  },
  {
    metric: $gettext('Memory Design'),
    value: 'Zero-allocation',
    description: $gettext('Advanced memory pooling system'),
  },
]

function handleConfirm() {
  emit('confirm')
}

function handleCancel() {
  visible.value = false
  emit('cancel')
}
</script>

<template>
  <AModal
    :open="visible"
    :title="$gettext('Enable Advanced Log Indexing')"
    :confirm-loading="loading"
    :ok-text="$gettext('Enable Indexing')"
    :cancel-text="$gettext('Cancel')"
    width="720px"
    @ok="handleConfirm"
    @cancel="handleCancel"
  >
    <div class="space-y-6">
      <!-- Warning Alert -->
      <AAlert
        :message="$gettext('Resource Usage Warning')"
        :description="$gettext('Enabling advanced log indexing will consume significant computational resources including CPU and memory. Please ensure your system meets the minimum requirements before proceeding.')"
        type="warning"
        show-icon
        :icon="h(WarningOutlined)"
      />

      <!-- System Requirements -->
      <div>
        <ATypographyTitle :level="4" class="mb-3">
          <InfoCircleOutlined class="mr-2" />
          {{ $gettext('System Requirements') }}
        </ATypographyTitle>

        <AList
          :data-source="systemRequirements"
          item-layout="horizontal"
        >
          <template #renderItem="{ item }">
            <AListItem>
              <AListItemMeta>
                <template #avatar>
                  <component :is="item.icon" />
                </template>
                <template #title>
                  <ATypographyText strong>
                    {{ item.title }}
                  </ATypographyText>
                </template>
                <template #description>
                  <div class="space-y-1">
                    <div>
                      <ATypographyText>
                        {{ $gettext('Minimum:') }}
                      </ATypographyText>
                      <ATypographyText type="secondary">
                        {{ item.requirement }}
                      </ATypographyText>
                    </div>
                    <div>
                      <ATypographyText>
                        {{ $gettext('Recommended:') }}
                      </ATypographyText>
                      <ATypographyText type="secondary">
                        {{ item.recommended }}
                      </ATypographyText>
                    </div>
                  </div>
                </template>
              </AListItemMeta>
            </AListItem>
          </template>
        </AList>

        <!-- Subtle note about Bleve index storage location -->
        <div class="mt-2">
          <ATypographyText type="secondary" class="text-xs">
            {{ $gettext('* Index files are stored in the "log-index" directory within your Nginx UI config path by default.') }}
            {{ $gettext('If you want to change the storage location, you can set the `IndexPath` of `nginx_log` section in the Nginx UI config.') }}
          </ATypographyText>
        </div>
      </div>

      <ADivider />

      <!-- Performance Statistics -->
      <div>
        <ATypographyTitle :level="4" class="mb-3">
          <CheckCircleOutlined class="mr-2 text-green-500" />
          {{ $gettext('Expected Performance') }}
        </ATypographyTitle>

        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          <div
            v-for="stat in performanceStats"
            :key="stat.metric"
            class="bg-gray-50 dark:bg-gray-800 p-4 rounded-lg"
          >
            <div class="text-xl font-bold text-blue-600 dark:text-blue-400 mb-1">
              {{ stat.value }}
            </div>
            <div class="font-medium text-gray-900 dark:text-gray-100 mb-1 text-sm">
              {{ stat.metric }}
            </div>
            <div class="text-sm text-gray-600 dark:text-gray-400">
              {{ stat.description }}
            </div>
          </div>
        </div>

        <div class="mt-3">
          <ATypographyText type="secondary" class="text-xs">
            {{ $gettext('* Performance metrics measured on Apple M2 Pro (12-core) with 32GB RAM. Actual performance may vary based on your hardware configuration.') }}
          </ATypographyText>
        </div>
      </div>

      <ADivider />

      <!-- Features -->
      <div>
        <ATypographyTitle :level="4" class="mb-3">
          <ThunderboltOutlined class="mr-2 text-blue-500" />
          {{ $gettext('Features') }}
        </ATypographyTitle>

        <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
          <div class="flex items-center space-x-2">
            <CheckCircleOutlined class="text-green-500" />
            <ATypographyText>{{ $gettext('Zero-allocation pipeline') }}</ATypographyText>
          </div>
          <div class="flex items-center space-x-2">
            <CheckCircleOutlined class="text-green-500" />
            <ATypographyText>{{ $gettext('Dynamic shard management') }}</ATypographyText>
          </div>
          <div class="flex items-center space-x-2">
            <CheckCircleOutlined class="text-green-500" />
            <ATypographyText>{{ $gettext('Advanced search & filtering') }}</ATypographyText>
          </div>
          <div class="flex items-center space-x-2">
            <CheckCircleOutlined class="text-green-500" />
            <ATypographyText>{{ $gettext('Real-time analytics dashboard') }}</ATypographyText>
          </div>
          <div class="flex items-center space-x-2">
            <CheckCircleOutlined class="text-green-500" />
            <ATypographyText>{{ $gettext('Offline GeoIP analysis') }}</ATypographyText>
          </div>
          <div class="flex items-center space-x-2">
            <CheckCircleOutlined class="text-green-500" />
            <ATypographyText>{{ $gettext('Incremental index scanning') }}</ATypographyText>
          </div>
          <div class="flex items-center space-x-2">
            <CheckCircleOutlined class="text-green-500" />
            <ATypographyText>{{ $gettext('Full-text search support') }}</ATypographyText>
          </div>
          <div class="flex items-center space-x-2">
            <CheckCircleOutlined class="text-green-500" />
            <ATypographyText>{{ $gettext('Automated log rotation detection') }}</ATypographyText>
          </div>
          <div class="flex items-center space-x-2">
            <CheckCircleOutlined class="text-green-500" />
            <ATypographyText>{{ $gettext('Cross-file timeline correlation') }}</ATypographyText>
          </div>
          <div class="flex items-center space-x-2">
            <CheckCircleOutlined class="text-green-500" />
            <ATypographyText>{{ $gettext('Compressed log file support') }}</ATypographyText>
          </div>
          <div class="flex items-center space-x-2">
            <CheckCircleOutlined class="text-green-500" />
            <ATypographyText>{{ $gettext('Error pattern recognition') }}</ATypographyText>
          </div>
          <div class="flex items-center space-x-2">
            <CheckCircleOutlined class="text-green-500" />
            <ATypographyText>{{ $gettext('Multi-dimensional data visualization') }}</ATypographyText>
          </div>
        </div>
      </div>

      <!-- License Notice -->
      <div>
        <ATypographyTitle :level="4" class="mb-3">
          <HeartOutlined class="mr-2 text-red-500" />
          {{ $gettext('Open Source Limitation') }}
        </ATypographyTitle>

        <div class="space-y-2">
          <div class="flex items-center space-x-2">
            <CheckCircleOutlined class="text-green-500" />
            <ATypographyText class="text-sm">
              {{ $gettext('Advanced log indexing features are free and open source for all users') }}
            </ATypographyText>
          </div>
          <div class="flex items-center space-x-2">
            <CloseCircleOutlined class="text-orange-500" />
            <ATypographyText class="text-sm">
              {{ $gettext('We do not accept any feature requests') }}
            </ATypographyText>
          </div>
          <div class="flex items-center space-x-2">
            <MailOutlined class="text-blue-500" />
            <ATypographyText class="text-sm">
              {{ $gettext('For commercial or professional use, contact') }}
              <a href="mailto:business@uozi.com" class="text-blue-600 hover:text-blue-800">business@uozi.com</a>
            </ATypographyText>
          </div>
        </div>
      </div>

      <!-- Final Warning -->
      <AAlert
        :message="$gettext('Confirmation Required')"
        :description="$gettext('By enabling advanced indexing, you acknowledge that your system meets the requirements and understand the performance implications. This will start indexing existing log files immediately.')"
        type="info"
        show-icon
        :icon="h(InfoCircleOutlined)"
      />
    </div>
  </AModal>
</template>

<style scoped lang="less">
:deep(.ant-list-item-meta-description) {
  margin-top: 8px;
}
</style>
