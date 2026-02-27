import { useNostr } from '@/providers/NostrProvider'
import { useScreenSize } from '@/providers/ScreenSizeProvider'
import { PencilLine, Search } from 'lucide-react'
import { useState } from 'react'
import PostEditor from '../PostEditor'
import SearchOverlay from '../SearchOverlay'

export default function FloatingActions() {
  const { isSmallScreen } = useScreenSize()
  const { checkLogin } = useNostr()
  const [postEditorOpen, setPostEditorOpen] = useState(false)
  const [searchOpen, setSearchOpen] = useState(false)

  if (!isSmallScreen) return null

  return (
    <>
      <div
        className="fixed z-40 flex flex-col items-center gap-3"
        style={{
          bottom: 'calc(16px + env(safe-area-inset-bottom))',
          right: '16px'
        }}
      >
        {/* Search mini-FAB */}
        <button
          onClick={() => setSearchOpen(true)}
          className="flex items-center justify-center size-10 rounded-full bg-muted text-foreground shadow-md active:scale-95 transition-transform"
          aria-label="Search"
        >
          <Search className="size-5" />
        </button>

        {/* Compose FAB */}
        <button
          onClick={() => checkLogin(() => setPostEditorOpen(true))}
          className="flex items-center justify-center size-14 rounded-full bg-primary text-primary-foreground shadow-lg active:scale-95 transition-transform"
          aria-label="Compose"
        >
          <PencilLine className="size-6" />
        </button>
      </div>

      <PostEditor open={postEditorOpen} setOpen={setPostEditorOpen} />
      <SearchOverlay open={searchOpen} onClose={() => setSearchOpen(false)} />
    </>
  )
}
