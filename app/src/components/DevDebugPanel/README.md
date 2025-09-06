# DevDebugPanel Component

A development-only debug panel component that provides a floating debug interface for troubleshooting and development purposes.

## Features

- **Development Only**: Automatically disabled in production builds
- **Floating Panel**: Positioned as a fixed overlay with toggle functionality
- **Customizable Content**: Uses slots for flexible debug information display
- **Modern Styling**: Dark theme with backdrop blur and professional appearance

## Usage

```vue
<script setup lang="ts">
import { DevDebugPanel } from '@/components/DevDebugPanel'

const debugData = computed(() => ({
  status: 'active',
  // ... other debug properties
}))
</script>

<template>
  <div>
    <!-- Your main content -->

    <!-- Debug Panel (only renders in development) -->
    <DevDebugPanel :debug-data="debugData">
      <template #default="{ debugData: slotDebugData }">
        <div class="debug-item">
          <span class="debug-label">Status:</span>
          <span class="debug-value">{{ (slotDebugData as any).status }}</span>
        </div>
        <div class="debug-item">
          <span class="debug-label">Quick Actions:</span>
          <div class="mt-2">
            <AButton size="small" @click="someAction">
              Action
            </AButton>
          </div>
        </div>
      </template>
    </DevDebugPanel>
  </div>
</template>
```

## Props

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `title` | `string` | `'Debug Panel'` | Panel title (currently not used in display) |
| `initialVisible` | `boolean` | `false` | Whether the panel starts visible |
| `debugData` | `Record<string, unknown>` | `{}` | Debug data passed to slot |

## Styling Classes

The component provides several CSS classes for styling debug content:

- `.debug-item` - Container for debug information rows
- `.debug-label` - Styling for debug labels
- `.debug-value` - Styling for debug values

## Security

- Component automatically detects and only renders in development environment
- Console warning is displayed if component is used in production
- No debug information is exposed in production builds

## Example Implementations

### Login Page Debug
Shows loading state, 2FA status, and route information with quick action buttons.

### Log List Debug
Displays indexing status, processing information, and table metadata with modal controls.

## Notes

- The component uses a fixed position overlay that appears in the top-right corner
- Toggle button allows showing/hiding the debug panel
- Uses Ant Design components for consistent styling
- Fully supports dark/light theme switching
