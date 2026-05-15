export interface LLMProviderPreset {
  value: string
  label: string
  baseUrl?: string
}

export const LLM_MODELS = [
  'deepseek-ai/deepseek-v4-flash',
  'deepseek-v3',
  'o3-mini',
  'o1',
  'deepseek-reasoner',
  'deepseek-chat',
  'gpt-4o-mini',
  'gpt-4o',
  'gpt-4',
  'gpt-4-32k',
  'gpt-4-turbo',
  'gpt-3.5-turbo',
]

export const LLM_PROVIDERS: LLMProviderPreset[] = [
  {
    value: 'openai',
    label: 'OpenAI',
  },
  {
    value: 'atlas_cloud',
    label: 'Atlas Cloud',
    baseUrl: 'https://api.atlascloud.ai/v1',
  },
  {
    value: 'custom',
    label: 'Custom',
  },
]

export const LLM_PROVIDER_BASE_URLS = [
  'https://api.openai.com',
  'https://api.atlascloud.ai/v1',
  'https://api.deepseek.com',
  'http://localhost:11434',
]
