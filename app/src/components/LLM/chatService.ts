import type { CodeBlockState } from './types'
import type { ChatComplicationMessage } from '@/api/llm'
import { storeToRefs } from 'pinia'
import { urlJoin } from '@/lib/helper'
import { useUserStore } from '@/pinia'
import { updateCodeBlockState } from './utils'

export class ChatService {
  private buffer = ''
  private lastChunkStr = ''
  private codeBlockState: CodeBlockState = reactive({
    isInCodeBlock: false,
    backtickCount: 0,
  })

  // applyChunk: Process one SSE chunk and update content directly
  private applyChunk(input: Uint8Array, targetMsg: ChatComplicationMessage) {
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
      if (content === this.lastChunkStr)
        continue

      this.lastChunkStr = content

      // Only detect substrings
      updateCodeBlockState(content, this.codeBlockState)

      // Directly append content to buffer
      this.buffer += content

      // Update message content immediately - typewriter effect is handled in ChatMessage.vue
      targetMsg.content = this.buffer
    }
  }

  // request: Send messages to server, receive SSE, and process chunks
  async request(
    type: string | undefined,
    messages: ChatComplicationMessage[],
    onProgress?: (message: ChatComplicationMessage) => void,
    language?: string,
    nginxConfig?: string,
  ): Promise<ChatComplicationMessage> {
    // Reset buffer flags each time
    this.buffer = ''
    this.lastChunkStr = ''
    this.codeBlockState.isInCodeBlock = false
    this.codeBlockState.backtickCount = 0

    const user = useUserStore()
    const { token } = storeToRefs(user)

    // Filter out empty assistant messages for the request
    const requestMessages = messages.filter(msg =>
      msg.role === 'user' || (msg.role === 'assistant' && msg.content.trim() !== ''),
    )

    const res = await fetch(urlJoin(window.location.pathname, '/api/llm'), {
      method: 'POST',
      headers: {
        Accept: 'text/event-stream',
        Authorization: token.value,
      },
      body: JSON.stringify({
        type,
        messages: requestMessages,
        language,
        nginx_config: nginxConfig,
      }),
    })

    if (!res.body) {
      throw new Error('No response body')
    }

    const reader = res.body.getReader()

    // Create assistant message for streaming updates
    const assistantMessage: ChatComplicationMessage = {
      role: 'assistant',
      content: '',
    }

    while (true) {
      try {
        const { done, value } = await reader.read()
        if (done) {
          break
        }
        if (value) {
          // Process each chunk
          this.applyChunk(value, assistantMessage)
          onProgress?.(assistantMessage)
        }
      }
      catch {
        // In case of error
        break
      }
    }

    return assistantMessage
  }
}
