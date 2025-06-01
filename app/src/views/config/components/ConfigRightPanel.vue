<script setup lang="ts">
import type { Config } from '@/api/config'
import ConfigRightPanel from './ConfigRightPanel/ConfigRightPanel.vue'

interface ConfigRightPanelProps {
  addMode: boolean
  newPath: string
  modifiedAt: string
  origName: string
}

const props = defineProps<ConfigRightPanelProps>()
const data = defineModel<Config>('data', { required: true })
</script>

<template>
  <ConfigRightPanel
    v-model:data="data"
    :add-mode="props.addMode"
    :new-path="props.newPath"
    :modified-at="props.modifiedAt"
    :orig-name="props.origName"
  />
</template>
