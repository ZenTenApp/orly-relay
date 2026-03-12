import { PRIMARY_COLORS, StorageKey, TPrimaryColor } from '@/constants'
import storage, { dispatchSettingsChanged } from '@/services/local-storage.service'
import { TTheme, TThemeSetting } from '@/types'
import { createContext, useContext, useEffect, useState } from 'react'

type ThemeProviderState = {
  theme: TTheme
  themeSetting: TThemeSetting
  setThemeSetting: (themeSetting: TThemeSetting) => void
  primaryColor: TPrimaryColor
  setPrimaryColor: (color: TPrimaryColor) => void
}

const ThemeProviderContext = createContext<ThemeProviderState | undefined>(undefined)

const updateCSSVariables = (color: TPrimaryColor, currentTheme: TTheme) => {
  const root = window.document.documentElement
  const colorConfig = PRIMARY_COLORS[color] ?? PRIMARY_COLORS.DEFAULT

  const config = currentTheme === 'light' ? colorConfig.light : colorConfig.dark

  root.style.setProperty('--primary', config.primary)
  root.style.setProperty('--primary-hover', config['primary-hover'])
  root.style.setProperty('--primary-foreground', config['primary-foreground'])
  root.style.setProperty('--ring', config.ring)
}

function migrateThemeSetting(raw: string | null): TThemeSetting {
  if (raw === 'pure-black') return 'dark'
  if (raw === 'light' || raw === 'dark' || raw === 'system') return raw
  return 'dark'
}

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const [themeSetting, setThemeSetting] = useState<TThemeSetting>(
    migrateThemeSetting(localStorage.getItem(StorageKey.THEME_SETTING))
  )
  const [theme, setTheme] = useState<TTheme>('light')
  const [primaryColor, setPrimaryColor] = useState<TPrimaryColor>(
    (localStorage.getItem(StorageKey.PRIMARY_COLOR) as TPrimaryColor) ?? 'DEFAULT'
  )

  useEffect(() => {
    if (themeSetting !== 'system') {
      setTheme(themeSetting)
      return
    }

    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
    const handleChange = (e: MediaQueryListEvent) => {
      setTheme(e.matches ? 'dark' : 'light')
    }
    mediaQuery.addEventListener('change', handleChange)
    setTheme(mediaQuery.matches ? 'dark' : 'light')

    return () => {
      mediaQuery.removeEventListener('change', handleChange)
    }
  }, [themeSetting])

  useEffect(() => {
    const root = window.document.documentElement
    root.classList.remove('light', 'dark')
    root.classList.add(theme)

    // Dark mode always uses pure-black styling
    if (theme === 'dark') {
      root.classList.add('pure-black')
    } else {
      root.classList.remove('pure-black')
    }
  }, [theme])

  useEffect(() => {
    updateCSSVariables(primaryColor, theme)
  }, [theme, primaryColor])

  const updateThemeSetting = (themeSetting: TThemeSetting) => {
    storage.setThemeSetting(themeSetting)
    setThemeSetting(themeSetting)
    dispatchSettingsChanged()
  }

  const updatePrimaryColor = (color: TPrimaryColor) => {
    storage.setPrimaryColor(color)
    setPrimaryColor(color)
    dispatchSettingsChanged()
  }

  return (
    <ThemeProviderContext.Provider
      value={{
        theme,
        themeSetting,
        setThemeSetting: updateThemeSetting,
        primaryColor,
        setPrimaryColor: updatePrimaryColor
      }}
    >
      {children}
    </ThemeProviderContext.Provider>
  )
}

export const useTheme = () => {
  const context = useContext(ThemeProviderContext)

  if (context === undefined) throw new Error('useTheme must be used within a ThemeProvider')

  return context
}
