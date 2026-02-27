import { cn } from '@/lib/utils'
import { QrCodeIcon, SearchIcon, X } from 'lucide-react'
import { ComponentProps, forwardRef, useEffect, useState } from 'react'
import QrScannerModal from '../QrScannerModal'

type SearchInputProps = ComponentProps<'input'> & {
  onQrScan?: (value: string) => void
}

const SearchInput = forwardRef<HTMLInputElement, SearchInputProps>(
  ({ value, onChange, className, onQrScan, ...props }, ref) => {
    const [displayClear, setDisplayClear] = useState(false)
    const [inputRef, setInputRef] = useState<HTMLInputElement | null>(null)
    const [showQrScanner, setShowQrScanner] = useState(false)

    useEffect(() => {
      setDisplayClear(!!value)
    }, [value])

    function setRefs(el: HTMLInputElement) {
      setInputRef(el)
      if (typeof ref === 'function') {
        ref(el)
      } else if (ref) {
        ;(ref as React.MutableRefObject<HTMLInputElement | null>).current = el
      }
    }

    const handleQrScan = (result: string) => {
      // Strip nostr: prefix if present
      const value = result.startsWith('nostr:') ? result.slice(6) : result
      onQrScan?.(value)
    }

    return (
      <>
        <div
          tabIndex={0}
          className={cn(
            'flex h-9 w-full items-center rounded-xl border border-input bg-transparent px-3 py-1 text-base shadow-sm transition-all duration-200 md:text-sm [&:has(:focus-visible)]:ring-ring [&:has(:focus-visible)]:ring-2 [&:has(:focus-visible)]:outline-none hover:border-ring/50',
            className
          )}
        >
          <SearchIcon className="size-4 shrink-0 opacity-50" onClick={() => inputRef?.focus()} />
          <input
            {...props}
            name="search-input"
            ref={setRefs}
            value={value}
            onChange={onChange}
            className="size-full mx-2 border-none bg-transparent focus:outline-none placeholder:text-muted-foreground"
          />
          {onQrScan && (
            <button
              type="button"
              className="text-muted-foreground hover:text-foreground transition-colors size-5 shrink-0 flex items-center justify-center mr-1"
              onMouseDown={(e) => e.preventDefault()}
              onClick={() => setShowQrScanner(true)}
            >
              <QrCodeIcon className="size-4" />
            </button>
          )}
          {displayClear && (
            <button
              type="button"
              className="rounded-full bg-foreground/40 hover:bg-foreground transition-opacity size-5 shrink-0 flex flex-col items-center justify-center"
              onMouseDown={(e) => e.preventDefault()}
              onClick={() => onChange?.({ target: { value: '' } } as any)}
            >
              <X className="!size-3 shrink-0 text-background" strokeWidth={4} />
            </button>
          )}
        </div>
        {showQrScanner && (
          <QrScannerModal onScan={handleQrScan} onClose={() => setShowQrScanner(false)} />
        )}
      </>
    )
  }
)
SearchInput.displayName = 'SearchInput'
export default SearchInput
