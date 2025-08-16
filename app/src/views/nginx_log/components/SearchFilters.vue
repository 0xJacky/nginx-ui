<script setup lang="ts">
import type { SearchFilters } from '@/api/nginx_log'
import { CaretRightOutlined } from '@ant-design/icons-vue'
import { browserOptions, deviceOptions, methodOptions, osOptions, statusOptions } from './search-filter-options'

// Emits
interface Emits {
  (e: 'search'): void
  (e: 'reset'): void
}

const emit = defineEmits<Emits>()

// Use defineModel for simplified v-model handling
const filters = defineModel<SearchFilters>({ required: true })

// Collapse state
const collapsed = ref(true)

function handleSearch() {
  emit('search')
}

function handleReset() {
  filters.value = {
    query: '',
    ip: '',
    method: '',
    status: [],
    path: '',
    user_agent: '',
    referer: '',
    browser: [],
    os: [],
    device: [],
  }
  emit('reset')
}
</script>

<template>
  <div class="bg-gray-50 dark:bg-trueGray-800 rounded border border-gray-200 dark:border-trueGray-700">
    <!-- Header -->
    <div
      class="px-4 py-3 cursor-pointer hover:bg-gray-100 dark:hover:bg-trueGray-700 flex items-center justify-between"
      @click="collapsed = !collapsed"
    >
      <div class="flex items-center space-x-2 min-h-[1.5rem]">
        <CaretRightOutlined
          class="transition-transform text-sm flex-shrink-0 leading-6" :class="[collapsed ? '' : 'rotate-90']"
        />
        <div class="text-sm font-medium text-gray-900 dark:text-trueGray-100 leading-6">
          {{ $gettext('Search Filters') }}
        </div>
      </div>
    </div>

    <!-- Content -->
    <div v-show="!collapsed" class="p-4 space-y-4 border-t border-gray-200 dark:border-trueGray-700">
      <!-- Row 1: Basic Search -->
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-3">
        <!-- Full Text Search -->
        <div class="lg:col-span-2">
          <label class="block text-xs font-medium text-gray-700 dark:text-trueGray-300 mb-1">
            {{ $gettext('Full Text Search') }}
          </label>
          <AInput
            v-model:value="filters.query"
            :placeholder="$gettext('Search in log content...')"
            @press-enter="handleSearch"
          />
        </div>

        <!-- IP Address -->
        <div>
          <label class="block text-xs font-medium text-gray-700 dark:text-trueGray-300 mb-1">
            {{ $gettext('IP Address') }}
          </label>
          <AInput
            v-model:value="filters.ip"
            placeholder="192.168.1.1"
            @press-enter="handleSearch"
          />
        </div>
      </div>

      <!-- Row 2: Request Details -->
      <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-3">
        <!-- HTTP Method -->
        <div>
          <label class="block text-xs font-medium text-gray-700 dark:text-trueGray-300 mb-1">
            {{ $gettext('Method') }}
          </label>
          <ASelect
            v-model:value="filters.method"
            :placeholder="$gettext('Any')"
            allow-clear
            style="width: 100%"
            :options="methodOptions"
          />
        </div>

        <!-- Status Codes -->
        <div>
          <label class="block text-xs font-medium text-gray-700 dark:text-trueGray-300 mb-1">
            {{ $gettext('Status') }}
          </label>
          <ASelect
            v-model:value="filters.status"
            mode="tags"
            :placeholder="$gettext('Type or select status codes')"
            allow-clear
            style="width: 100%"
            :options="statusOptions"
            :token-separators="[',', ' ']"
          />
        </div>

        <!-- Request Path -->
        <div class="md:col-span-2">
          <label class="block text-xs font-medium text-gray-700 dark:text-trueGray-300 mb-1">
            {{ $gettext('Request Path') }}
          </label>
          <AInput
            v-model:value="filters.path"
            placeholder="/"
            @press-enter="handleSearch"
          />
        </div>
      </div>

      <!-- Row 3: Client Info -->
      <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-3">
        <!-- Browser -->
        <div>
          <label class="block text-xs font-medium text-gray-700 dark:text-trueGray-300 mb-1">
            {{ $gettext('Browser') }}
          </label>
          <ASelect
            v-model:value="filters.browser"
            mode="tags"
            :placeholder="$gettext('Type or select browser')"
            allow-clear
            style="width: 100%"
            :options="browserOptions"
            :token-separators="[',', ' ']"
          />
        </div>

        <!-- Operating System -->
        <div>
          <label class="block text-xs font-medium text-gray-700 dark:text-trueGray-300 mb-1">
            {{ $gettext('OS') }}
          </label>
          <ASelect
            v-model:value="filters.os"
            mode="tags"
            :placeholder="$gettext('Type or select OS')"
            allow-clear
            style="width: 100%"
            :options="osOptions"
            :token-separators="[',', ' ']"
          />
        </div>

        <!-- Device Type -->
        <div>
          <label class="block text-xs font-medium text-gray-700 dark:text-trueGray-300 mb-1">
            {{ $gettext('Device') }}
          </label>
          <ASelect
            v-model:value="filters.device"
            mode="tags"
            :placeholder="$gettext('Type or select device')"
            allow-clear
            style="width: 100%"
            :options="deviceOptions"
            :token-separators="[',', ' ']"
          />
        </div>

        <!-- Referer -->
        <div>
          <label class="block text-xs font-medium text-gray-700 dark:text-trueGray-300 mb-1">
            {{ $gettext('Referer') }}
          </label>
          <AInput
            v-model:value="filters.referer"
            :placeholder="$gettext('https://...')"
            @press-enter="handleSearch"
          />
        </div>
      </div>

      <!-- Row 4: Advanced -->
      <div class="grid grid-cols-1 gap-3">
        <!-- User Agent -->
        <div>
          <label class="block text-xs font-medium text-gray-700 dark:text-trueGray-300 mb-1">
            {{ $gettext('User Agent') }}
          </label>
          <AInput
            v-model:value="filters.user_agent"
            :placeholder="$gettext('Mozilla/5.0...')"
            @press-enter="handleSearch"
          />
        </div>
      </div>

      <!-- Action Buttons -->
      <div class="flex items-center pt-3 border-t border-gray-200 dark:border-trueGray-700 justify-end">
        <div class="flex space-x-2">
          <AButton @click="handleReset">
            {{ $gettext('Reset') }}
          </AButton>
          <AButton type="primary" @click="handleSearch">
            {{ $gettext('Search') }}
          </AButton>
        </div>
      </div>
    </div>
  </div>
</template>
