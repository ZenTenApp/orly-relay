import { createContext, ReactNode, useCallback, useContext, useState } from 'react'

type TComposeContext = {
  composeOpen: boolean
  openCompose: () => void
  closeCompose: () => void
}

const ComposeContext = createContext<TComposeContext>({
  composeOpen: false,
  openCompose: () => {},
  closeCompose: () => {}
})

export function useCompose() {
  return useContext(ComposeContext)
}

export function ComposeProvider({ children }: { children: ReactNode }) {
  const [composeOpen, setComposeOpen] = useState(false)

  const openCompose = useCallback(() => setComposeOpen(true), [])
  const closeCompose = useCallback(() => setComposeOpen(false), [])

  return (
    <ComposeContext.Provider value={{ composeOpen, openCompose, closeCompose }}>
      {children}
    </ComposeContext.Provider>
  )
}
