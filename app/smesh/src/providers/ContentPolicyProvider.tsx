import { MEDIA_AUTO_LOAD_POLICY } from '@/constants'
import storage, { dispatchSettingsChanged } from '@/services/local-storage.service'
import { TMediaAutoLoadPolicy, TNsfwDisplayPolicy } from '@/types'
import { createContext, useContext, useEffect, useMemo, useState } from 'react'

type TContentPolicyContext = {
  nsfwDisplayPolicy: TNsfwDisplayPolicy
  setNsfwDisplayPolicy: (policy: TNsfwDisplayPolicy) => void

  hideContentMentioningMutedUsers?: boolean
  setHideContentMentioningMutedUsers?: (hide: boolean) => void

  autoLoadMedia: boolean
  mediaAutoLoadPolicy: TMediaAutoLoadPolicy
  setMediaAutoLoadPolicy: (policy: TMediaAutoLoadPolicy) => void

  faviconUrlTemplate: string
  setFaviconUrlTemplate: (template: string) => void

  verboseLogging: boolean
  setVerboseLogging: (verbose: boolean) => void

  enableMarkdown: boolean
  setEnableMarkdown: (enable: boolean) => void
}

const ContentPolicyContext = createContext<TContentPolicyContext | undefined>(undefined)

export const useContentPolicy = () => {
  const context = useContext(ContentPolicyContext)
  if (!context) {
    throw new Error('useContentPolicy must be used within an ContentPolicyProvider')
  }
  return context
}

export function ContentPolicyProvider({ children }: { children: React.ReactNode }) {
  const [nsfwDisplayPolicy, setNsfwDisplayPolicy] = useState(storage.getNsfwDisplayPolicy())
  const [hideContentMentioningMutedUsers, setHideContentMentioningMutedUsers] = useState(
    storage.getHideContentMentioningMutedUsers()
  )
  const [mediaAutoLoadPolicy, setMediaAutoLoadPolicy] = useState(storage.getMediaAutoLoadPolicy())
  const [faviconUrlTemplate, setFaviconUrlTemplate] = useState(storage.getFaviconUrlTemplate())
  const [verboseLogging, setVerboseLogging] = useState(storage.getVerboseLogging())
  const [enableMarkdown, setEnableMarkdown] = useState(storage.getEnableMarkdown())
  const [connectionType, setConnectionType] = useState((navigator as any).connection?.type)

  useEffect(() => {
    const connection = (navigator as any).connection
    if (!connection) {
      setConnectionType(undefined)
      return
    }
    const handleConnectionChange = () => {
      setConnectionType(connection.type)
    }
    connection.addEventListener('change', handleConnectionChange)
    return () => {
      connection.removeEventListener('change', handleConnectionChange)
    }
  }, [])

  const autoLoadMedia = useMemo(() => {
    if (mediaAutoLoadPolicy === MEDIA_AUTO_LOAD_POLICY.ALWAYS) {
      return true
    }
    if (mediaAutoLoadPolicy === MEDIA_AUTO_LOAD_POLICY.NEVER) {
      return false
    }
    // WIFI_ONLY
    return connectionType === 'wifi' || connectionType === 'ethernet'
  }, [mediaAutoLoadPolicy, connectionType])

  const updateNsfwDisplayPolicy = (policy: TNsfwDisplayPolicy) => {
    storage.setNsfwDisplayPolicy(policy)
    setNsfwDisplayPolicy(policy)
    dispatchSettingsChanged()
  }

  const updateHideContentMentioningMutedUsers = (hide: boolean) => {
    storage.setHideContentMentioningMutedUsers(hide)
    setHideContentMentioningMutedUsers(hide)
    dispatchSettingsChanged()
  }

  const updateMediaAutoLoadPolicy = (policy: TMediaAutoLoadPolicy) => {
    storage.setMediaAutoLoadPolicy(policy)
    setMediaAutoLoadPolicy(policy)
    dispatchSettingsChanged()
  }

  const updateFaviconUrlTemplate = (template: string) => {
    storage.setFaviconUrlTemplate(template)
    setFaviconUrlTemplate(template)
    dispatchSettingsChanged()
  }

  const updateVerboseLogging = (verbose: boolean) => {
    storage.setVerboseLogging(verbose)
    setVerboseLogging(verbose)
    dispatchSettingsChanged()
  }

  const updateEnableMarkdown = (enable: boolean) => {
    storage.setEnableMarkdown(enable)
    setEnableMarkdown(enable)
    dispatchSettingsChanged()
  }

  return (
    <ContentPolicyContext.Provider
      value={{
        nsfwDisplayPolicy,
        setNsfwDisplayPolicy: updateNsfwDisplayPolicy,
        hideContentMentioningMutedUsers,
        setHideContentMentioningMutedUsers: updateHideContentMentioningMutedUsers,
        autoLoadMedia,
        mediaAutoLoadPolicy,
        setMediaAutoLoadPolicy: updateMediaAutoLoadPolicy,
        faviconUrlTemplate,
        setFaviconUrlTemplate: updateFaviconUrlTemplate,
        verboseLogging,
        setVerboseLogging: updateVerboseLogging,
        enableMarkdown,
        setEnableMarkdown: updateEnableMarkdown
      }}
    >
      {children}
    </ContentPolicyContext.Provider>
  )
}
