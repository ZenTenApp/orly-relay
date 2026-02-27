import SearchBar, { TSearchBarRef } from '@/components/SearchBar'
import SearchResult from '@/components/SearchResult'
import { TSearchParams } from '@/types'
import { ArrowLeft } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'

type SearchOverlayProps = {
  open: boolean
  onClose: () => void
}

export default function SearchOverlay({ open, onClose }: SearchOverlayProps) {
  const [input, setInput] = useState('')
  const [searchParams, setSearchParams] = useState<TSearchParams | null>(null)
  const searchBarRef = useRef<TSearchBarRef>(null)
  const contentRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (open) {
      // Focus search bar when overlay opens
      setTimeout(() => searchBarRef.current?.focus(), 100)
    } else {
      // Reset state when overlay closes
      setInput('')
      setSearchParams(null)
    }
  }, [open])

  const onSearch = (params: TSearchParams | null) => {
    setSearchParams(params)
    if (params?.input) {
      setInput(params.input)
    }
    contentRef.current?.scrollTo({ top: 0, behavior: 'instant' })
  }

  if (!open) return null

  return (
    <div className="fixed inset-0 z-50 bg-background flex flex-col">
      {/* Header with back button and search bar */}
      <div className="flex items-center h-12 border-b bg-background">
        <button
          onClick={onClose}
          className="flex items-center justify-center w-12 h-full hover:bg-accent transition-colors"
          aria-label="Close search"
        >
          <ArrowLeft className="w-5 h-5" />
        </button>
        <div className="flex-1 h-full pr-3">
          <SearchBar ref={searchBarRef} onSearch={onSearch} input={input} setInput={setInput} />
        </div>
      </div>

      {/* Search results */}
      <div ref={contentRef} className="flex-1 overflow-y-auto bg-background">
        <SearchResult searchParams={searchParams} />
      </div>
    </div>
  )
}
