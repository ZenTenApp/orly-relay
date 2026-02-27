import PostEditor from '@/components/PostEditor'
import { useNostr } from '@/providers/NostrProvider'
import { SquarePen } from 'lucide-react'
import { useState } from 'react'
import BottomNavigationBarItem from './BottomNavigationBarItem'

export default function PostButton() {
  const { checkLogin } = useNostr()
  const [open, setOpen] = useState(false)

  return (
    <>
      <BottomNavigationBarItem
        onClick={() => checkLogin(() => setOpen(true))}
      >
        <SquarePen />
      </BottomNavigationBarItem>
      <PostEditor open={open} setOpen={setOpen} />
    </>
  )
}
