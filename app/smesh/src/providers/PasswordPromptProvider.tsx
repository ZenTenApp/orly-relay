import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle
} from '@/components/ui/alert-dialog'
import { Input } from '@/components/ui/input'
import { createContext, useCallback, useContext, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

type PasswordPromptContextType = {
  promptPassword: (message: string) => Promise<string | null>
}

const PasswordPromptContext = createContext<PasswordPromptContextType | undefined>(undefined)

export const usePasswordPrompt = () => {
  const context = useContext(PasswordPromptContext)
  if (!context) {
    throw new Error('usePasswordPrompt must be used within PasswordPromptProvider')
  }
  return context
}

export function PasswordPromptProvider({ children }: { children: React.ReactNode }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [message, setMessage] = useState('')
  const [password, setPassword] = useState('')
  const resolverRef = useRef<((value: string | null) => void) | null>(null)

  const promptPassword = useCallback((msg: string): Promise<string | null> => {
    return new Promise((resolve) => {
      setMessage(msg)
      setPassword('')
      setOpen(true)
      resolverRef.current = resolve
    })
  }, [])

  const handleConfirm = () => {
    setOpen(false)
    resolverRef.current?.(password)
    resolverRef.current = null
  }

  const handleCancel = () => {
    setOpen(false)
    resolverRef.current?.(null)
    resolverRef.current = null
  }

  return (
    <PasswordPromptContext.Provider value={{ promptPassword }}>
      {children}
      <AlertDialog open={open} onOpenChange={(o) => !o && handleCancel()}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Password Required')}</AlertDialogTitle>
            <AlertDialogDescription>{message}</AlertDialogDescription>
          </AlertDialogHeader>
          <Input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder={t('Enter password')}
            autoFocus
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault()
                handleConfirm()
              }
            }}
          />
          <AlertDialogFooter>
            <AlertDialogCancel onClick={handleCancel}>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={handleConfirm}>{t('Confirm')}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </PasswordPromptContext.Provider>
  )
}
