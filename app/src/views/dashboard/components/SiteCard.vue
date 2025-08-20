<script setup lang="ts">
import type { SiteInfo } from '@/api/site_navigation'
import {
  ClockCircleOutlined,
  CodeOutlined,
  ExclamationCircleOutlined,
  SettingOutlined,
} from '@ant-design/icons-vue'
import { truncate, upperFirst } from 'lodash'
import { SiteStatus } from '@/constants/site-status'

interface Props {
  site: SiteInfo
  settingsMode: boolean
}

interface Emits {
  (e: 'openConfig', site: SiteInfo): void
}

defineProps<Props>()
defineEmits<Emits>()

// Check if site can be opened (only HTTP/HTTPS)
function canOpenSite(site: SiteInfo): boolean {
  const scheme = site.scheme || site.health_check_protocol || 'http'
  return scheme === 'http' || scheme === 'https'
}

// Open site in new tab (only for HTTP/HTTPS)
function openSite(site: SiteInfo) {
  if (!canOpenSite(site)) {
    return
  }

  // Use display_url if available, otherwise construct from scheme and host_port
  let targetUrl = site.display_url || site.url

  // If we have scheme and host_port, construct the URL
  if (site.scheme && site.host_port && (site.scheme === 'http' || site.scheme === 'https')) {
    targetUrl = `${site.scheme}://${site.host_port}`
  }

  window.open(targetUrl, '_blank')
}

// Handle favicon loading error
function handleFaviconError(event: Event) {
  const img = event.target as HTMLImageElement
  img.style.display = 'none'
}

// Get avatar color based on site name
function getAvatarColor(name: string): string {
  const colors = [
    '#f87171',
    '#fb923c',
    '#facc15',
    '#a3e635',
    '#4ade80',
    '#22d3ee',
    '#60a5fa',
    '#a78bfa',
    '#f472b6',
    '#fb7185',
  ]

  let hash = 0
  for (let i = 0; i < name.length; i++) {
    hash = name.charCodeAt(i) + ((hash << 5) - hash)
  }

  return colors[Math.abs(hash) % colors.length]
}

// Get initials from site name
function getInitials(name: string): string {
  const parts = name.split('.')
  return truncate(
    parts
      .map(part => upperFirst(part.charAt(0)))
      .join(''),
    { length: 2, omission: '' },
  )
}

// Get status CSS class
function getStatusClass(status: string): string {
  switch (status) {
    case SiteStatus.ONLINE:
      return 'status-online'
    case SiteStatus.OFFLINE:
      return 'status-offline'
    case SiteStatus.ERROR:
      return 'status-error'
    case SiteStatus.CHECKING:
      return 'status-checking'
    default:
      return 'status-unknown'
  }
}
</script>

<template>
  <div
    class="site-card"
    :class="{
      'settings-mode': settingsMode,
      'clickable': !settingsMode && canOpenSite(site),
      'non-clickable': !settingsMode && !canOpenSite(site),
    }"
    :data-url="site.url"
    @click="!settingsMode && canOpenSite(site) && openSite(site)"
  >
    <div class="site-card-header">
      <div class="site-icon">
        <img
          v-if="site.favicon_data"
          :src="site.favicon_data"
          :alt="site.name"
          class="w-8 h-8 rounded"
          @error="handleFaviconError"
        >
        <div
          v-else
          class="avatar-fallback"
          :style="{ backgroundColor: getAvatarColor(site.name) }"
        >
          {{ getInitials(site.name) }}
        </div>
      </div>

      <div v-if="!settingsMode" class="site-status">
        <div
          class="status-indicator"
          :class="getStatusClass(site.status)"
        />
      </div>
    </div>

    <div class="site-info">
      <h3 class="site-title">
        {{ site.title || site.name }}
      </h3>
      <p class="site-url">
        <span v-if="site.scheme && site.host_port" class="url-parts">
          <span class="scheme">{{ site.scheme }}://</span><span class="host-port">{{ site.host_port }}</span>
        </span>
        <span v-else>{{ site.display_url || site.url }}</span>
      </p>

      <div class="site-details">
        <div v-if="site.status === SiteStatus.ONLINE" class="detail-item">
          <ClockCircleOutlined class="detail-icon" />
          <span>{{ site.response_time }}ms</span>
        </div>
        <div v-if="site.status_code" class="detail-item">
          <CodeOutlined class="detail-icon" />
          <span>{{ site.status_code }}</span>
        </div>
        <div v-if="site.error" class="detail-item error">
          <ExclamationCircleOutlined class="detail-icon" />
          <span>{{ site.error }}</span>
        </div>
      </div>
    </div>

    <!-- Settings button in settings mode -->
    <div v-if="settingsMode" class="site-card-config">
      <AButton
        type="text"
        size="small"
        @click.stop="$emit('openConfig', site)"
      >
        <template #icon>
          <SettingOutlined />
        </template>
      </AButton>
    </div>

    <!-- Drag handle in settings mode -->
    <div v-if="settingsMode" class="drag-handle">
      <div class="drag-dots">
        <div class="dot" />
        <div class="dot" />
        <div class="dot" />
        <div class="dot" />
      </div>
    </div>
  </div>
</template>

<style scoped>
.site-card {
  @apply relative bg-white dark:bg-trueGray-900 rounded-lg shadow-sm border border-gray-200 dark:border-gray-700 p-4 transition-all duration-200;
}

.site-card.clickable {
  @apply cursor-pointer hover:scale-105;
}

.site-card.non-clickable {
  @apply cursor-default;
  opacity: 0.8;
}

.site-card.settings-mode {
  @apply cursor-move;
}

.site-card.settings-mode:hover {
  @apply scale-100;
}

.site-card-header {
  @apply flex items-center justify-between mb-3;
}

.site-icon img {
  @apply w-8 h-8 rounded object-cover;
}

.avatar-fallback {
  @apply w-8 h-8 rounded flex items-center justify-center text-white font-medium text-sm;
}

.site-status {
  @apply flex items-center;
}

.status-indicator {
  @apply w-3 h-3 rounded-full;
}

.status-online {
  @apply bg-green-500;
}

.status-offline {
  @apply bg-red-500;
}

.status-error {
  @apply bg-yellow-500;
}

.status-checking {
  @apply bg-blue-500 animate-pulse;
}

.status-unknown {
  @apply bg-gray-400;
}

.site-info {
  @apply space-y-2;
}

.site-title {
  @apply font-medium text-gray-900 dark:text-gray-100 text-lg truncate;
}

.scheme {
  @apply text-sm text-gray-600 dark:text-gray-400;
}

.site-url {
  @apply text-sm text-gray-600 dark:text-gray-400 truncate;
}

.url-parts {
  @apply inline;
}

.host-port {
  @apply text-gray-700 dark:text-gray-300;
}

.site-details {
  @apply flex flex-wrap gap-3 text-xs;
}

.detail-item {
  @apply flex items-center gap-1 text-gray-600 dark:text-gray-400;
}

.detail-item.error {
  @apply text-red-600 dark:text-red-400;
}

.detail-icon {
  @apply w-3 h-3;
}

.site-card-config {
  @apply absolute top-2 right-2;
}

.drag-handle {
  @apply absolute bottom-2 right-2 opacity-50 hover:opacity-100 transition-opacity;
}

.drag-dots {
  @apply grid grid-cols-2 gap-1 p-1;
}

.dot {
  @apply w-1 h-1 bg-gray-400 rounded-full;
}

/* Sortable states */
.site-card-ghost {
  @apply opacity-50;
}

.site-card-chosen {
  @apply transform scale-105;
}

.site-card-drag {
  @apply transform rotate-2;
}
</style>
