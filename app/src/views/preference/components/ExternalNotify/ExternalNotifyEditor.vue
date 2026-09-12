<script setup lang="ts">
import type { StdTableColumn } from '@uozi-admin/curd'
import type { ExternalNotifyConfig } from './types'
import { StdForm } from '@uozi-admin/curd'
import { testMessage } from '@/api/external_notify'
import CodeEditor from '@/components/CodeEditor'
import gettext from '@/gettext'
import configMap from './index'

const props = defineProps<{
  type?: string
}>()

const { message } = App.useApp()

const modelValue = defineModel<Record<string, string>>({ default: () => ({}) })
const multilineConfigKeys = new Set(['html_template', 'message_body'])
const teamsMessageBodyTemplate = `[
  {
    "type": "TextBlock",
    "text": "{{title}}",
    "weight": "Bolder",
    "size": "Large",
    "color": "red"
  },
  {
    "type": "FactSet",
    "facts": [
      {
        "title": "Content",
        "value": "{{content}}"
      }
    ]
  },
  {
    "type": "TextBlock",
    "text": "Color keywords: red/green/yellow/blue",
    "wrap": true,
    "color": "blue"
  }
]
`

const currentConfig = computed<ExternalNotifyConfig | undefined>(() => {
  return configMap[props.type?.toLowerCase() ?? '']
})

const allowedConfigKeys = computed<string[]>(() => {
  return currentConfig.value?.config.map(item => item.key) ?? []
})

interface JsonValidationError {
  line: number
  column: number
  reason: string
}

function parseJsonPositionError(message: string): number | undefined {
  const match = /position\s+(\d+)/i.exec(message)
  if (!match)
    return undefined

  const position = Number.parseInt(match[1], 10)
  if (Number.isNaN(position))
    return undefined

  return position
}

function positionToLineColumn(text: string, position: number): { line: number, column: number } {
  const safePosition = Math.max(0, Math.min(position, text.length))
  let line = 1
  let column = 1

  for (let index = 0; index < safePosition; index++) {
    if (text[index] === '\n') {
      line++
      column = 1
      continue
    }
    column++
  }

  return { line, column }
}

const messageBodyJsonError = computed<JsonValidationError | undefined>(() => {
  if (!allowedConfigKeys.value.includes('message_body'))
    return undefined

  const raw = modelValue.value.message_body?.trim()
  if (!raw)
    return undefined

  try {
    JSON.parse(raw)
    return undefined
  }
  catch (error) {
    const reason = error instanceof Error ? error.message : String(error)
    const position = parseJsonPositionError(reason)

    if (position === undefined) {
      return { line: 1, column: 1, reason }
    }

    const { line, column } = positionToLineColumn(raw, position)
    return { line, column, reason }
  }
})

const messageBodyJsonErrorText = computed<string>(() => {
  if (!messageBodyJsonError.value)
    return ''

  return `${$gettext('Invalid JSON at line')} ${messageBodyJsonError.value.line}, ${$gettext('column')} ${messageBodyJsonError.value.column}`
})

const sanitizedConfig = computed<Record<string, string>>(() => {
  const allowed = new Set(allowedConfigKeys.value)
  return Object.fromEntries(
    Object.entries(modelValue.value).filter(([key]) => allowed.has(key)),
  )
})

watch([() => props.type, allowedConfigKeys], () => {
  if (!currentConfig.value)
    return

  const nextConfig = sanitizedConfig.value
  if (allowedConfigKeys.value.includes('message_body') && !nextConfig.message_body?.trim()) {
    nextConfig.message_body = teamsMessageBodyTemplate
  }
  if (JSON.stringify(nextConfig) !== JSON.stringify(modelValue.value)) {
    modelValue.value = nextConfig
  }
}, { immediate: true })

const columns = computed<StdTableColumn[]>(() => {
  if (!currentConfig.value)
    return []

  return currentConfig.value.config.filter(item => !multilineConfigKeys.has(item.key)).map(item => ({
    title: $gettext(item.label),
    dataIndex: item.key,
    key: item.key,
    edit: {
      type: 'input',
      formItem: {
        label: $gettext(item.label),
      },
    },
  }))
})

const loading = ref(false)

async function handleSendTestMessage() {
  if (!props.type) {
    message.error($gettext('Please select a notification type'))
    return
  }

  if (messageBodyJsonError.value) {
    message.error(messageBodyJsonErrorText.value)
    return
  }

  loading.value = true
  try {
    await testMessage({
      type: props.type,
      language: gettext.current,
      config: sanitizedConfig.value,
    })
    message.success($gettext('Test message sent successfully'))
  }
  catch (error) {
    console.error('Test message error:', error)
  }
  finally {
    loading.value = false
  }
}
</script>

<template>
  <div v-if="currentConfig">
    <StdForm
      v-model:data="modelValue"
      :columns
    />

    <AFormItem
      v-for="item in currentConfig.config.filter(item => multilineConfigKeys.has(item.key))"
      :key="item.key"
    >
      <template #label>
        <span>{{ $gettext(item.label) }}</span>
        <a
          v-if="item.key === 'message_body'"
          href="https://learn.microsoft.com/adaptive-cards/"
          target="_blank"
          rel="noopener noreferrer"
          class="ml-1"
        >
          [{{ $gettext('Microsoft Docs') }}]
        </a>
        <a
          v-if="item.key === 'message_body'"
          href="https://learn.microsoft.com/zh-cn/connectors/teams/?tabs=text1%2Cdotnet#example-of-sending-requests"
          target="_blank"
          rel="noopener noreferrer"
          class="ml-2"
        >
          [Teams Request Example]
        </a>
      </template>
      <ATextarea
        v-if="item.key !== 'message_body'"
        v-model:value="modelValue[item.key]"
        :rows="12"
      />
      <CodeEditor
        v-else
        v-model:content="modelValue[item.key]"
        lang="json"
        default-height="280px"
        :disable-code-completion="true"
      />
      <AAlert
        v-if="item.key === 'message_body' && messageBodyJsonError"
        class="mt-2"
        type="error"
        show-icon
        :message="messageBodyJsonErrorText"
      />
    </AFormItem>

    <div>
      <AButton
        type="primary"
        size="small"
        :loading="loading"
        @click="handleSendTestMessage"
      >
        {{ $gettext("Send test message") }}
      </AButton>
    </div>
  </div>
</template>

<style scoped lang="less">

</style>
