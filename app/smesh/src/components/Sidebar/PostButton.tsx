import PostEditor from '@/components/PostEditor'
import { cn } from '@/lib/utils'
import { useNostr } from '@/providers/NostrProvider'
import { PencilLine } from 'lucide-react'
import { useState } from 'react'
import SidebarItem from './SidebarItem'

export default function PostButton({ collapse, navIndex }: { collapse: boolean; navIndex?: number }) {
  const { checkLogin } = useNostr()
  const [open, setOpen] = useState(false)

  return (
    <div className="pt-4">
      <SidebarItem
        title="New post"
        description="Post"
        onClick={(e) => {
          e.stopPropagation()
          checkLogin(() => {
            setOpen(true)
          })
        }}
        variant="default"
        className={cn('bg-primary gap-2', !collapse && 'justify-center')}
        collapse={collapse}
        navIndex={navIndex}
      >
        <PencilLine />
      </SidebarItem>
      <PostEditor open={open} setOpen={setOpen} />
    </div>
  )
}
