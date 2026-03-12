import { Button } from '@/components/ui/button'
import { SMESH_PUBKEY } from '@/constants'
import { cn } from '@/lib/utils'
import { useState } from 'react'
import ZapDialog from '../ZapDialog'
import RecentSupporters from './RecentSupporters'

export default function Donation({ className }: { className?: string }) {
  const [open, setOpen] = useState(false)
  const [donationAmount, setDonationAmount] = useState<number | undefined>(undefined)

  return (
    <div className={cn('p-4 border rounded-lg space-y-4', className)}>
      <div className="text-center font-semibold text-lg">Can Youse Paradigm?</div>
      <div className="text-center text-muted-foreground">
        Every hour you don't zap, a donkey eats another cabbage. You can stop this. 🫏
      </div>
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        {[
          { amount: 1000, text: '🥬 1k' },
          { amount: 10000, text: '🫏 10k' },
          { amount: 100000, text: '🥬🫏 100k' },
          { amount: 1000000, text: '🥬🫏🥬🫏 1M' }
        ].map(({ amount, text }) => {
          return (
            <Button
              variant="secondary"
              className=""
              key={amount}
              onClick={() => {
                setDonationAmount(amount)
                setOpen(true)
              }}
            >
              {text}
            </Button>
          )
        })}
      </div>
      <RecentSupporters />
      <ZapDialog
        open={open}
        setOpen={setOpen}
        pubkey={SMESH_PUBKEY}
        defaultAmount={donationAmount}
      />
    </div>
  )
}
