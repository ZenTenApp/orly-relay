import Note from '@/components/Note'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  createCommentDraftEvent,
  createHighlightDraftEvent,
  createPollDraftEvent,
  createShortTextNoteDraftEvent,
  deleteDraftEventCache
} from '@/lib/draft-event'
import { cn, isTouchDevice } from '@/lib/utils'
import { useNostr } from '@/providers/NostrProvider'
import { useScreenSize } from '@/providers/ScreenSizeProvider'
import client from '@/services/client.service'
import postEditorCache from '@/services/post-editor-cache.service'
import threadService from '@/services/thread.service'
import { TPollCreateData } from '@/types'
import { rewriteText } from '@/services/llm.service'
import storage from '@/services/local-storage.service'
import { ImageUp, ListTodo, LoaderCircle, Settings, Smile, Sparkles, X } from 'lucide-react'
import { Event, kinds } from 'nostr-tools'
import { forwardRef, useEffect, useImperativeHandle, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import EmojiPickerDialog from '../EmojiPickerDialog'
import Mentions from './Mentions'
import PollEditor from './PollEditor'
import PostOptions from './PostOptions'
import PostTextarea, { TPostTextareaHandle } from './PostTextarea'
import Uploader from './Uploader'

export type TPostContentHandle = {
  reset: () => void
}

const PostContent = forwardRef<
  TPostContentHandle,
  {
    defaultContent?: string
    parentStuff?: Event | string
    close: () => void
    highlightedText?: string
  }
>(({ defaultContent = '', parentStuff, close, highlightedText }, ref) => {
  const { t } = useTranslation()
  const { pubkey, publish, checkLogin } = useNostr()
  const [text, setText] = useState('')
  const textareaRef = useRef<TPostTextareaHandle>(null)
  const [posting, setPosting] = useState(false)
  const [uploadProgresses, setUploadProgresses] = useState<
    { file: File; progress: number; cancel: () => void }[]
  >([])
  const parentEvent = useMemo(
    () => (parentStuff && typeof parentStuff !== 'string' ? parentStuff : undefined),
    [parentStuff]
  )
  const { isSmallScreen } = useScreenSize()
  const [showMoreOptions, setShowMoreOptions] = useState(false)
  const [addClientTag, setAddClientTag] = useState(false)
  const [mentions, setMentions] = useState<string[]>([])
  const [isNsfw, setIsNsfw] = useState(false)
  const [isPoll, setIsPoll] = useState(false)
  const [pollCreateData, setPollCreateData] = useState<TPollCreateData>({
    isMultipleChoice: false,
    options: ['', ''],
    endsAt: undefined,
    relays: []
  })
  const [minPow, setMinPow] = useState(0)
  const [rewriting, setRewriting] = useState(false)
  const llmConfig = useMemo(() => (pubkey ? storage.getLlmConfig(pubkey) : null), [pubkey])
  const llmConfigured = !!(llmConfig?.apiKey && llmConfig?.systemPrompt)
  const isFirstRender = useRef(true)
  const canPost = useMemo(() => {
    return (
      !!pubkey &&
      (!!text || !!highlightedText) &&
      !posting &&
      !uploadProgresses.length &&
      (!isPoll || pollCreateData.options.filter((option) => !!option.trim()).length >= 2)
    )
  }, [pubkey, text, highlightedText, posting, uploadProgresses, isPoll, pollCreateData])

  useImperativeHandle(ref, () => ({
    reset: () => {
      textareaRef.current?.clear()
      setText('')
      setMentions([])
      setIsNsfw(false)
      setIsPoll(false)
      setPollCreateData({
        isMultipleChoice: false,
        options: ['', ''],
        endsAt: undefined,
        relays: []
      })
      setAddClientTag(false)
      setMinPow(0)
    }
  }))

  useEffect(() => {
    if (isFirstRender.current) {
      isFirstRender.current = false
      const cachedSettings = postEditorCache.getPostSettingsCache({
        defaultContent,
        parentStuff
      })
      if (cachedSettings) {
        setIsNsfw(cachedSettings.isNsfw ?? false)
        setIsPoll(cachedSettings.isPoll ?? false)
        setPollCreateData(
          cachedSettings.pollCreateData ?? {
            isMultipleChoice: false,
            options: ['', ''],
            endsAt: undefined,
            relays: []
          }
        )
        setAddClientTag(cachedSettings.addClientTag ?? false)
      }
      return
    }
    postEditorCache.setPostSettingsCache(
      { defaultContent, parentStuff },
      {
        isNsfw,
        isPoll,
        pollCreateData,
        addClientTag
      }
    )
  }, [defaultContent, parentStuff, isNsfw, isPoll, pollCreateData, addClientTag])

  const postingRef = useRef(false)

  const post = async (e?: React.MouseEvent) => {
    e?.stopPropagation()
    checkLogin(async () => {
      if (!canPost || !pubkey || postingRef.current) return

      postingRef.current = true
      setPosting(true)
      try {
        // Auto-rewrite if enabled
        let finalText = text
        if (llmConfig?.autoRewrite && llmConfig.apiKey && llmConfig.systemPrompt && text.trim()) {
          try {
            finalText = await rewriteText(llmConfig.apiKey, llmConfig.systemPrompt, text, llmConfig.model)
            textareaRef.current?.replaceText(finalText)
          } catch (error) {
            toast.error(
              `${t('Auto-rewrite failed, posting original text')}: ${error instanceof Error ? error.message : String(error)}`,
              { duration: 5000 }
            )
          }
        }

        const draftEvent = await createDraftEvent({
          parentStuff,
          highlightedText,
          text: finalText,
          mentions,
          isPoll,
          pollCreateData,
          pubkey,
          addClientTag,
          isProtectedEvent: false,
          isNsfw
        })

        // For external content comments, relay selection happens in determineTargetRelays
        // based on relay hints in tags - no need to specify additional relays here
        const additionalRelayUrls = isPoll ? pollCreateData.relays : []

        const newEvent = await publish(draftEvent, {
          additionalRelayUrls,
          minPow
        })
        postEditorCache.clearPostCache({ defaultContent, parentStuff })
        deleteDraftEventCache(draftEvent)
        threadService.addRepliesToThread([newEvent])
        toast.success(t('Post successful'), { duration: 2000 })
        close()

        // Check for relay failures in the background after modal closes
        client.lastPublishFailedRelays.then((failedUrls) => {
          if (failedUrls.length > 0) {
            toast.error(
              `${t('Failed to publish to')}: ${failedUrls.join(', ')}`,
              { duration: 5000 }
            )
          }
        })
      } catch (error) {
        const errors = error instanceof AggregateError ? error.errors : [error]
        errors.forEach((err) => {
          toast.error(
            `${t('Failed to post')}: ${err instanceof Error ? err.message : String(err)}`,
            { duration: 10_000 }
          )
          console.error(err)
        })
        return
      } finally {
        setPosting(false)
        postingRef.current = false
      }
    })
  }

  const handlePollToggle = () => {
    if (parentStuff) return

    setIsPoll((prev) => !prev)
  }

  const handleRewrite = async () => {
    if (!llmConfig?.apiKey || !llmConfig?.systemPrompt || !text.trim() || rewriting) return
    setRewriting(true)
    try {
      const rewritten = await rewriteText(llmConfig.apiKey, llmConfig.systemPrompt, text, llmConfig.model)
      textareaRef.current?.replaceText(rewritten)
    } catch (error) {
      toast.error(
        `${t('Rewrite failed')}: ${error instanceof Error ? error.message : String(error)}`,
        { duration: 10_000 }
      )
    } finally {
      setRewriting(false)
    }
  }

  const handleUploadStart = (file: File, cancel: () => void) => {
    setUploadProgresses((prev) => [...prev, { file, progress: 0, cancel }])
  }

  const handleUploadProgress = (file: File, progress: number) => {
    setUploadProgresses((prev) =>
      prev.map((item) => (item.file === file ? { ...item, progress } : item))
    )
  }

  const handleUploadEnd = (file: File) => {
    setUploadProgresses((prev) => prev.filter((item) => item.file !== file))
  }

  return (
    <div className={cn('space-y-2', isSmallScreen ? 'flex flex-col h-full' : 'flex flex-col')}>
      <div className="flex items-center justify-between shrink-0">
        <div className="flex gap-2 items-center">
          <Uploader
            responsive
            onUploadSuccess={({ primaryUrl }) => {
              textareaRef.current?.appendText(primaryUrl, true)
            }}
            onUploadStart={handleUploadStart}
            onUploadEnd={handleUploadEnd}
            onProgress={handleUploadProgress}
            accept="image/*,video/*,audio/*"
          >
            <Button variant="ghost" size="icon">
              <ImageUp />
            </Button>
          </Uploader>
          {/* I'm not sure why, but after triggering the virtual keyboard,
              opening the emoji picker drawer causes an issue,
              the emoji I tap isn't the one that gets inserted. */}
          {!isTouchDevice() && (
            <EmojiPickerDialog
              onEmojiClick={(emoji) => {
                if (!emoji) return
                textareaRef.current?.insertEmoji(emoji)
              }}
            >
              <Button variant="ghost" size="icon">
                <Smile />
              </Button>
            </EmojiPickerDialog>
          )}
          {!parentStuff && (
            <Button
              variant="ghost"
              size="icon"
              title={t('Create Poll')}
              className={isPoll ? 'bg-accent' : ''}
              onClick={handlePollToggle}
            >
              <ListTodo />
            </Button>
          )}
          <Button
            variant="ghost"
            size="icon"
            className={showMoreOptions ? 'bg-accent' : ''}
            onClick={() => setShowMoreOptions((pre) => !pre)}
          >
            <Settings />
          </Button>
          {llmConfigured && !llmConfig?.autoRewrite && (
            <Button
              variant="ghost"
              size="icon"
              title={t('Rewrite with AI')}
              disabled={!text.trim() || rewriting || posting}
              onClick={handleRewrite}
            >
              {rewriting ? <LoaderCircle className="animate-spin" /> : <Sparkles />}
            </Button>
          )}
        </div>
        <div className="flex gap-2 items-center">
          <Mentions
            content={text}
            parentEvent={parentEvent}
            mentions={mentions}
            setMentions={setMentions}
          />
          <div className="flex gap-2 items-center max-sm:hidden">
            <Button
              variant="secondary"
              onClick={(e) => {
                e.stopPropagation()
                close()
              }}
            >
              {t('Cancel')}
            </Button>
            <Button type="submit" disabled={!canPost} onClick={post}>
              {posting && <LoaderCircle className="animate-spin" />}
              {parentStuff ? (highlightedText ? t('Publish Highlight') : t('Reply')) : t('Post')}
            </Button>
          </div>
        </div>
      </div>
      <div className="shrink-0">
        <PostOptions
          posting={posting}
          show={showMoreOptions}
          addClientTag={addClientTag}
          setAddClientTag={setAddClientTag}
          isNsfw={isNsfw}
          setIsNsfw={setIsNsfw}
          minPow={minPow}
          setMinPow={setMinPow}
        />
      </div>
      {parentEvent && (
        <ScrollArea className="flex max-h-48 flex-col overflow-y-auto rounded-lg border bg-muted/40 shrink-0">
          <div className="p-2 sm:p-3 pointer-events-none">
            {highlightedText ? (
              <div className="flex gap-4">
                <div className="w-1 flex-shrink-0 my-1 bg-primary/60 rounded-md" />
                <div className="italic whitespace-pre-line">{highlightedText}</div>
              </div>
            ) : (
              <Note size="small" event={parentEvent} hideParentNotePreview />
            )}
          </div>
        </ScrollArea>
      )}
      <PostTextarea
        ref={textareaRef}
        text={text}
        setText={setText}
        defaultContent={defaultContent}
        parentStuff={parentStuff}
        onSubmit={() => post()}
        className={cn(isPoll ? 'min-h-20' : 'min-h-52', isSmallScreen && 'flex-1')}
        fillHeight={isSmallScreen}
        onUploadStart={handleUploadStart}
        onUploadProgress={handleUploadProgress}
        onUploadEnd={handleUploadEnd}
        placeholder={highlightedText ? t('Write your thoughts about this highlight...') : undefined}
      />
      {isPoll && (
        <div className="shrink-0">
          <PollEditor
            pollCreateData={pollCreateData}
            setPollCreateData={setPollCreateData}
            setIsPoll={setIsPoll}
          />
        </div>
      )}
      {uploadProgresses.length > 0 &&
        uploadProgresses.map(({ file, progress, cancel }, index) => (
          <div key={`${file.name}-${index}`} className="mt-2 flex items-end gap-2 shrink-0">
            <div className="min-w-0 flex-1">
              <div className="truncate text-xs text-muted-foreground mb-1">
                {file.name ?? t('Uploading...')}
              </div>
              <div className="h-0.5 w-full rounded-full bg-muted overflow-hidden">
                <div
                  className="h-full bg-primary transition-[width] duration-200 ease-out"
                  style={{ width: `${progress}%` }}
                />
              </div>
            </div>
            <button
              type="button"
              onClick={() => {
                cancel?.()
                handleUploadEnd(file)
              }}
              className="text-muted-foreground hover:text-foreground"
              title={t('Cancel')}
            >
              <X className="h-4 w-4" />
            </button>
          </div>
        ))}
      <div className="sm:hidden shrink-0">
        <Button className="w-full" type="submit" disabled={!canPost} onClick={post}>
          {posting && <LoaderCircle className="animate-spin" />}
          {parentStuff ? t('Reply') : t('Post')}
        </Button>
      </div>
    </div>
  )
})

PostContent.displayName = 'PostContent'
export default PostContent

async function createDraftEvent({
  parentStuff,
  text,
  mentions,
  isPoll,
  pollCreateData,
  pubkey,
  addClientTag,
  isProtectedEvent,
  isNsfw,
  highlightedText
}: {
  parentStuff: Event | string | undefined
  text: string
  mentions: string[]
  isPoll: boolean
  pollCreateData: TPollCreateData
  pubkey: string
  addClientTag: boolean
  isProtectedEvent: boolean
  isNsfw: boolean
  highlightedText?: string
}) {
  const { parentEvent, externalContent } =
    typeof parentStuff === 'string'
      ? { parentEvent: undefined, externalContent: parentStuff }
      : { parentEvent: parentStuff, externalContent: undefined }

  if (highlightedText && parentEvent) {
    return createHighlightDraftEvent(highlightedText, text, parentEvent, mentions, {
      addClientTag,
      protectedEvent: isProtectedEvent,
      isNsfw
    })
  }

  if (parentStuff && (externalContent || parentEvent?.kind !== kinds.ShortTextNote)) {
    return await createCommentDraftEvent(text, parentStuff, mentions, {
      addClientTag,
      protectedEvent: isProtectedEvent,
      isNsfw
    })
  }

  if (isPoll) {
    return await createPollDraftEvent(pubkey, text, mentions, pollCreateData, {
      addClientTag,
      isNsfw
    })
  }

  return await createShortTextNoteDraftEvent(text, mentions, {
    parentEvent,
    addClientTag,
    protectedEvent: isProtectedEvent,
    isNsfw
  })
}
