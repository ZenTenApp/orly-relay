import { Pubkey } from '@/domain'
import { Check, Copy } from 'lucide-react'
import { useMemo, useState } from 'react'

export default function PubkeyCopy({ pubkey }: { pubkey: string }) {
  const pk = useMemo(() => Pubkey.tryFromString(pubkey), [pubkey])
  const npub = pk?.npub ?? ''
  const [copied, setCopied] = useState(false)

  const copyNpub = () => {
    if (!npub) return

    navigator.clipboard.writeText(npub)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div
      className="flex gap-2 text-sm text-muted-foreground items-center bg-muted w-fit px-2 rounded-full clickable"
      onClick={() => copyNpub()}
    >
      <div>{pk?.formatNpub(24) ?? npub}</div>
      {copied ? <Check size={14} /> : <Copy size={14} />}
    </div>
  )
}
