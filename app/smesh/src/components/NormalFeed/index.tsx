import NoteList, { TNoteListRef } from '@/components/NoteList'
import PostEditor from '@/components/PostEditor'
import Tabs from '@/components/Tabs'
import { isTouchDevice } from '@/lib/utils'
import { useCompose } from '@/providers/ComposeProvider'
import { useKindFilter } from '@/providers/KindFilterProvider'
import { useUserTrust } from '@/providers/UserTrustProvider'
import storage, { dispatchSettingsChanged } from '@/services/local-storage.service'
import { TFeedSubRequest, TNoteListMode } from '@/types'
import { useMemo, useRef, useState } from 'react'
import KindFilter from '../KindFilter'
import { RefreshButton } from '../RefreshButton'

export default function NormalFeed({
  subRequests,
  areAlgoRelays = false,
  isMainFeed = false,
  showRelayCloseReason = false,
  enableSocialGraphFilter = false,
  onRefresh
}: {
  subRequests: TFeedSubRequest[]
  areAlgoRelays?: boolean
  isMainFeed?: boolean
  showRelayCloseReason?: boolean
  enableSocialGraphFilter?: boolean
  onRefresh?: () => void
}) {
  const { hideUntrustedNotes } = useUserTrust()
  const { showKinds } = useKindFilter()
  const { composeOpen, closeCompose } = useCompose()
  const [temporaryShowKinds, setTemporaryShowKinds] = useState(showKinds)
  const [listMode, setListMode] = useState<TNoteListMode>(() => {
    const stored = storage.getNoteListMode() as string
    // If stored mode was 'posts' or '24h' (legacy), use 'postsAndReplies' instead
    return stored === 'posts' || stored === '24h' ? 'postsAndReplies' : stored as TNoteListMode
  })
  const supportTouch = useMemo(() => isTouchDevice(), [])
  const noteListRef = useRef<TNoteListRef>(null)
  const topRef = useRef<HTMLDivElement>(null)
  const showKindsFilter = useMemo(() => {
    return subRequests.every((req) => !req.filter.kinds?.length)
  }, [subRequests])

  const handleListModeChange = (mode: TNoteListMode) => {
    setListMode(mode)
    if (isMainFeed) {
      storage.setNoteListMode(mode)
      dispatchSettingsChanged()
    }
    topRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }

  const handleShowKindsChange = (newShowKinds: number[]) => {
    setTemporaryShowKinds(newShowKinds)
    noteListRef.current?.scrollToTop()
  }

  return (
    <>
      <Tabs
        value={listMode}
        tabs={[
          { value: 'postsAndReplies', label: 'Feed' }
        ]}
        onTabChange={(listMode) => {
          handleListModeChange(listMode as TNoteListMode)
        }}
        options={
          <>
            {!supportTouch && (
              <RefreshButton
                onClick={() => {
                  if (onRefresh) {
                    onRefresh()
                    return
                  }
                  noteListRef.current?.refresh()
                }}
              />
            )}
            {showKindsFilter && (
              <KindFilter
                showKinds={temporaryShowKinds}
                onShowKindsChange={handleShowKindsChange}
                showSocialGraphFilter={enableSocialGraphFilter}
              />
            )}
          </>
        }
      />
      {composeOpen && (
        <PostEditor
          inline
          open={composeOpen}
          setOpen={(v) => { if (!v) closeCompose() }}
        />
      )}
      <div ref={topRef} className="scroll-mt-[calc(6rem+1px)]" />
      <NoteList
        ref={noteListRef}
        showKinds={temporaryShowKinds}
        subRequests={subRequests}
        hideReplies={listMode === 'posts'}
        hideUntrustedNotes={hideUntrustedNotes}
        areAlgoRelays={areAlgoRelays}
        showRelayCloseReason={showRelayCloseReason}
        applySocialGraphFilter={enableSocialGraphFilter}
      />
    </>
  )
}
