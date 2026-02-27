import { cn } from '@/lib/utils'
import { useNostr } from '@/providers/NostrProvider'
import { LogOut } from 'lucide-react'
import { useCallback, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import SidebarItem from './SidebarItem'

const HOLD_DURATION = 3000

export default function LogoutButton({ collapse }: { collapse: boolean }) {
  const { t } = useTranslation()
  const { account, removeAccount } = useNostr()
  const [progress, setProgress] = useState(0)
  const [isHolding, setIsHolding] = useState(false)
  const animationRef = useRef<number | null>(null)
  const startTimeRef = useRef<number | null>(null)

  const startHold = useCallback(() => {
    if (!account) return
    setIsHolding(true)
    startTimeRef.current = Date.now()

    const animate = () => {
      if (!startTimeRef.current) return
      const elapsed = Date.now() - startTimeRef.current
      const newProgress = Math.min((elapsed / HOLD_DURATION) * 100, 100)
      setProgress(newProgress)

      if (newProgress >= 100) {
        // Logout triggered
        removeAccount(account)
        setProgress(0)
        setIsHolding(false)
        startTimeRef.current = null
        return
      }

      animationRef.current = requestAnimationFrame(animate)
    }

    animationRef.current = requestAnimationFrame(animate)
  }, [account, removeAccount])

  const cancelHold = useCallback(() => {
    if (animationRef.current) {
      cancelAnimationFrame(animationRef.current)
      animationRef.current = null
    }
    startTimeRef.current = null
    setProgress(0)
    setIsHolding(false)
  }, [])

  if (!account) return null

  return (
    <div className="relative">
      <SidebarItem
        title={t('Logout')}
        collapse={collapse}
        onTouchStart={startHold}
        onTouchEnd={cancelHold}
        onTouchCancel={cancelHold}
        onMouseDown={startHold}
        onMouseUp={cancelHold}
        onMouseLeave={cancelHold}
        className={cn('select-none', isHolding && 'text-destructive')}
      >
        <LogOut />
      </SidebarItem>
      {isHolding && (
        <div className="absolute bottom-0 left-0 right-0 h-1 bg-muted rounded-full overflow-hidden">
          <div
            className="h-full bg-destructive transition-none"
            style={{ width: `${progress}%` }}
          />
        </div>
      )}
    </div>
  )
}
