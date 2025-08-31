<script setup lang="ts">
import type { License, LicenseStats } from '@/api/license'
import license from '@/api/license'

const loading = ref(false)
const activeTab = ref('all')
const backendLicenses = ref<License[]>([])
const frontendLicenses = ref<License[]>([])
const stats = ref<LicenseStats>()

const columns = [
  {
    title: $gettext('Name'),
    dataIndex: 'name',
    key: 'name',
    sorter: (a: License, b: License) => a.name.localeCompare(b.name),
    width: 300,
    ellipsis: true,
  },
  {
    title: $gettext('License'),
    dataIndex: 'license',
    key: 'license',
    sorter: (a: License, b: License) => a.license.localeCompare(b.license),
    width: 120,
  },
  {
    title: $gettext('Version'),
    dataIndex: 'version',
    key: 'version',
    width: 120,
  },
  {
    title: $gettext('URL'),
    dataIndex: 'url',
    key: 'url',
    width: 80,
  },
]

async function fetchLicenses() {
  loading.value = true
  try {
    const [backendRes, frontendRes, statsRes] = await Promise.all([
      license.getBackend(),
      license.getFrontend(),
      license.getStats(),
    ])

    backendLicenses.value = backendRes
    frontendLicenses.value = frontendRes
    stats.value = statsRes
  }
  catch (error) {
    console.error(error)
  }
  finally {
    loading.value = false
  }
}

function getAllLicenses() {
  return [...backendLicenses.value, ...frontendLicenses.value]
}

function getCurrentLicenses() {
  switch (activeTab.value) {
    case 'backend':
      return backendLicenses.value
    case 'frontend':
      return frontendLicenses.value
    default:
      return getAllLicenses()
  }
}

onMounted(() => {
  fetchLicenses()
})
</script>

<script lang="ts">
function getLicenseColor(license: string): string {
  const colors: Record<string, string> = {
    'MIT': 'green',
    'Apache-2.0': 'blue',
    'BSD-3-Clause': 'cyan',
    'BSD-2-Clause': 'cyan',
    'GPL-3.0': 'orange',
    'AGPL-3.0': 'red',
    'ISC': 'geekblue',
    'Unknown': 'default',
    'Custom': 'purple',
  }
  return colors[license] || 'default'
}

export { getLicenseColor }
</script>

<template>
  <div>
    <ACard v-if="stats" class="mb-4">
      <ARow :gutter="[16, 16]">
        <ACol :xs="12" :sm="12" :md="6" :lg="6" :xl="6">
          <AStatistic
            :title="$gettext('Total Components')"
            :value="stats.total"
          />
        </ACol>
        <ACol :xs="12" :sm="12" :md="6" :lg="6" :xl="6">
          <AStatistic
            :title="$gettext('Backend')"
            :value="stats.total_backend"
          />
        </ACol>
        <ACol :xs="12" :sm="12" :md="6" :lg="6" :xl="6">
          <AStatistic
            :title="$gettext('Frontend')"
            :value="stats.total_frontend"
          />
        </ACol>
        <ACol :xs="12" :sm="12" :md="6" :lg="6" :xl="6">
          <AStatistic
            :title="$gettext('License Types')"
            :value="Object.keys(stats.license_distribution || {}).length"
          />
        </ACol>
      </ARow>

      <ADivider />

      <h4>{{ $gettext('License Distribution') }}</h4>
      <ARow :gutter="[16, 16]">
        <ACol
          v-for="[licenseName, count] in Object.entries(stats.license_distribution || {})"
          :key="licenseName"
          :xs="24" :sm="12" :md="8" :lg="6" :xl="6"
        >
          <div class="license-item">
            <ATag :color="getLicenseColor(licenseName)">
              {{ licenseName }}
            </ATag>
            <span class="ml-2">{{ count }} {{ $gettext('components') }}</span>
          </div>
        </ACol>
      </ARow>
    </ACard>

    <ACard>
      <ATabs v-model:active-key="activeTab">
        <ATabPane key="all" :tab="$gettext('All Components')">
          <ATable
            :columns="columns"
            :data-source="getCurrentLicenses()"
            :loading="loading"
            :pagination="{ pageSize: 20, showSizeChanger: true, showQuickJumper: true }"
            :scroll="{ x: 800 }"
            size="small"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'name'">
                <ATypographyText code>
                  {{ record.name }}
                </ATypographyText>
              </template>
              <template v-else-if="column.key === 'license'">
                <ATag :color="getLicenseColor(record.license)">
                  {{ record.license }}
                </ATag>
              </template>
              <template v-else-if="column.key === 'url'">
                <AButton
                  type="link"
                  size="small"
                  :href="record.url"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  {{ $gettext('View') }}
                </AButton>
              </template>
            </template>
          </ATable>
        </ATabPane>

        <ATabPane key="backend" :tab="$gettext('Backend')">
          <ATable
            :columns="columns"
            :data-source="getCurrentLicenses()"
            :loading="loading"
            :pagination="{ pageSize: 20, showSizeChanger: true, showQuickJumper: true }"
            :scroll="{ x: 800 }"
            size="small"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'name'">
                <ATypographyText code>
                  {{ record.name }}
                </ATypographyText>
              </template>
              <template v-else-if="column.key === 'license'">
                <ATag :color="getLicenseColor(record.license)">
                  {{ record.license }}
                </ATag>
              </template>
              <template v-else-if="column.key === 'url'">
                <AButton
                  type="link"
                  size="small"
                  :href="record.url"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  {{ $gettext('View') }}
                </AButton>
              </template>
            </template>
          </ATable>
        </ATabPane>

        <ATabPane key="frontend" :tab="$gettext('Frontend')">
          <ATable
            :columns="columns"
            :data-source="getCurrentLicenses()"
            :loading="loading"
            :pagination="{ pageSize: 20, showSizeChanger: true, showQuickJumper: true }"
            :scroll="{ x: 800 }"
            size="small"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'name'">
                <ATypographyText code>
                  {{ record.name }}
                </ATypographyText>
              </template>
              <template v-else-if="column.key === 'license'">
                <ATag :color="getLicenseColor(record.license)">
                  {{ record.license }}
                </ATag>
              </template>
              <template v-else-if="column.key === 'url'">
                <AButton
                  type="link"
                  size="small"
                  :href="record.url"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  {{ $gettext('View') }}
                </AButton>
              </template>
            </template>
          </ATable>
        </ATabPane>
      </ATabs>
    </ACard>
  </div>
</template>

<style lang="less" scoped>
.license-item {
  display: flex;
  align-items: center;
  padding: 4px 0;
  flex-wrap: wrap;
  gap: 8px;

  @media (max-width: 576px) {
    flex-direction: column;
    align-items: flex-start;
  }
}

:deep(.ant-table-wrapper) {
  @media (max-width: 768px) {
    .ant-table-pagination {
      .ant-pagination-options {
        display: none;
      }
    }
  }
}

:deep(.ant-statistic) {
  text-align: center;

  @media (max-width: 768px) {
    margin-bottom: 16px;
  }
}
</style>
