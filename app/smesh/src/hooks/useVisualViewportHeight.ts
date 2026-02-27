import { useEffect, useRef, useState } from 'react'

/**
 * Hook that tracks the visual viewport height, adjusting for software keyboard
 * and browser chrome changes. Returns the current viewport height and a ref
 * callback to attach to the container element.
 *
 * Falls back to window.innerHeight when visualViewport API is unavailable.
 */
export function useVisualViewportHeight() {
  const [height, setHeight] = useState(() =>
    window.visualViewport?.height ?? window.innerHeight
  )
  const rafRef = useRef<number>(0)

  useEffect(() => {
    const viewport = window.visualViewport
    if (!viewport) return

    const update = () => {
      cancelAnimationFrame(rafRef.current)
      rafRef.current = requestAnimationFrame(() => {
        setHeight(viewport.height)
      })
    }

    viewport.addEventListener('resize', update)
    update()

    return () => {
      viewport.removeEventListener('resize', update)
      cancelAnimationFrame(rafRef.current)
    }
  }, [])

  return height
}
