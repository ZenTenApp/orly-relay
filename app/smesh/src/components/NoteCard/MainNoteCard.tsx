import PostEditor from '@/components/PostEditor'
import { Separator } from '@/components/ui/separator'
import { useKeyboardNavigable } from '@/hooks/useKeyboardNavigable'
import { toNote } from '@/lib/link'
import { cn } from '@/lib/utils'
import { useSecondaryPage } from '@/PageManager'
import { TNavigationColumn } from '@/providers/KeyboardNavigationProvider'
import { Event } from 'nostr-tools'
import { useState } from 'react'
import Collapsible from '../Collapsible'
import Note from '../Note'
import StuffStats from '../StuffStats'
import PinnedButton from './PinnedButton'
import RepostDescription from './RepostDescription'

export default function MainNoteCard({
  event,
  className,
  reposters,
  embedded,
  originalNoteId,
  pinned = false,
  navColumn,
  navIndex
}: {
  event: Event
  className?: string
  reposters?: string[]
  embedded?: boolean
  originalNoteId?: string
  pinned?: boolean
  navColumn?: TNavigationColumn
  navIndex?: number
}) {
  const { push } = useSecondaryPage()
  const { ref, isSelected } = useKeyboardNavigable(navColumn ?? 1, navIndex ?? 0, {
    meta: { type: 'note', event }
  })
  const [replyOpen, setReplyOpen] = useState(false)

  return (
    <div
      ref={ref}
      className={cn(className, 'scroll-mt-[6.5rem]', isSelected && 'ring-2 ring-primary ring-inset')}
      onClick={(e) => {
        e.stopPropagation()
        push(toNote(originalNoteId ?? event))
      }}
    >
      <div
        className={cn(
          'clickable transition-all duration-200',
          embedded ? 'p-3 sm:p-4 border rounded-xl bg-card' : 'py-3 hover:bg-accent/30'
        )}
      >
        <Collapsible alwaysExpand={embedded}>
          {pinned && <PinnedButton event={event} />}
          <RepostDescription className={embedded ? '' : 'px-4'} reposters={reposters} />
          <Note
            className={embedded ? '' : 'px-4'}
            size={embedded ? 'small' : 'normal'}
            event={event}
            originalNoteId={originalNoteId}
          />
        </Collapsible>
        {!embedded && (
          <StuffStats
            className="mt-3 px-4"
            stuff={event}
            onReplyClick={() => setReplyOpen(true)}
          />
        )}
      </div>
      {!embedded && <Separator />}
      {replyOpen && (
        <div onClick={(e) => e.stopPropagation()}>
          <PostEditor
            inline
            parentStuff={event}
            open={replyOpen}
            setOpen={setReplyOpen}
          />
        </div>
      )}
    </div>
  )
}
