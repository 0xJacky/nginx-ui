export interface CodeBlockState {
  isInCodeBlock: boolean
  backtickCount: number
}

export interface LLMProps {
  content: string
  path?: string
}

export interface LLMSession {
  id: string
  title: string
  path?: string
  createdAt: Date
  updatedAt: Date
  messageCount: number
}

export interface LLMSessionState {
  sessions: LLMSession[]
  activeSessionId: string | null
}
