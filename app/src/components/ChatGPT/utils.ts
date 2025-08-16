import type { CodeBlockState } from './types'

/**
 * transformReasonerThink: if <think> appears but is not paired with </think>, it will be automatically supplemented, and the entire text will be converted to a Markdown quote
 */
export function transformReasonerThink(rawText: string): string {
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
export function transformText(rawText: string): string {
  return transformReasonerThink(rawText)
}

/**
 * updateCodeBlockState: The number of unnecessary scans is reduced by changing the scanning method of incremental content
 */
export function updateCodeBlockState(chunk: string, codeBlockState: CodeBlockState) {
  // count all ``` in chunk
  // note to distinguish how many "backticks" are not paired

  const regex = /```/g

  while (regex.exec(chunk) !== null) {
    codeBlockState.backtickCount++
    // if backtickCount is even -> closed
    codeBlockState.isInCodeBlock = codeBlockState.backtickCount % 2 !== 0
  }
}

// Global scroll debouncing
let scrollTimeoutId: number | null = null

/**
 * scrollToBottom: Scroll container to bottom with optimized performance
 */
export function scrollToBottom() {
  // Simple debounce to avoid stuttering from over-optimization
  if (scrollTimeoutId) {
    return
  }

  scrollTimeoutId = window.setTimeout(() => {
    const container = document.querySelector('.right-settings .ant-card-body')
    if (container) {
      // Set scrollTop directly to avoid animation stuttering
      container.scrollTop = container.scrollHeight
    }
    scrollTimeoutId = null
  }, 50) // Reduced to 50ms for better responsiveness
}

/**
 * scrollToBottomSmooth: Smooth scroll version for manual interactions
 */
export function scrollToBottomSmooth() {
  const container = document.querySelector('.right-settings .ant-card-body')
  if (container) {
    container.scrollTo({
      top: container.scrollHeight,
      behavior: 'smooth',
    })
  }
}
