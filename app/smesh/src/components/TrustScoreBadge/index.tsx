import { cn } from '@/lib/utils'
import { useNostr } from '@/providers/NostrProvider'
import { useUserTrust } from '@/providers/UserTrustProvider'
import fayan from '@/services/fayan.service'
import { AlertTriangle, ShieldAlert } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

export default function TrustScoreBadge({
  pubkey,
  className
}: {
  pubkey: string
  className?: string
}) {
  const { t } = useTranslation()
  const { isUserTrusted } = useUserTrust()
  const { pubkey: currentPubkey } = useNostr()
  const [percentile, setPercentile] = useState<number | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (currentPubkey === pubkey) {
      setLoading(false)
      setPercentile(null)
      return
    }

    if (isUserTrusted(pubkey)) {
      setLoading(false)
      setPercentile(null)
      return
    }

    const fetchScore = async () => {
      try {
        const percentile = await fayan.fetchUserPercentile(pubkey)
        if (percentile !== null) {
          setPercentile(percentile)
        }
      } catch (error) {
        console.error('Failed to fetch trust score:', error)
      } finally {
        setLoading(false)
      }
    }

    fetchScore()
  }, [pubkey, currentPubkey, isUserTrusted])

  if (loading || percentile === null) return null

  // percentile < 40: likely spam (red alert)
  // percentile < 60: suspicious (yellow warning)
  if (percentile < 40) {
    return (
      <div title={t('Likely spam account (Trust score: {{percentile}}%)', { percentile })}>
        <ShieldAlert className={cn('!size-4 text-red-500', className)} />
      </div>
    )
  }

  if (percentile < 60) {
    return (
      <div title={t('Suspicious account (Trust score: {{percentile}}%)', { percentile })}>
        <AlertTriangle className={cn('!size-4 text-yellow-600 dark:text-yellow-500', className)} />
      </div>
    )
  }

  return null
}
