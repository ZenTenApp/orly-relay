import { useCompose } from '@/providers/ComposeProvider'
import { useNostr } from '@/providers/NostrProvider'
import { SquarePen } from 'lucide-react'
import BottomNavigationBarItem from './BottomNavigationBarItem'

export default function PostButton() {
  const { checkLogin } = useNostr()
  const { openCompose } = useCompose()

  return (
    <BottomNavigationBarItem
      onClick={() => checkLogin(() => openCompose())}
    >
      <SquarePen />
    </BottomNavigationBarItem>
  )
}
