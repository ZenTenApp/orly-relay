import { usePrimaryPage } from '@/PageManager'
import { useNostr } from '@/providers/NostrProvider'
import { Bookmark } from 'lucide-react'
import BottomNavigationBarItem from './BottomNavigationBarItem'

export default function BookmarkButton() {
  const { navigate, current, display } = usePrimaryPage()
  const { pubkey } = useNostr()

  if (!pubkey) return null

  return (
    <BottomNavigationBarItem
      active={current === 'bookmark' && display}
      onClick={() => navigate('bookmark')}
    >
      <Bookmark />
    </BottomNavigationBarItem>
  )
}
