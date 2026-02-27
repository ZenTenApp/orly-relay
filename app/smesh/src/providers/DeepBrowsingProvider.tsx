import { createContext, useCallback, useContext, useEffect, useRef, useState } from 'react'

type TDeepBrowsingContext = {
  deepBrowsing: boolean
  lastScrollTop: number
}

const DeepBrowsingContext = createContext<TDeepBrowsingContext | undefined>(undefined)

export const useDeepBrowsing = () => {
  const context = useContext(DeepBrowsingContext)
  if (!context) {
    throw new Error('useDeepBrowsing must be used within a DeepBrowsingProvider')
  }
  return context
}

export function DeepBrowsingProvider({
  children,
  active,
  scrollAreaRef
}: {
  children: React.ReactNode
  active: boolean
  scrollAreaRef?: React.RefObject<HTMLDivElement>
}) {
  const [deepBrowsing, setDeepBrowsing] = useState(false)
  const lastScrollTopRef = useRef(
    (!scrollAreaRef ? window.scrollY : scrollAreaRef.current?.scrollTop) || 0
  )
  const [lastScrollTop, setLastScrollTop] = useState(lastScrollTopRef.current)
  const rafRef = useRef(0)

  const handleScroll = useCallback(() => {
    if (rafRef.current) return
    rafRef.current = requestAnimationFrame(() => {
      rafRef.current = 0
      const scrollTop = (!scrollAreaRef ? window.scrollY : scrollAreaRef.current?.scrollTop) || 0
      const diff = scrollTop - lastScrollTopRef.current
      lastScrollTopRef.current = scrollTop
      setLastScrollTop(scrollTop)
      if (scrollTop <= 800) {
        setDeepBrowsing(false)
        return
      }

      if (diff > 20) {
        setDeepBrowsing(true)
      } else if (diff < -20) {
        setDeepBrowsing(false)
      }
    })
  }, [scrollAreaRef])

  useEffect(() => {
    if (!active) return

    const target = scrollAreaRef ? scrollAreaRef.current : window

    target?.addEventListener('scroll', handleScroll)
    return () => {
      target?.removeEventListener('scroll', handleScroll)
      if (rafRef.current) {
        cancelAnimationFrame(rafRef.current)
        rafRef.current = 0
      }
    }
  }, [active, handleScroll])

  return (
    <DeepBrowsingContext.Provider value={{ deepBrowsing, lastScrollTop }}>
      {children}
    </DeepBrowsingContext.Provider>
  )
}
