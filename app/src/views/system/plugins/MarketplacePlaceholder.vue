<script setup lang="ts">
import { ShopOutlined } from '@antdv-next/icons'
</script>

<template>
  <AEmpty>
    <template #image>
      <ShopOutlined class="text-5xl text-gray-400" />
    </template>
    <template #description>
      <div class="mx-auto max-w-md">
        <p class="mb-1 font-medium">
          {{ $gettext('The plugin marketplace is not available yet') }}
        </p>
        <p class="mb-0 text-gray-500">
          {{ $gettext('Until it ships, install plugins from a bundle file on the Installed tab.') }}
        </p>
      </div>
    </template>
  </AEmpty>
</template>
