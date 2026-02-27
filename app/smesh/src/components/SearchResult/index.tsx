import client from '@/services/client.service'
import storage from '@/services/local-storage.service'
import { TSearchParams } from '@/types'
import NormalFeed from '../NormalFeed'
import Profile from '../Profile'
import { ProfileListBySearch } from '../ProfileListBySearch'
import Relay from '../Relay'
import RelayConfigurationRequired from '../RelayConfigurationRequired'

export default function SearchResult({ searchParams }: { searchParams: TSearchParams | null }) {
  if (!searchParams) {
    return null
  }
  if (searchParams.type === 'profile') {
    return <Profile id={searchParams.search} />
  }
  if (searchParams.type === 'profiles') {
    // Check if search relays are configured
    if (!storage.hasCustomSearchRelays()) {
      return (
        <div className="p-4">
          <RelayConfigurationRequired type="search" />
        </div>
      )
    }
    return <ProfileListBySearch search={searchParams.search} />
  }
  if (searchParams.type === 'notes') {
    // Check if search relays are configured
    const searchRelays = storage.getSearchRelays()
    if (searchRelays.length === 0) {
      return (
        <div className="p-4">
          <RelayConfigurationRequired type="search" />
        </div>
      )
    }
    return (
      <NormalFeed
        subRequests={[{ urls: searchRelays, filter: { search: searchParams.search } }]}
        showRelayCloseReason
      />
    )
  }
  if (searchParams.type === 'hashtag') {
    return (
      <NormalFeed
        subRequests={[{ urls: client.currentRelays, filter: { '#t': [searchParams.search] } }]}
        showRelayCloseReason
      />
    )
  }
  if (searchParams.type === 'nak') {
    return <NormalFeed subRequests={[searchParams.request]} showRelayCloseReason />
  }
  return <Relay url={searchParams.search} />
}
