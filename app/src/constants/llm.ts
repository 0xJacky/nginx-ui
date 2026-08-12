export interface LLMModelPricing {
  input: number
  output: number
  cacheRead?: number
  cacheWrite?: number
}

export interface LLMModelPreset {
  value: string
  contextWindow: number
  pricing: LLMModelPricing
  inputModalities: string[]
  thinkingModes: string[]
}

export interface LLMProviderEndpoint {
  region: string
  openaiBaseUrl: string
  anthropicBaseUrl: string
  docsUrl: string
}

export interface LLMProviderPreset {
  value: string
  label: string
  baseUrl?: string
  models?: LLMModelPreset[]
  endpoints?: LLMProviderEndpoint[]
}

const MINIMAX_MODELS: LLMModelPreset[] = [
  {
    value: 'MiniMax-M3',
    contextWindow: 1_000_000,
    pricing: {
      input: 0.6,
      output: 2.4,
      cacheRead: 0.12,
    },
    inputModalities: ['text', 'image', 'video'],
    thinkingModes: ['adaptive', 'disabled'],
  },
  {
    value: 'MiniMax-M2.7',
    contextWindow: 204_800,
    pricing: {
      input: 0.3,
      output: 1.2,
      cacheRead: 0.06,
      cacheWrite: 0.375,
    },
    inputModalities: ['text'],
    thinkingModes: ['always_on'],
  },
]

export const LLM_MODELS = [
  ...MINIMAX_MODELS.map(model => model.value),
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
    value: 'minimax',
    label: 'MiniMax',
    baseUrl: 'https://api.minimax.io/v1',
    models: MINIMAX_MODELS,
    endpoints: [
      {
        region: 'global_en',
        openaiBaseUrl: 'https://api.minimax.io/v1',
        anthropicBaseUrl: 'https://api.minimax.io/anthropic',
        docsUrl: 'https://platform.minimax.io/docs',
      },
      {
        region: 'cn_zh',
        openaiBaseUrl: 'https://api.minimaxi.com/v1',
        anthropicBaseUrl: 'https://api.minimaxi.com/anthropic',
        docsUrl: 'https://platform.minimaxi.com/docs',
      },
    ],
  },
  {
    value: 'custom',
    label: 'Custom',
  },
]

export const LLM_PROVIDER_BASE_URLS = [...new Set([
  'https://api.openai.com',
  'https://api.atlascloud.ai/v1',
  ...LLM_PROVIDERS.flatMap(provider => provider.endpoints?.map(endpoint => endpoint.openaiBaseUrl) ?? []),
  'https://api.deepseek.com',
  'http://localhost:11434',
])]
