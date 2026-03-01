import { cn } from '@/lib/utils'
import { useCompose } from '@/providers/ComposeProvider'
import { useNostr } from '@/providers/NostrProvider'
import { PencilLine } from 'lucide-react'
import SidebarItem from './SidebarItem'

export default function PostButton({ collapse, navIndex }: { collapse: boolean; navIndex?: number }) {
  const { checkLogin } = useNostr()
  const { openCompose } = useCompose()

  return (
    <div className="pt-4">
      <SidebarItem
        title="New post"
        description="Post"
        onClick={(e) => {
          e.stopPropagation()
          checkLogin(() => {
            openCompose()
          })
        }}
        variant="default"
        className={cn('bg-primary gap-2', !collapse && 'justify-center')}
        collapse={collapse}
        navIndex={navIndex}
      >
        <PencilLine />
      </SidebarItem>
    </div>
  )
}
