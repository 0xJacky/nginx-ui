<script setup lang="ts">
import type { ProxyCacheConfig } from '@/api/ngx'

const value = defineModel<ProxyCacheConfig>({
  default: reactive({
    enabled: false,
    path: '/var/cache/nginx/proxy_cache',
    levels: '1:2',
    use_temp_path: 'off',
    keys_zone: 'proxy_cache:10m',
    inactive: '60m',
    max_size: '1g',
    min_free: '',
    manager_files: '',
    manager_sleep: '',
    manager_threshold: '',
    loader_files: '',
    loader_sleep: '',
    loader_threshold: '',
    purger: 'off',
    purger_files: '',
    purger_sleep: '',
    purger_threshold: '',
  }),
})

const timeUnitOptions = [
  { value: 'ms', label: 'ms' },
  { value: 's', label: 's' },
  { value: 'm', label: 'm' },
  { value: 'h', label: 'h' },
  { value: 'd', label: 'd' },
  { value: 'w', label: 'w' },
  { value: 'M', label: 'M' },
  { value: 'y', label: 'y' },
]

const sizeUnitOptions = [
  { value: 'k', label: 'K' },
  { value: 'm', label: 'M' },
  { value: 'g', label: 'G' },
]

const timeValues = reactive({
  inactive: { value: '60', unit: 'm' },
  manager_sleep: { value: '', unit: 'ms' },
  manager_threshold: { value: '', unit: 'ms' },
  loader_sleep: { value: '', unit: 'ms' },
  loader_threshold: { value: '', unit: 'ms' },
  purger_sleep: { value: '', unit: 'ms' },
  purger_threshold: { value: '', unit: 'ms' },
})

const sizeValues = reactive({
  max_size: { value: '1', unit: 'g' },
  min_free: { value: '', unit: 'm' },
})

function initTimeValues() {
  const timeFields = ['inactive', 'manager_sleep', 'manager_threshold', 'loader_sleep', 'loader_threshold', 'purger_sleep', 'purger_threshold']

  timeFields.forEach(field => {
    const fieldValue = value.value[field]
    if (fieldValue) {
      const match = fieldValue.match(/^(\d+)([a-z]+)$/i)
      if (match) {
        timeValues[field].value = match[1]
        timeValues[field].unit = match[2]
      }
    }
  })
}

function initSizeValues() {
  const sizeFields = ['max_size', 'min_free']

  sizeFields.forEach(field => {
    const fieldValue = value.value[field]
    if (fieldValue) {
      const match = fieldValue.match(/^(\d+)([kmg])$/i)
      if (match) {
        sizeValues[field].value = match[1]
        sizeValues[field].unit = match[2].toLowerCase()
      }
    }
  })
}

function updateTimeValue(field) {
  if (timeValues[field].value) {
    value.value[field] = `${timeValues[field].value}${timeValues[field].unit}`
  }
  else {
    value.value[field] = ''
  }
}

function updateSizeValue(field) {
  if (sizeValues[field].value) {
    value.value[field] = `${sizeValues[field].value}${sizeValues[field].unit}`
  }
  else {
    value.value[field] = ''
  }
}

initTimeValues()
initSizeValues()
</script>

<template>
  <AForm layout="vertical">
    <AFormItem :label="$gettext('Enable Proxy Cache')" name="enabled">
      <ASwitch v-model:checked="value.enabled" />
    </AFormItem>

    <div v-if="value.enabled" class="pt-4">
      <ADivider>{{ $gettext('Basic Settings') }}</ADivider>

      <ARow :gutter="16">
        <ACol :span="12">
          <AFormItem
            :label="$gettext('Cache Path')"
            name="path"
            required
            :help="$gettext('Directory path to store cache files')"
          >
            <AInput v-model:value="value.path" placeholder="/var/cache/nginx/proxy_cache" />
          </AFormItem>
        </ACol>

        <ACol :span="12">
          <AFormItem
            :label="$gettext('Directory Levels')"
            name="levels"
            :help="$gettext('Cache subdirectory levels structure, e.g. 1:2')"
          >
            <AInput v-model:value="value.levels" placeholder="1:2" />
          </AFormItem>
        </ACol>
      </ARow>

      <ARow :gutter="16">
        <ACol :span="12">
          <AFormItem
            :label="$gettext('Shared Memory Zone')"
            name="keys_zone"
            required
            :help="$gettext('Define shared memory zone name and size, e.g. proxy_cache:10m')"
          >
            <AInput v-model:value="value.keys_zone" placeholder="proxy_cache:10m" />
          </AFormItem>
        </ACol>

        <ACol :span="12">
          <AFormItem
            :label="$gettext('Use Temporary Path')"
            name="use_temp_path"
            :help="$gettext('Whether to use a temporary path when writing temporary files')"
          >
            <ASwitch
              v-model:checked="value.use_temp_path"
              :checked-children="$gettext('On')"
              :un-checked-children="$gettext('Off')"
              checked-value="on"
              un-checked-value="off"
            />
          </AFormItem>
        </ACol>
      </ARow>

      <ARow :gutter="16">
        <ACol :span="12">
          <AFormItem
            :label="$gettext('Inactive Time')"
            name="inactive"
            :help="$gettext('Cache items not accessed within this time will be removed')"
          >
            <AInputGroup compact>
              <AInput
                v-model:value="timeValues.inactive.value"
                style="width: 65%"
                placeholder="60"
                @change="updateTimeValue('inactive')"
              />
              <ASelect
                v-model:value="timeValues.inactive.unit"
                style="width: 35%"
                :options="timeUnitOptions"
                @change="updateTimeValue('inactive')"
              />
            </AInputGroup>
          </AFormItem>
        </ACol>

        <ACol :span="12">
          <AFormItem
            :label="$gettext('Maximum Cache Size')"
            name="max_size"
            :help="$gettext('Maximum total size of the cache')"
          >
            <AInputGroup compact>
              <AInput
                v-model:value="sizeValues.max_size.value"
                style="width: 65%"
                placeholder="1"
                @change="updateSizeValue('max_size')"
              />
              <ASelect
                v-model:value="sizeValues.max_size.unit"
                style="width: 35%"
                :options="sizeUnitOptions"
                @change="updateSizeValue('max_size')"
              />
            </AInputGroup>
          </AFormItem>
        </ACol>
      </ARow>

      <ARow :gutter="16">
        <ACol :span="12">
          <AFormItem
            :label="$gettext('Minimum Free Space')"
            name="min_free"
            :help="$gettext('Minimum free space in the cache directory')"
          >
            <AInputGroup compact>
              <AInput
                v-model:value="sizeValues.min_free.value"
                style="width: 65%"
                placeholder="100"
                @change="updateSizeValue('min_free')"
              />
              <ASelect
                v-model:value="sizeValues.min_free.unit"
                style="width: 35%"
                :options="sizeUnitOptions"
                @change="updateSizeValue('min_free')"
              />
            </AInputGroup>
          </AFormItem>
        </ACol>
      </ARow>

      <ADivider>{{ $gettext('Cache Manager Settings') }}</ADivider>

      <ARow :gutter="16">
        <ACol :span="12">
          <AFormItem
            :label="$gettext('Manager Files')"
            name="manager_files"
            :help="$gettext('Number of files processed by cache manager at once')"
          >
            <AInput v-model:value="value.manager_files" placeholder="e.g. 100" />
          </AFormItem>
        </ACol>

        <ACol :span="12">
          <AFormItem
            :label="$gettext('Manager Sleep')"
            name="manager_sleep"
            :help="$gettext('Sleep time between cache manager iterations')"
          >
            <AInputGroup compact>
              <AInput
                v-model:value="timeValues.manager_sleep.value"
                style="width: 65%"
                placeholder="50"
                @change="updateTimeValue('manager_sleep')"
              />
              <ASelect
                v-model:value="timeValues.manager_sleep.unit"
                style="width: 35%"
                :options="timeUnitOptions"
                @change="updateTimeValue('manager_sleep')"
              />
            </AInputGroup>
          </AFormItem>
        </ACol>
      </ARow>

      <ARow :gutter="16">
        <ACol :span="12">
          <AFormItem
            :label="$gettext('Manager Threshold')"
            name="manager_threshold"
            :help="$gettext('Cache manager processing time threshold')"
          >
            <AInputGroup compact>
              <AInput
                v-model:value="timeValues.manager_threshold.value"
                style="width: 65%"
                placeholder="200"
                @change="updateTimeValue('manager_threshold')"
              />
              <ASelect
                v-model:value="timeValues.manager_threshold.unit"
                style="width: 35%"
                :options="timeUnitOptions"
                @change="updateTimeValue('manager_threshold')"
              />
            </AInputGroup>
          </AFormItem>
        </ACol>
      </ARow>

      <ADivider>{{ $gettext('Loader Settings') }}</ADivider>

      <ARow :gutter="16">
        <ACol :span="12">
          <AFormItem
            :label="$gettext('Loader Files')"
            name="loader_files"
            :help="$gettext('Number of files processed by cache loader at once')"
          >
            <AInput v-model:value="value.loader_files" placeholder="e.g. 100" />
          </AFormItem>
        </ACol>

        <ACol :span="12">
          <AFormItem
            :label="$gettext('Loader Sleep')"
            name="loader_sleep"
            :help="$gettext('Sleep time between cache loader iterations')"
          >
            <AInputGroup compact>
              <AInput
                v-model:value="timeValues.loader_sleep.value"
                style="width: 65%"
                placeholder="50"
                @change="updateTimeValue('loader_sleep')"
              />
              <ASelect
                v-model:value="timeValues.loader_sleep.unit"
                style="width: 35%"
                :options="timeUnitOptions"
                @change="updateTimeValue('loader_sleep')"
              />
            </AInputGroup>
          </AFormItem>
        </ACol>
      </ARow>

      <ARow :gutter="16">
        <ACol :span="12">
          <AFormItem
            :label="$gettext('Loader Threshold')"
            name="loader_threshold"
            :help="$gettext('Cache loader processing time threshold')"
          >
            <AInputGroup compact>
              <AInput
                v-model:value="timeValues.loader_threshold.value"
                style="width: 65%"
                placeholder="200"
                @change="updateTimeValue('loader_threshold')"
              />
              <ASelect
                v-model:value="timeValues.loader_threshold.unit"
                style="width: 35%"
                :options="timeUnitOptions"
                @change="updateTimeValue('loader_threshold')"
              />
            </AInputGroup>
          </AFormItem>
        </ACol>
      </ARow>

      <!-- <ADivider>{{ $gettext('Purger Settings') }}</ADivider>

      <ARow :gutter="16">
        <ACol :span="12">
          <AFormItem
            :label="$gettext('Enable Purger')"
            name="purger"
            :help="$gettext('Whether to enable the cache purger')"
          >
            <ASwitch
              v-model:checked="value.purger"
              :checked-children="$gettext('On')"
              :un-checked-children="$gettext('Off')"
              checked-value="on"
              un-checked-value="off"
            />
          </AFormItem>
        </ACol>

        <ACol :span="12">
          <AFormItem
            :label="$gettext('Purger Files')"
            name="purger_files"
            :help="$gettext('Number of files processed by purger at once')"
          >
            <AInput v-model:value="value.purger_files" placeholder="e.g. 10" />
          </AFormItem>
        </ACol>
      </ARow>

      <ARow :gutter="16">
        <ACol :span="12">
          <AFormItem
            :label="$gettext('Purger Sleep')"
            name="purger_sleep"
            :help="$gettext('Sleep time between purger iterations')"
          >
            <AInputGroup compact>
              <AInput
                v-model:value="timeValues.purger_sleep.value"
                style="width: 65%"
                placeholder="50"
                @change="updateTimeValue('purger_sleep')"
              />
              <ASelect
                v-model:value="timeValues.purger_sleep.unit"
                style="width: 35%"
                :options="timeUnitOptions"
                @change="updateTimeValue('purger_sleep')"
              />
            </AInputGroup>
          </AFormItem>
        </ACol>

        <ACol :span="12">
          <AFormItem
            :label="$gettext('Purger Threshold')"
            name="purger_threshold"
            :help="$gettext('Purger processing time threshold')"
          >
            <AInputGroup compact>
              <AInput
                v-model:value="timeValues.purger_threshold.value"
                style="width: 65%"
                placeholder="200"
                @change="updateTimeValue('purger_threshold')"
              />
              <ASelect
                v-model:value="timeValues.purger_threshold.unit"
                style="width: 35%"
                :options="timeUnitOptions"
                @change="updateTimeValue('purger_threshold')"
              />
            </AInputGroup>
          </AFormItem>
        </ACol>
      </ARow> -->
    </div>
  </AForm>
</template>
