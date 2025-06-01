<script setup lang="ts">
import type { Config } from '@/api/config'
import { formatDateTime } from '@/lib/helper'
import { useSettingsStore } from '@/pinia'
import ConfigName from '@/views/config/components/ConfigName.vue'
import Deploy from './Deploy.vue'

interface BasicProps {
  addMode: boolean
  newPath: string
  modifiedAt: string
  origName: string
}

const props = defineProps<BasicProps>()
const data = defineModel<Config>('data', { required: true })
const settings = useSettingsStore()
</script>

<template>
  <div class="px-6">
    <AForm
      layout="vertical"
      :model="data"
      :rules="{
        name: [
          { required: true, message: $gettext('Please input a filename') },
          { pattern: /^[^\\/]+$/, message: $gettext('Invalid filename') },
        ],
      }"
    >
      <AFormItem
        name="name"
        :label="$gettext('Name')"
      >
        <AInput v-if="props.addMode" v-model:value="data.name" />
        <ConfigName v-else :name="data.name" :dir="data.dir" />
      </AFormItem>
      <AFormItem
        v-if="!props.addMode"
        :label="$gettext('Path')"
      >
        {{ decodeURIComponent(data.filepath) }}
      </AFormItem>
      <AFormItem
        v-show="data.name !== props.origName"
        :label="props.addMode ? $gettext('New Path') : $gettext('Changed Path')"
        required
      >
        {{ decodeURIComponent(props.newPath) }}
      </AFormItem>
      <AFormItem
        v-if="!props.addMode"
        :label="$gettext('Updated at')"
      >
        {{ formatDateTime(props.modifiedAt) }}
      </AFormItem>
      <AFormItem
        v-if="!settings.is_remote"
        :label="$gettext('Deploy')"
      >
        <Deploy v-model:data="data" />
      </AFormItem>
    </AForm>
  </div>
</template>
