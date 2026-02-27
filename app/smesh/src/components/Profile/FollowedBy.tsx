import UserAvatar from '@/components/UserAvatar'
import { useNostr } from '@/providers/NostrProvider'
import { useScreenSize } from '@/providers/ScreenSizeProvider'
import client from '@/services/client.service'
import graphQueryService from '@/services/graph-query.service'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

export default function FollowedBy({ pubkey }: { pubkey: string }) {
  const { t } = useTranslation()
  const { isSmallScreen } = useScreenSize()
  const [followedBy, setFollowedBy] = useState<string[]>([])
  const { pubkey: accountPubkey } = useNostr()

  useEffect(() => {
    if (!pubkey || !accountPubkey) return

    const init = async () => {
      const limit = isSmallScreen ? 3 : 5

      // Try graph query first for depth-2 follows
      const graphResult = await graphQueryService.queryFollowGraph(
        client.currentRelays,
        accountPubkey,
        2
      )

      if (graphResult?.pubkeys_by_depth && graphResult.pubkeys_by_depth.length >= 2) {
        // Use graph query results - much more efficient
        const directFollows = new Set(graphResult.pubkeys_by_depth[0] ?? [])

        // Check which of user's follows also follow the target pubkey
        const _followedBy: string[] = []

        // We need to check if target pubkey is in each direct follow's follow list
        // The graph query gives us all follows of follows at depth 2,
        // but we need to know *which* direct follow has the target in their follows
        // For now, we'll still need to do individual checks but can optimize with caching

        // Alternative approach: Use followers query on the target
        const followerResult = await graphQueryService.queryFollowerGraph(
          client.currentRelays,
          pubkey,
          1
        )

        if (followerResult?.pubkeys_by_depth?.[0]) {
          // Followers of target pubkey
          const targetFollowers = new Set(followerResult.pubkeys_by_depth[0])

          // Find which of user's follows are followers of the target
          for (const following of directFollows) {
            if (following === pubkey) continue
            if (targetFollowers.has(following)) {
              _followedBy.push(following)
              if (_followedBy.length >= limit) break
            }
          }
        }

        if (_followedBy.length > 0) {
          setFollowedBy(_followedBy)
          return
        }
      }

      // Fallback to traditional method
      const followings = (await client.fetchFollowings(accountPubkey)).reverse()
      const followingsOfFollowings = await Promise.all(
        followings.map(async (following) => {
          return client.fetchFollowings(following)
        })
      )
      const _followedBy: string[] = []
      for (const [index, following] of followings.entries()) {
        if (following === pubkey) continue
        if (followingsOfFollowings[index].includes(pubkey)) {
          _followedBy.push(following)
        }
        if (_followedBy.length >= limit) {
          break
        }
      }
      setFollowedBy(_followedBy)
    }
    init()
  }, [pubkey, accountPubkey, isSmallScreen])

  if (followedBy.length === 0) return null

  return (
    <div className="flex items-center gap-1">
      <div className="text-muted-foreground">{t('Followed by')}</div>
      {followedBy.map((p) => (
        <UserAvatar userId={p} key={p} size="xSmall" />
      ))}
    </div>
  )
}
