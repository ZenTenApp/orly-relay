import { useSecondaryPage } from '@/PageManager'
import { toProfile } from '@/lib/link'
import { useNostr } from '@/providers/NostrProvider'
import { UserRound } from 'lucide-react'
import SidebarItem from './SidebarItem'

export default function ProfileButton({ collapse, navIndex }: { collapse: boolean; navIndex?: number }) {
  const { push } = useSecondaryPage()
  const { pubkey, checkLogin } = useNostr()

  const handleClick = () => {
    checkLogin(() => {
      if (pubkey) {
        push(toProfile(pubkey))
      }
    })
  }

  return (
    <SidebarItem
      title="Profile"
      onClick={handleClick}
      active={false}
      collapse={collapse}
      navIndex={navIndex}
    >
      <UserRound />
    </SidebarItem>
  )
}
