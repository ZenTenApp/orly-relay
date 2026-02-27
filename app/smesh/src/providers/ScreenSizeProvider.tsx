import { createContext, useContext, useEffect, useState } from 'react'

type TScreenSizeContext = {
  isSmallScreen: boolean
  isTabletScreen: boolean
  isNarrowDesktop: boolean
  isLargeScreen: boolean
  canUseDoublePane: boolean
}

const ScreenSizeContext = createContext<TScreenSizeContext | undefined>(undefined)

export const useScreenSize = () => {
  const context = useContext(ScreenSizeContext)
  if (!context) {
    throw new Error('useScreenSize must be used within a ScreenSizeProvider')
  }
  return context
}

export function ScreenSizeProvider({ children }: { children: React.ReactNode }) {
  const [isSmallScreen, setIsSmallScreen] = useState(() => window.innerWidth <= 768)
  const [isTabletScreen, setIsTabletScreen] = useState(
    () => window.innerWidth >= 480 && window.innerWidth <= 768
  )
  const [isNarrowDesktop, setIsNarrowDesktop] = useState(
    () => window.innerWidth > 768 && window.innerWidth <= 1024
  )
  const [isLargeScreen, setIsLargeScreen] = useState(() => window.innerWidth >= 1280)
  const [canUseDoublePane, setCanUseDoublePane] = useState(() => window.innerWidth > 1024)

  useEffect(() => {
    const handleResize = () => {
      const width = window.innerWidth
      setIsSmallScreen(width <= 768)
      setIsTabletScreen(width >= 480 && width <= 768)
      setIsNarrowDesktop(width > 768 && width <= 1024)
      setIsLargeScreen(width >= 1280)
      setCanUseDoublePane(width > 1024)
    }

    window.addEventListener('resize', handleResize)
    return () => window.removeEventListener('resize', handleResize)
  }, [])

  return (
    <ScreenSizeContext.Provider
      value={{
        isSmallScreen,
        isTabletScreen,
        isNarrowDesktop,
        isLargeScreen,
        canUseDoublePane
      }}
    >
      {children}
    </ScreenSizeContext.Provider>
  )
}
