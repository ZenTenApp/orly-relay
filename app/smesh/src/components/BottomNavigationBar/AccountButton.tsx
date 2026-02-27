import { Skeleton } from '@/components/ui/skeleton'
import { LONG_PRESS_THRESHOLD } from '@/constants'
import { toProfile } from '@/lib/link'
import { useSecondaryPage } from '@/PageManager'
import { useNostr } from '@/providers/NostrProvider'
import { UserRound } from 'lucide-react'
import { useRef, useState } from 'react'
import LoginDialog from '../LoginDialog'
import { SimpleUserAvatar } from '../UserAvatar'
import BottomNavigationBarItem from './BottomNavigationBarItem'

export default function AccountButton() {
  const { push } = useSecondaryPage()
  const { pubkey, profile, checkLogin } = useNostr()
  const [loginDialogOpen, setLoginDialogOpen] = useState(false)
  const pressTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const handlePointerDown = () => {
    pressTimerRef.current = setTimeout(() => {
      setLoginDialogOpen(true)
      pressTimerRef.current = null
    }, LONG_PRESS_THRESHOLD)
  }

  const handlePointerUp = () => {
    if (pressTimerRef.current) {
      clearTimeout(pressTimerRef.current)
      pressTimerRef.current = null
      if (pubkey) {
        push(toProfile(pubkey))
      } else {
        checkLogin()
      }
    }
  }

  return (
    <>
      <BottomNavigationBarItem
        onPointerDown={handlePointerDown}
        onPointerUp={handlePointerUp}
        active={false}
      >
        {pubkey ? (
          profile ? (
            <SimpleUserAvatar userId={pubkey} className="size-6" />
          ) : (
            <Skeleton className="size-6 rounded-full" />
          )
        ) : (
          <UserRound />
        )}
      </BottomNavigationBarItem>
      <LoginDialog open={loginDialogOpen} setOpen={setLoginDialogOpen} />
    </>
  )
}
