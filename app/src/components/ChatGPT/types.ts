export interface CodeBlockState {
  isInCodeBlock: boolean
  backtickCount: number
}

export interface ChatGPTProps {
  content: string
  path?: string
}
