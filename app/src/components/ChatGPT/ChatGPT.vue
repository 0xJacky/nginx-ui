<script setup lang="ts">
import type { ChatComplicationMessage } from '@/api/openai'
import openai from '@/api/openai'
import ChatGPT_logo from '@/assets/svg/ChatGPT_logo.svg?component'
import { urlJoin } from '@/lib/helper'
import { useSettingsStore, useUserStore } from '@/pinia'
import Icon, { SendOutlined } from '@ant-design/icons-vue'
import hljs from 'highlight.js'
import nginx from 'highlight.js/lib/languages/nginx'

import { Marked } from 'marked'
import { markedHighlight } from 'marked-highlight'
import { storeToRefs } from 'pinia'
import 'highlight.js/styles/vs2015.css'

const props = defineProps<{
  content: string
  path?: string
}>()

hljs.registerLanguage('nginx', nginx)

const { language: current } = storeToRefs(useSettingsStore())

const messages = defineModel<ChatComplicationMessage[]>('historyMessages', {
  type: Array,
  default: reactive([]),
})

const loading = ref(false)
const askBuffer = ref('')

async function request() {
  loading.value = true

  const t = ref({
    role: 'assistant',
    content: '',
  })

  const user = useUserStore()

  const { token } = storeToRefs(user)

  messages.value = [...messages.value!, t.value]

  await nextTick()

  scrollToBottom()

  const res = await fetch(urlJoin(window.location.pathname, '/api/chatgpt'), {
    method: 'POST',
    headers: { Accept: 'text/event-stream', Authorization: token.value },
    body: JSON.stringify({ filepath: props.path, messages: messages.value?.slice(0, messages.value?.length - 1) }),
  })

  const reader = res.body!.getReader()

  let buffer = ''

  let hasCodeBlockIndicator = false

  while (true) {
    try {
      const { done, value } = await reader.read()
      if (done) {
        setTimeout(() => {
          scrollToBottom()
        }, 500)
        loading.value = false
        storeRecord()
        break
      }
      apply(value!)
    }
    catch {
      break
    }
  }

  function apply(input: Uint8Array) {
    const decoder = new TextDecoder('utf-8')
    const raw = decoder.decode(input)

    // console.log(input, raw)

    const line = raw.split('\n\n')

    line?.forEach(v => {
      const data = v.slice('event:message\ndata:'.length)
      if (!data)
        return

      const content = JSON.parse(data).content

      if (!hasCodeBlockIndicator)
        hasCodeBlockIndicator = content.includes('`')

      for (const c of content) {
        buffer += c
        if (hasCodeBlockIndicator) {
          if (isCodeBlockComplete(buffer)) {
            t.value.content = buffer
            hasCodeBlockIndicator = false
          }
          else {
            t.value.content = `${buffer}\n\`\`\``
          }
        }
        else {
          t.value.content = buffer
        }
      }

      // keep container scroll to bottom
      scrollToBottom()
    })
  }

  function isCodeBlockComplete(text: string) {
    const codeBlockRegex = /```/g
    const matches = text.match(codeBlockRegex)
    if (matches)
      return matches.length % 2 === 0
    else
      return true
  }

  function scrollToBottom() {
    const container = document.querySelector('.right-settings .ant-card-body')
    if (container)
      container.scrollTop = container.scrollHeight
  }
}

async function send() {
  if (!messages.value)
    messages.value = []

  if (messages.value.length === 0) {
    messages.value = [{
      role: 'user',
      content: `${props.content}\n\nCurrent Language Code: ${current.value}`,
    }]
  }
  else {
    messages.value = [...messages.value, {
      role: 'user',
      content: askBuffer.value,
    }]
    askBuffer.value = ''
  }

  await nextTick()

  await request()
}

const marked = new Marked(
  markedHighlight({
    langPrefix: 'hljs language-',
    highlight(code, lang) {
      const language = hljs.getLanguage(lang) ? lang : 'nginx'

      return hljs.highlight(code, { language }).value
    },
  }),
)

marked.setOptions({
  pedantic: false,
  gfm: true,
  breaks: false,
})

function storeRecord() {
  openai.store_record({
    file_name: props.path,
    messages: messages.value,
  })
}

function clearRecord() {
  openai.store_record({
    file_name: props.path,
    messages: [],
  })
  messages.value = []
}

const editingIdx = ref(-1)

async function regenerate(index: number) {
  editingIdx.value = -1
  messages.value = messages.value?.slice(0, index)
  await nextTick()
  await request()
}

const show = computed(() => !messages.value || messages.value?.length === 0)
</script>

<template>
  <div
    v-if="show"
    class="chat-start"
  >
    <AButton
      :loading="loading"
      @click="send"
    >
      <Icon
        v-if="!loading"
        :component="ChatGPT_logo"
      />
      {{ $gettext('Ask ChatGPT for Help') }}
    </AButton>
  </div>
  <div
    v-else
    class="chatgpt-container"
  >
    <AList
      class="chatgpt-log"
      item-layout="horizontal"
      :data-source="messages"
    >
      <template #renderItem="{ item, index }">
        <AListItem>
          <AComment :author="item.role === 'assistant' ? $gettext('Assistant') : $gettext('User')">
            <template #content>
              <div
                v-if="item.role === 'assistant' || editingIdx !== index"
                v-dompurify-html="marked.parse(item.content)"
                class="content"
              />
              <AInput
                v-else
                v-model:value="item.content"
                class="pa-0"
                :bordered="false"
              />
            </template>
            <template #actions>
              <span
                v-if="item.role === 'user' && editingIdx !== index"
                @click="editingIdx = index"
              >
                {{ $gettext('Modify') }}
              </span>
              <template v-else-if="editingIdx === index">
                <span @click="regenerate(index + 1)">{{ $gettext('Save') }}</span>
                <span @click="editingIdx = -1">{{ $gettext('Cancel') }}</span>
              </template>
              <span
                v-else-if="!loading"
                @click="regenerate(index)"
              >
                {{ $gettext('Reload') }}
              </span>
            </template>
          </AComment>
        </AListItem>
      </template>
    </AList>
    <div class="input-msg">
      <div class="control-btn">
        <ASpace v-show="!loading">
          <APopconfirm
            :cancel-text="$gettext('No')"
            :ok-text="$gettext('OK')"
            :title="$gettext('Are you sure you want to clear the record of chat?')"
            @confirm="clearRecord"
          >
            <AButton type="text">
              {{ $gettext('Clear') }}
            </AButton>
          </APopconfirm>
          <AButton
            type="text"
            @click="regenerate((messages?.length ?? 1) - 1)"
          >
            {{ $gettext('Regenerate response') }}
          </AButton>
        </ASpace>
      </div>
      <ATextarea
        v-model:value="askBuffer"
        auto-size
      />
      <div class="send-btn">
        <AButton
          size="small"
          type="text"
          :loading="loading"
          @click="send"
        >
          <SendOutlined />
        </AButton>
      </div>
    </div>
  </div>
</template>

<style lang="less" scoped>
.chatgpt-container {
  margin: 0 auto;
  max-width: 800px;

  .chatgpt-log {
    .content {
      width: 100%;

      :deep(.hljs) {
        border-radius: 5px;
      }
    }

    :deep(.ant-list-item) {
      padding: 0;
    }

    :deep(.ant-comment-content) {
      width: 100%;
    }

    :deep(.ant-comment) {
      width: 100%;
    }

    :deep(.ant-comment-content-detail) {
      width: 100%;

      p {
        margin-bottom: 10px;
      }
    }

    :deep(.ant-list-item:first-child) {
      display: none;
    }
  }

  .input-msg {
    position: relative;

    .control-btn {
      display: flex;
      justify-content: center;
    }

    .send-btn {
      position: absolute;
      right: 0;
      bottom: 3px;
    }
  }
}
</style>
