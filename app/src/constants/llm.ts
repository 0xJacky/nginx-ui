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

export type LLMThinkingLevel = 'off' | 'low' | 'medium' | 'high'

export type LLMThinkingParams = Partial<Record<LLMThinkingLevel, Record<string, unknown>>>

export const LLM_THINKING_LEVELS: LLMThinkingLevel[] = ['off', 'low', 'medium', 'high']

export function thinkingLevelLabel(level: LLMThinkingLevel | '') {
  switch (level) {
    case 'off':
      return $gettext('Off')
    case 'low':
      return $gettext('Low')
    case 'medium':
      return $gettext('Medium')
    case 'high':
      return $gettext('High')
    default:
      return $gettext('Default')
  }
}

export interface LLMThinkingPreset {
  value: string
  label: () => string
  // The request fields for each supported level, undefined for custom
  params?: LLMThinkingParams
  // Model names this preset is suggested for when a model is added
  match?: RegExp
}

function effortLevels(build: (level: Exclude<LLMThinkingLevel, 'off'>) => Record<string, unknown>) {
  return {
    low: build('low'),
    medium: build('medium'),
    high: build('high'),
  }
}

// Provider conventions for selecting a thinking level through the chat
// completion request body. Values can be edited per model after applying one.
export const LLM_THINKING_PRESETS: LLMThinkingPreset[] = [
  {
    value: 'none',
    label: () => $gettext('No thinking control'),
    params: {},
  },
  {
    value: 'openai',
    label: () => 'OpenAI / Ollama (reasoning_effort)',
    params: {
      off: { reasoning_effort: 'none' },
      ...effortLevels(level => ({ reasoning_effort: level })),
    },
    match: /^(?:gpt-5|o\d)/i,
  },
  {
    value: 'thinking_type',
    label: () => 'DeepSeek / Volcengine Ark (thinking.type)',
    params: {
      off: { thinking: { type: 'disabled' } },
      ...effortLevels(level => ({ thinking: { type: 'enabled' }, reasoning_effort: level })),
    },
    match: /deepseek|doubao/i,
  },
  {
    value: 'qwen',
    label: () => 'Qwen / Model Studio (enable_thinking)',
    params: {
      off: { enable_thinking: false },
      low: { enable_thinking: true, thinking_budget: 1024 },
      medium: { enable_thinking: true, thinking_budget: 8192 },
      high: { enable_thinking: true },
    },
    match: /qwen/i,
  },
  {
    value: 'openrouter',
    label: () => 'OpenRouter (reasoning.effort)',
    params: {
      off: { reasoning: { enabled: false } },
      ...effortLevels(level => ({ reasoning: { effort: level } })),
    },
  },
  {
    value: 'anthropic',
    label: () => 'Claude (thinking.budget_tokens)',
    params: {
      off: { thinking: { type: 'disabled' } },
      low: { thinking: { type: 'enabled', budget_tokens: 2048 } },
      medium: { thinking: { type: 'enabled', budget_tokens: 8192 } },
      high: { thinking: { type: 'enabled', budget_tokens: 24576 } },
    },
    match: /claude/i,
  },
  {
    value: 'chat_template',
    label: () => 'vLLM / SGLang (chat_template_kwargs)',
    params: {
      off: { chat_template_kwargs: { enable_thinking: false } },
      high: { chat_template_kwargs: { enable_thinking: true } },
    },
  },
  {
    value: 'custom',
    label: () => $gettext('Custom'),
  },
]

const OLLAMA_PORT = /:11434(?:\/|$)/

// Suggests how a newly added model switches thinking. The serving endpoint
// wins over the model name: Ollama takes reasoning_effort for every model.
export function suggestThinkingPreset(modelName: string, baseUrl = ''): LLMThinkingPreset {
  const byEndpoint = OLLAMA_PORT.test(baseUrl) ? 'openai' : undefined
  return LLM_THINKING_PRESETS.find(preset => preset.value === byEndpoint)
    ?? LLM_THINKING_PRESETS.find(preset => preset.match?.test(modelName))
    ?? LLM_THINKING_PRESETS[0]
}
