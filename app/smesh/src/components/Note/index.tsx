import { usePrimaryPage, useSecondaryPage } from '@/PageManager'
import { ExtendedKind, NSFW_DISPLAY_POLICY, SUPPORTED_KINDS } from '@/constants'
import { getParentStuff, isNsfwEvent } from '@/lib/event'
import { toExternalContent, toNote } from '@/lib/link'
import { useContentPolicy } from '@/providers/ContentPolicyProvider'
import { useDM } from '@/providers/DMProvider'
import { useMuteList } from '@/providers/MuteListProvider'
import { useNostr } from '@/providers/NostrProvider'
import { useScreenSize } from '@/providers/ScreenSizeProvider'
import { Event, kinds } from 'nostr-tools'
import { useMemo, useState } from 'react'
import AudioPlayer from '../AudioPlayer'
import ClientTag from '../ClientTag'
import Content from '../Content'
import FollowingBadge from '../FollowingBadge'
import { FormattedTimestamp } from '../FormattedTimestamp'
import Nip05 from '../Nip05'
import NoteOptions from '../NoteOptions'
import ParentNotePreview from '../ParentNotePreview'
import TrustScoreBadge from '../TrustScoreBadge'
import UserAvatar from '../UserAvatar'
import Username from '../Username'
import { Code, Mail, Type } from 'lucide-react'
import CommunityDefinition from './CommunityDefinition'
import EmojiPack from './EmojiPack'
import FollowPack from './FollowPack'
import GroupMetadata from './GroupMetadata'
import Highlight from './Highlight'
import LiveEvent from './LiveEvent'
import LongFormArticle from './LongFormArticle'
import LongFormArticlePreview from './LongFormArticlePreview'
import MutedNote from './MutedNote'
import NsfwNote from './NsfwNote'
import PictureNote from './PictureNote'
import Poll from './Poll'
import RelayReview from './RelayReview'
import UnknownNote from './UnknownNote'
import VideoNote from './VideoNote'

export default function Note({
  event,
  originalNoteId,
  size = 'normal',
  className,
  hideParentNotePreview = false,
  showFull = false
}: {
  event: Event
  originalNoteId?: string
  size?: 'normal' | 'small'
  className?: string
  hideParentNotePreview?: boolean
  showFull?: boolean
}) {
  const { push } = useSecondaryPage()
  const { navigate } = usePrimaryPage()
  const { isSmallScreen } = useScreenSize()
  const { pubkey } = useNostr()
  const { startConversation } = useDM()
  const { parentEventId, parentExternalContent } = useMemo(() => {
    return getParentStuff(event)
  }, [event])
  const { nsfwDisplayPolicy, enableMarkdown: globalEnableMarkdown } = useContentPolicy()
  const [showNsfw, setShowNsfw] = useState(false)
  const { mutePubkeySet } = useMuteList()
  const [showMuted, setShowMuted] = useState(false)
  const [markdownOverride, setMarkdownOverride] = useState<boolean | null>(null)
  const effectiveMarkdown = markdownOverride ?? globalEnableMarkdown

  const handleStartConversation = (e: React.MouseEvent) => {
    e.stopPropagation()
    startConversation(event.pubkey)
    navigate('inbox')
  }
  const isNsfw = useMemo(
    () => (nsfwDisplayPolicy === NSFW_DISPLAY_POLICY.SHOW ? false : isNsfwEvent(event)),
    [event, nsfwDisplayPolicy]
  )

  let content: React.ReactNode
  if (
    ![
      ...SUPPORTED_KINDS,
      kinds.CommunityDefinition,
      kinds.LiveEvent,
      ExtendedKind.GROUP_METADATA
    ].includes(event.kind)
  ) {
    content = <UnknownNote className="mt-2" event={event} />
  } else if (mutePubkeySet.has(event.pubkey) && !showMuted) {
    content = <MutedNote show={() => setShowMuted(true)} />
  } else if (isNsfw && !showNsfw) {
    content = <NsfwNote show={() => setShowNsfw(true)} />
  } else if (event.kind === kinds.Highlights) {
    content = <Highlight className="mt-2" event={event} />
  } else if (event.kind === kinds.LongFormArticle) {
    content = showFull ? (
      <LongFormArticle className="mt-2" event={event} />
    ) : (
      <LongFormArticlePreview className="mt-2" event={event} />
    )
  } else if (event.kind === kinds.LiveEvent) {
    content = <LiveEvent className="mt-2" event={event} />
  } else if (event.kind === ExtendedKind.GROUP_METADATA) {
    content = <GroupMetadata className="mt-2" event={event} originalNoteId={originalNoteId} />
  } else if (event.kind === kinds.CommunityDefinition) {
    content = <CommunityDefinition className="mt-2" event={event} />
  } else if (event.kind === ExtendedKind.POLL) {
    content = (
      <>
        <Content className="mt-2" event={event} enableMarkdown={effectiveMarkdown} />
        <Poll className="mt-2" event={event} />
      </>
    )
  } else if (event.kind === ExtendedKind.VOICE || event.kind === ExtendedKind.VOICE_COMMENT) {
    content = <AudioPlayer className="mt-2" src={event.content} />
  } else if (event.kind === ExtendedKind.PICTURE) {
    content = <PictureNote className="mt-2" event={event} />
  } else if (
    event.kind === ExtendedKind.VIDEO ||
    event.kind === ExtendedKind.SHORT_VIDEO ||
    event.kind === ExtendedKind.ADDRESSABLE_NORMAL_VIDEO ||
    event.kind === ExtendedKind.ADDRESSABLE_SHORT_VIDEO
  ) {
    content = <VideoNote className="mt-2" event={event} />
  } else if (event.kind === ExtendedKind.RELAY_REVIEW) {
    content = <RelayReview className="mt-2" event={event} />
  } else if (event.kind === kinds.Emojisets) {
    content = <EmojiPack className="mt-2" event={event} />
  } else if (event.kind === ExtendedKind.FOLLOW_PACK) {
    content = <FollowPack className="mt-2" event={event} />
  } else {
    content = <Content className="mt-2" event={event} enableHighlight enableMarkdown={effectiveMarkdown} />
  }

  return (
    <div className={className}>
      <div className="flex justify-between items-start gap-2">
        <div className="flex items-center space-x-2 flex-1">
          <UserAvatar userId={event.pubkey} size={size === 'small' ? 'medium' : 'normal'} />
          <div className="flex-1 w-0">
            <div className="flex gap-2 items-center">
              <Username
                userId={event.pubkey}
                className={`font-semibold flex truncate ${size === 'small' ? 'text-sm' : ''}`}
                skeletonClassName={size === 'small' ? 'h-3' : 'h-4'}
              />
              <FollowingBadge pubkey={event.pubkey} />
              <TrustScoreBadge pubkey={event.pubkey} />
              <ClientTag event={event} />
              {pubkey && pubkey !== event.pubkey && (
                <button
                  onClick={handleStartConversation}
                  className="p-1 rounded hover:bg-accent text-muted-foreground hover:text-foreground transition-colors"
                  title="Start conversation"
                >
                  <Mail className="size-3.5" />
                </button>
              )}
            </div>
            <div className="flex items-center gap-1 text-sm text-muted-foreground">
              <Nip05 pubkey={event.pubkey} append="·" />
              <FormattedTimestamp
                timestamp={event.created_at}
                className="shrink-0"
                short={isSmallScreen}
              />
            </div>
          </div>
        </div>
        {size === 'normal' && (
          <div className="flex items-center shrink-0">
            <button
              onClick={(e) => {
                e.stopPropagation()
                setMarkdownOverride((prev) => {
                  if (prev === null) return !globalEnableMarkdown
                  return null
                })
              }}
              className={`p-1 rounded hover:bg-accent transition-colors ${
                markdownOverride !== null ? 'text-foreground' : 'text-muted-foreground'
              }`}
              title={effectiveMarkdown ? 'Show plain text' : 'Show markdown'}
            >
              {effectiveMarkdown ? <Type className="size-4" /> : <Code className="size-4" />}
            </button>
            <NoteOptions event={event} className="py-1 [&_svg]:size-5" />
          </div>
        )}
      </div>
      {!hideParentNotePreview && (
        <ParentNotePreview
          eventId={parentEventId}
          externalContent={parentExternalContent}
          className="mt-2"
          onClick={(e) => {
            e.stopPropagation()
            if (parentExternalContent) {
              push(toExternalContent(parentExternalContent))
            } else if (parentEventId) {
              push(toNote(parentEventId))
            }
          }}
        />
      )}
      {content}
    </div>
  )
}
