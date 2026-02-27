const ANTHROPIC_API_URL = 'https://api.anthropic.com/v1/messages'
const ANTHROPIC_MODELS_URL = 'https://api.anthropic.com/v1/models'
const ANTHROPIC_VERSION = '2023-06-01'
export const DEFAULT_MODEL = 'claude-sonnet-4-20250514'
const MAX_TOKENS = 4096

export type TAnthropicModel = {
  id: string
  display_name: string
}

// Cache models per API key to avoid repeated fetches
const modelCache = new Map<string, { models: TAnthropicModel[]; timestamp: number }>()
const CACHE_TTL = 30 * 60 * 1000 // 30 minutes

export async function fetchModels(apiKey: string): Promise<TAnthropicModel[]> {
  const cached = modelCache.get(apiKey)
  if (cached && Date.now() - cached.timestamp < CACHE_TTL) {
    return cached.models
  }

  const allModels: TAnthropicModel[] = []
  let afterId: string | undefined

  // Paginate through all models
  do {
    const url = new URL(ANTHROPIC_MODELS_URL)
    url.searchParams.set('limit', '100')
    if (afterId) url.searchParams.set('after_id', afterId)

    const response = await fetch(url.toString(), {
      headers: {
        'x-api-key': apiKey,
        'anthropic-version': ANTHROPIC_VERSION,
        'anthropic-dangerous-direct-browser-access': 'true'
      }
    })

    if (!response.ok) {
      const errorBody = await response.text()
      console.error('[LLM] Failed to fetch models:', response.status, errorBody)
      throw new Error(`Failed to fetch models (${response.status})`)
    }

    const data = await response.json()
    const models = (data.data ?? []) as TAnthropicModel[]
    allModels.push(...models)
    afterId = data.has_more ? data.last_id : undefined
  } while (afterId)

  modelCache.set(apiKey, { models: allModels, timestamp: Date.now() })
  return allModels
}

export async function rewriteText(
  apiKey: string,
  systemPrompt: string,
  noteText: string,
  model?: string
): Promise<string> {
  const body = {
    model: model || DEFAULT_MODEL,
    max_tokens: MAX_TOKENS,
    system: systemPrompt + '\n\nIMPORTANT: Output ONLY the rewritten text. Do not include any commentary, explanations, notes, or preamble. Your entire response must be the rewritten text and nothing else.',
    messages: [
      {
        role: 'user',
        content: noteText
      }
    ]
  }
  console.debug('[LLM] Sending request:', { model: body.model, systemPrompt: body.system })

  const response = await fetch(ANTHROPIC_API_URL, {
    method: 'POST',
    headers: {
      'x-api-key': apiKey,
      'anthropic-version': ANTHROPIC_VERSION,
      'content-type': 'application/json',
      'anthropic-dangerous-direct-browser-access': 'true'
    },
    body: JSON.stringify(body)
  })

  if (!response.ok) {
    const errorBody = await response.text()
    console.error('[LLM] API error:', response.status, errorBody)
    throw new Error(`Anthropic API error (${response.status}): ${errorBody}`)
  }

  const data = await response.json()
  console.debug('[LLM] Response received:', { stopReason: data.stop_reason, model: data.model })

  const textBlock = data.content?.find(
    (block: { type: string; text?: string }) => block.type === 'text'
  )
  if (!textBlock?.text) {
    throw new Error('No text content in API response')
  }
  return textBlock.text
}
