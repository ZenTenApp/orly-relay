import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { useNostr } from '@/providers/NostrProvider'
import { DEFAULT_MODEL, fetchModels, TAnthropicModel } from '@/services/llm.service'
import storage from '@/services/local-storage.service'
import { TLlmConfig } from '@/types'
import { LoaderCircle } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

const DEFAULT_CONFIG: TLlmConfig = {
  apiKey: '',
  model: '',
  systemPrompt: '',
  autoRewrite: false
}

export default function LlmSetting() {
  const { t } = useTranslation()
  const { pubkey } = useNostr()
  const [config, setConfig] = useState<TLlmConfig>(DEFAULT_CONFIG)
  const [models, setModels] = useState<TAnthropicModel[]>([])
  const [loadingModels, setLoadingModels] = useState(false)

  useEffect(() => {
    if (pubkey) {
      setConfig(storage.getLlmConfig(pubkey) ?? DEFAULT_CONFIG)
    }
  }, [pubkey])

  // Fetch models when API key changes
  useEffect(() => {
    if (!config.apiKey || config.apiKey.length < 10) {
      setModels([])
      return
    }

    let cancelled = false
    const timer = setTimeout(async () => {
      setLoadingModels(true)
      try {
        const result = await fetchModels(config.apiKey)
        if (!cancelled) setModels(result)
      } catch (err) {
        console.error('[LLM] Failed to fetch models:', err)
        if (!cancelled) setModels([])
      } finally {
        if (!cancelled) setLoadingModels(false)
      }
    }, 500) // debounce while typing the key

    return () => {
      cancelled = true
      clearTimeout(timer)
    }
  }, [config.apiKey])

  const save = useCallback(
    (updated: TLlmConfig) => {
      setConfig(updated)
      if (pubkey) {
        storage.setLlmConfig(pubkey, updated)
      }
    },
    [pubkey]
  )

  const selectedModel = config.model || DEFAULT_MODEL

  return (
    <div className="space-y-2">
      <Label className="text-base font-semibold">{t('LLM Settings')}</Label>

      <div className="space-y-1">
        <Label htmlFor="llm-api-key">{t('Anthropic API key')}</Label>
        <Input
          id="llm-api-key"
          type="password"
          placeholder="sk-ant-..."
          value={config.apiKey}
          onChange={(e) => save({ ...config, apiKey: e.target.value })}
        />
      </div>

      <div className="space-y-1">
        <Label>{t('Model')}</Label>
        {loadingModels ? (
          <div className="flex items-center gap-2 h-10 px-3 text-sm text-muted-foreground">
            <LoaderCircle className="size-4 animate-spin" />
            {t('Loading models...')}
          </div>
        ) : models.length > 0 ? (
          <Select value={selectedModel} onValueChange={(v) => save({ ...config, model: v })}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {models.map((m) => (
                <SelectItem key={m.id} value={m.id}>
                  {m.display_name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        ) : (
          <div className="flex items-center h-10 px-3 text-sm text-muted-foreground rounded-lg border border-input">
            {config.apiKey
              ? t('Enter a valid API key to load models')
              : t('Enter API key first')}
          </div>
        )}
      </div>

      <div className="space-y-1">
        <Label htmlFor="llm-system-prompt">{t('Rewrite instructions')}</Label>
        <Textarea
          id="llm-system-prompt"
          rows={6}
          placeholder={t(
            'e.g. Rewrite the following note to be concise and clear. Preserve the original meaning and tone.'
          )}
          value={config.systemPrompt}
          onChange={(e) => save({ ...config, systemPrompt: e.target.value })}
        />
      </div>

      <div className="flex items-center justify-between min-h-9">
        <Label htmlFor="llm-auto-rewrite" className="text-base font-normal">
          {t('Auto-rewrite on publish')}
        </Label>
        <Switch
          id="llm-auto-rewrite"
          checked={config.autoRewrite}
          onCheckedChange={(checked) => save({ ...config, autoRewrite: checked })}
        />
      </div>

      <p className="text-xs text-muted-foreground">
        {t('Only Anthropic Claude is supported.')}{' '}
        <a
          href="https://git.mleku.dev/mleku/smesh"
          target="_blank"
          rel="noopener noreferrer"
          className="underline"
        >
          {t('Create an issue or PR for other APIs')}
        </a>
      </p>
    </div>
  )
}
