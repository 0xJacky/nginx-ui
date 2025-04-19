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

// Global buffer for accumulation
let buffer = ''

// Track last chunk to avoid immediate repeated content
let lastChunkStr = ''

// define a type for tracking code block state
interface CodeBlockState {
  isInCodeBlock: boolean
  backtickCount: number
}

const codeBlockState: CodeBlockState = reactive({
  isInCodeBlock: false, // if in ``` code block
  backtickCount: 0, // count of ```
})

/**
 * transformReasonerThink: if <think> appears but is not paired with </think>, it will be automatically supplemented, and the entire text will be converted to a Markdown quote
 */
function transformReasonerThink(rawText: string): string {
  // 1. Count number of <think> vs </think>
  const openThinkRegex = /<think>/gi
  const closeThinkRegex = /<\/think>/gi

  const openCount = (rawText.match(openThinkRegex) || []).length
  const closeCount = (rawText.match(closeThinkRegex) || []).length

  // 2. If open tags exceed close tags, append missing </think> at the end
  if (openCount > closeCount) {
    const diff = openCount - closeCount
    rawText += '</think>'.repeat(diff)
  }

  // 3. Replace <think>...</think> blocks with Markdown blockquote ("> ...")
  return rawText.replace(/<think>([\s\S]*?)<\/think>/g, (match, p1) => {
    // Split the inner text by line, prefix each with "> "
    const lines = p1.trim().split('\n')
    const blockquoted = lines.map(line => `> ${line}`).join('\n')
    // Return the replaced Markdown quote
    return `\n${blockquoted}\n`
  })
}

/**
 * transformText: transform the text
 */
function transformText(rawText: string): string {
  return transformReasonerThink(rawText)
}

/**
 * scrollToBottom: Scroll container to bottom
 */
function scrollToBottom() {
  const container = document.querySelector('.right-settings .ant-card-body')
  if (container)
    container.scrollTop = container.scrollHeight
}

/**
 * updateCodeBlockState: The number of unnecessary scans is reduced by changing the scanning method of incremental content
 */
function updateCodeBlockState(chunk: string) {
  // count all ``` in chunk
  // note to distinguish how many "backticks" are not paired

  const regex = /```/g

  while (regex.exec(chunk) !== null) {
    codeBlockState.backtickCount++
    // if backtickCount is even -> closed
    codeBlockState.isInCodeBlock = codeBlockState.backtickCount % 2 !== 0
  }
}

/**
 * applyChunk: Process one SSE chunk and type out content character by character
 * @param input   A chunk of data (Uint8Array) from SSE
 * @param targetMsg  The assistant-type message object being updated
 */

async function applyChunk(input: Uint8Array, targetMsg: ChatComplicationMessage) {
  const decoder = new TextDecoder('utf-8')
  const raw = decoder.decode(input)
  // SSE default split by segment
  const lines = raw.split('\n\n')

  for (const line of lines) {
    if (!line.startsWith('event:message\ndata:'))
      continue

    const dataStr = line.slice('event:message\ndata:'.length)
    if (!dataStr)
      continue

    const content = JSON.parse(dataStr).content as string
    if (!content || content.trim() === '')
      continue
    if (content === lastChunkStr)
      continue

    lastChunkStr = content

    // Only detect substrings
    // 1. This can be processed in batches according to actual needs, reducing the number of character processing times
    updateCodeBlockState(content)

    for (const c of content) {
      buffer += c
      // codeBlockState.isInCodeBlock check if in code block
      targetMsg.content = buffer
      await nextTick()
      await new Promise(resolve => setTimeout(resolve, 20))
      scrollToBottom()
    }
  }
}

/**
 * request: Send messages to server, receive SSE, and process by typing out chunk by chunk
 */
async function request() {
  loading.value = true

  // Add an "assistant" message object
  const t = ref<ChatComplicationMessage>({
    role: 'assistant',
    content: '',
  })

  messages.value.push(t.value)

  // Reset buffer flags each time
  buffer = ''
  lastChunkStr = ''

  await nextTick()
  scrollToBottom()

  const user = useUserStore()
  const { token } = storeToRefs(user)

  const res = await fetch(urlJoin(window.location.pathname, '/api/chatgpt'), {
    method: 'POST',
    headers: {
      Accept: 'text/event-stream',
      Authorization: token.value,
    },
    body: JSON.stringify({
      filepath: props.path,
      messages: messages.value.slice(0, messages.value.length - 1),
    }),
  })

  if (!res.body) {
    loading.value = false
    return
  }

  const reader = res.body.getReader()

  while (true) {
    try {
      const { done, value } = await reader.read()
      if (done) {
        // SSE stream ended
        setTimeout(() => {
          scrollToBottom()
        }, 300)
        break
      }
      if (value) {
        // Process each chunk
        await applyChunk(value, t.value)
      }
    }
    catch {
      // In case of error
      break
    }
  }

  loading.value = false
  storeRecord()
}

/**
 * send: Add user message into messages then call request
 */
async function send() {
  if (!messages.value)
    messages.value = []

  if (messages.value.length === 0) {
    // The first message
    messages.value = [{
      role: 'user',
      content: `${props.content}\n\nCurrent Language Code: ${current.value}`,
    }]
  }
  else {
    // Append user's new message
    messages.value.push({
      role: 'user',
      content: askBuffer.value,
    })
    askBuffer.value = ''
  }

  await nextTick()
  await request()
}

// Markdown renderer
const marked = new Marked(
  markedHighlight({
    langPrefix: 'hljs language-',
    highlight(code, lang) {
      const language = hljs.getLanguage(lang) ? lang : 'nginx'
      return hljs.highlight(code, { language }).value
    },
  }),
)

// Basic marked options
marked.setOptions({
  pedantic: false,
  gfm: true,
  breaks: false,
})

/**
 * storeRecord: Save chat history
 */
function storeRecord() {
  openai.store_record({
    file_name: props.path,
    messages: messages.value,
  })
}

/**
 * clearRecord: Clears all messages
 */
function clearRecord() {
  openai.store_record({
    file_name: props.path,
    messages: [],
  })
  messages.value = []
}

// Manage editing
const editingIdx = ref(-1)

/**
 * regenerate: Removes messages after index and re-request the answer
 */
async function regenerate(index: number) {
  editingIdx.value = -1
  messages.value = messages.value.slice(0, index)
  await nextTick()
  await request()
}

/**
 * show: If empty, display start button
 */
const show = computed(() => !messages.value || messages.value.length === 0)
</script>

<template>
  <div
    v-if="show"
    class="chat-start mt-4"
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
                v-dompurify-html="marked.parse(transformText(item.content))"
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

      :deep(blockquote) {
        display: block;
        opacity: 0.6;
        margin: 0.5em 0;
        padding-left: 1em;
        border-left: 3px solid #ccc;
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
