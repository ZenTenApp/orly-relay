import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle
} from '@/components/ui/sheet'
import { useVisualViewportHeight } from '@/hooks/useVisualViewportHeight'
import { usePrimaryPage } from '@/PageManager'
import { useScreenSize } from '@/providers/ScreenSizeProvider'
import postEditor from '@/services/post-editor.service'
import { ArrowLeft, X } from 'lucide-react'
import { Event } from 'nostr-tools'
import { Dispatch, useMemo, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import PostContent, { TPostContentHandle } from './PostContent'
import Title from './Title'

export default function PostEditor({
  defaultContent = '',
  parentStuff,
  open,
  setOpen,
  highlightedText,
  inline = false
}: {
  defaultContent?: string
  parentStuff?: Event | string
  open: boolean
  setOpen: Dispatch<boolean>
  highlightedText?: string
  inline?: boolean
}) {
  const { t } = useTranslation()
  const { isSmallScreen } = useScreenSize()
  const { navigate } = usePrimaryPage()
  const viewportHeight = useVisualViewportHeight()
  const contentRef = useRef<TPostContentHandle>(null)

  const handleReset = () => {
    contentRef.current?.reset()
  }

  const content = useMemo(() => {
    return (
      <PostContent
        ref={contentRef}
        defaultContent={defaultContent}
        parentStuff={parentStuff}
        close={() => {
          setOpen(false)
          navigate('home')
        }}
        highlightedText={highlightedText}
      />
    )
  }, [highlightedText, navigate])

  if (inline) {
    if (!open) return null
    return (
      <div className="border-t border-b bg-card animate-in slide-in-from-top-2 fade-in duration-200">
        <div className="flex items-center h-10 px-3 border-b shrink-0">
          <div className="flex-1 text-sm font-medium truncate">
            {highlightedText ? t('Create Highlight') : <Title parentStuff={parentStuff} />}
          </div>
          <div className="flex items-center gap-1">
            <button
              onClick={handleReset}
              className="flex items-center justify-center w-8 h-8 rounded hover:bg-accent transition-colors text-sm"
              aria-label="Reset"
            >
              🧹
            </button>
            <button
              onClick={() => setOpen(false)}
              className="flex items-center justify-center w-8 h-8 rounded hover:bg-accent transition-colors"
              aria-label="Close"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
        </div>
        <div className="px-4 py-3">{content}</div>
      </div>
    )
  }

  if (isSmallScreen) {
    return (
      <Sheet open={open} onOpenChange={setOpen}>
        <SheetContent
          className="w-full p-0 border-none flex flex-col"
          style={{ height: `${viewportHeight}px` }}
          side="bottom"
          hideClose
          onEscapeKeyDown={(e) => {
            if (postEditor.isSuggestionPopupOpen) {
              e.preventDefault()
              postEditor.closeSuggestionPopup()
            }
          }}
        >
          <div className="flex items-center h-12 border-b shrink-0">
            <button
              onClick={() => setOpen(false)}
              className="flex items-center justify-center w-10 h-full hover:bg-accent transition-colors"
              aria-label="Close"
            >
              <ArrowLeft className="w-5 h-5" />
            </button>
            <SheetHeader className="flex-1">
              <SheetTitle className="text-start text-base font-medium">
                {highlightedText ? t('Create Highlight') : <Title parentStuff={parentStuff} />}
              </SheetTitle>
              <SheetDescription className="hidden" />
            </SheetHeader>
            <button
              onClick={handleReset}
              className="flex items-center justify-center w-10 h-full hover:bg-accent transition-colors"
              aria-label="Reset"
            >
              🧹
            </button>
          </div>
          <div className="flex flex-col flex-1 min-h-0 px-4 py-4">{content}</div>
        </SheetContent>
      </Sheet>
    )
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent
        className="p-0 max-w-2xl flex flex-col overflow-hidden"
        style={{ maxHeight: `${Math.min(viewportHeight * 0.9, viewportHeight)}px` }}
        withoutClose
        onEscapeKeyDown={(e) => {
          if (postEditor.isSuggestionPopupOpen) {
            e.preventDefault()
            postEditor.closeSuggestionPopup()
          }
        }}
      >
        <div className="px-6 pt-6 pb-2 shrink-0">
          <DialogHeader className="flex flex-row items-center justify-between">
            <DialogTitle>
              {highlightedText ? t('Create Highlight') : <Title parentStuff={parentStuff} />}
            </DialogTitle>
            <button
              onClick={handleReset}
              className="flex items-center justify-center w-8 h-8 rounded hover:bg-accent transition-colors"
              aria-label="Reset"
            >
              🧹
            </button>
            <DialogDescription className="hidden" />
          </DialogHeader>
        </div>
        <div className="flex-1 min-h-0 overflow-y-auto px-6 pb-6">
          {content}
        </div>
      </DialogContent>
    </Dialog>
  )
}
