<script setup lang="ts">
import type { ExternalNotifyConfig } from './types'
import { testMessage } from '@/api/external_notify'
import CodeEditor from '@/components/CodeEditor'
import { SensitiveInput } from '@/components/SensitiveString'
import gettext from '@/gettext'
import configMap from './index'

const props = defineProps<{
  type?: string
}>()

const { message } = App.useApp()

const modelValue = defineModel<Record<string, string>>({ default: () => ({}) })
const multilineConfigKeys = new Set(['html_template', 'message_body'])
const sensitiveConfigKeysByType: Record<string, Set<string>> = {
  email: new Set(['password']),
  teams: new Set(['client_secret']),
}
const teamsMessageBodyTemplate = `[
  {
    "type": "TextBlock",
    "text": "{{title}}",
    "weight": "Bolder",
    "size": "Large",
    "color": "{{level_color}}"
  },
  {
    "type": "FactSet",
    "facts": [
      {
        "title": "Type",
        "value": "{{type_label}} ({{type}} / {{type_i18n_key}})"
      },
      {
        "title": "LevelColor",
        "value": "{{level_color}}"
      },
      {
        "title": "Content",
        "value": "{{content}}"
      },
      {
        "title": "Go To",
        "value": "{{go_to_url}}"
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

const teamsPlaceholderHints = [
  '{{title}}',
  '{{content}}',
  '{{type}}',
  '{{type_label}}',
  '{{type_i18n_key}}',
  '{{level_color}}',
  '{{go_to_url}}',
]

const emailPlaceholderHints = [
  '{{.Title}}',
  '{{.Content}}',
  '{{.Type}}',
  '{{.LevelColor}}',
  '{{.TypeLabel}}',
  '{{.TypeI18nKey}}',
  '{{.GoToURL}}',
]

interface PlaceholderDoc {
  key: string
  description: string
}

const currentConfig = computed<ExternalNotifyConfig | undefined>(() => {
  return configMap[props.type?.toLowerCase() ?? '']
})

const allowedConfigKeys = computed<string[]>(() => {
  return currentConfig.value?.config.map(item => item.key) ?? []
})

const singleLineConfigItems = computed(() => {
  if (!currentConfig.value)
    return []

  return currentConfig.value.config.filter(item => !multilineConfigKeys.has(item.key))
})

function isSensitiveConfigKey(key: string): boolean {
  const type = props.type?.toLowerCase() ?? ''
  return sensitiveConfigKeysByType[type]?.has(key) ?? false
}

function resolveSensitiveConfigValue(key: string) {
  return async () => modelValue.value[key] ?? ''
}

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

const loading = ref(false)

const placeholderHintText = computed<string>(() => {
  if (props.type?.toLowerCase() === 'teams') {
    return teamsPlaceholderHints.join(', ')
  }

  if (props.type?.toLowerCase() === 'email') {
    return emailPlaceholderHints.join(', ')
  }

  return ''
})

const placeholderDocs = computed<PlaceholderDoc[]>(() => {
  if (props.type?.toLowerCase() === 'teams') {
    return [
      {
        key: '{{title}}',
        description: $gettext('Notification title text'),
      },
      {
        key: '{{content}}',
        description: $gettext('Notification content text'),
      },
      {
        key: '{{type}}',
        description: $gettext('Notification type code, such as error, warning, info, or success'),
      },
      {
        key: '{{type_label}}',
        description: $gettext('Localized notification type label for the current language'),
      },
      {
        key: '{{type_i18n_key}}',
        description: $gettext('Translation key of the notification type, such as Error or Warning'),
      },
      {
        key: '{{level_color}}',
        description: $gettext('Teams notification level color enum: Error=Attention, Warning=Warning, Info=Accent, Success=Good'),
      },
      {
        key: '{{go_to_url}}',
        description: $gettext('Full go-to URL generated from WebAuthn RPOrigins and route path'),
      },
    ]
  }

  if (props.type?.toLowerCase() === 'email') {
    return [
      {
        key: '{{.Title}}',
        description: $gettext('Notification title text'),
      },
      {
        key: '{{.Content}}',
        description: $gettext('Notification content text (HTML-escaped in the default template)'),
      },
      {
        key: '{{.Type}}',
        description: $gettext('Notification type code, such as error, warning, info, or success'),
      },
      {
        key: '{{.LevelColor}}',
        description: $gettext('Email notification level color, fixed to #1f2937'),
      },
      {
        key: '{{.TypeLabel}}',
        description: $gettext('Localized notification type label for the current language'),
      },
      {
        key: '{{.TypeI18nKey}}',
        description: $gettext('Translation key of the notification type, such as Error or Warning'),
      },
      {
        key: '{{.GoToURL}}',
        description: $gettext('Full go-to URL generated from WebAuthn RPOrigins and route path'),
      },
    ]
  }

  return []
})

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
    <AFormItem
      v-for="item in singleLineConfigItems"
      :key="item.key"
      :label="$gettext(item.label)"
    >
      <SensitiveInput
        v-if="isSensitiveConfigKey(item.key)"
        v-model="modelValue[item.key]"
        :resolve="resolveSensitiveConfigValue(item.key)"
      />
      <AInput
        v-else
        v-model:value="modelValue[item.key]"
      />
    </AFormItem>

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
          [{{ $gettext('Teams Request Example') }}]
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
        v-if="item.key === 'message_body' && placeholderHintText"
        class="mt-2"
        type="info"
        show-icon
        :title="$gettext('Available placeholders')"
      >
        <template #description>
          <div
            v-for="doc in placeholderDocs"
            :key="`msg-${doc.key}`"
            class="mt-1"
          >
            <strong>{{ doc.key }}</strong>: {{ doc.description }}
          </div>
        </template>
      </AAlert>
      <AAlert
        v-if="item.key === 'message_body' && messageBodyJsonError"
        class="mt-2"
        type="error"
        show-icon
        :title="messageBodyJsonErrorText"
      />
      <AAlert
        v-if="item.key === 'html_template' && placeholderHintText"
        class="mt-2"
        type="info"
        show-icon
        :title="$gettext('Available placeholders')"
      >
        <template #description>
          <div
            v-for="doc in placeholderDocs"
            :key="`html-${doc.key}`"
            class="mt-1"
          >
            <strong>{{ doc.key }}</strong>: {{ doc.description }}
          </div>
        </template>
      </AAlert>
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
