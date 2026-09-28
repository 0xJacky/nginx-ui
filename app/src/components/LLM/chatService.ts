import type { CodeBlockState } from './types'
import type { ChatComplicationMessage } from '@/api/llm'
import { storeToRefs } from 'pinia'
import { urlJoin } from '@/lib/helper'
import { useUserStore } from '@/pinia'
import { updateCodeBlockState } from './utils'

export class ChatService {
  private buffer = ''
  private lastChunkStr = ''
  // A network read can end inside an event, or inside a multi-byte character
  private pendingEvents = ''
  private decoder = new TextDecoder('utf-8')
  private codeBlockState: CodeBlockState = reactive({
    isInCodeBlock: false,
    backtickCount: 0,
  })

  // applyChunk: Process one SSE chunk and update content directly
  private applyChunk(input: Uint8Array, targetMsg: ChatComplicationMessage) {
    this.pendingEvents += this.decoder.decode(input, { stream: true })
    // Events end with a blank line; keep the unfinished tail for the next read
    const lines = this.pendingEvents.split('\n\n')
    this.pendingEvents = lines.pop() ?? ''

    for (const line of lines) {
      if (!line.startsWith('event:message\ndata:'))
        continue

      const dataStr = line.slice('event:message\ndata:'.length)
      if (!dataStr)
        continue

      const event = JSON.parse(dataStr) as { type?: string, content?: string }
      const content = event.content ?? ''

      if (event.type === 'reasoning') {
        targetMsg.reasoning_content = (targetMsg.reasoning_content ?? '') + content
        continue
      }

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
    osInfo?: string,
    model?: string,
    thinking?: string,
  ): Promise<ChatComplicationMessage> {
    // Reset buffer flags each time
    this.buffer = ''
    this.lastChunkStr = ''
    this.pendingEvents = ''
    this.decoder = new TextDecoder('utf-8')
    this.codeBlockState.isInCodeBlock = false
    this.codeBlockState.backtickCount = 0

    const user = useUserStore()
    const { token } = storeToRefs(user)

    // Filter out empty assistant messages for the request
    // Earlier reasoning stays in the chat; the server does not forward it
    const requestMessages = messages
      .filter(msg => msg.role === 'user' || (msg.role === 'assistant' && msg.content.trim() !== ''))
      .map(({ reasoning_content: _, ...msg }) => msg)

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
        os_info: osInfo,
        model: model || undefined,
        thinking: thinking || undefined,
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
