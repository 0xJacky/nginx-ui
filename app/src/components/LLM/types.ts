export interface CodeBlockState {
  isInCodeBlock: boolean
  backtickCount: number
}

export interface LLMProps {
  content: string
  path?: string
}
