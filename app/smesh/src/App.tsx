import 'yet-another-react-lightbox/styles.css'
import './index.css'

import { Toaster } from '@/components/ui/sonner'
import BackgroundRelayDiscovery from '@/components/BackgroundRelayDiscovery'
import UpdateNotification from '@/components/UpdateNotification'
import { BookmarksProvider } from '@/providers/BookmarksProvider'
import { ContentPolicyProvider } from '@/providers/ContentPolicyProvider'
import { EventHandlerProvider } from '@/providers/EventHandlerProvider'
import { DeletedEventProvider } from '@/providers/DeletedEventProvider'
import { DMProvider } from '@/providers/DMProvider'
import { EmojiPackProvider } from '@/providers/EmojiPackProvider'
import { FavoriteRelaysProvider } from '@/providers/FavoriteRelaysProvider'
import { FeedProvider } from '@/providers/FeedProvider'
import { FollowListProvider } from '@/providers/FollowListProvider'
import { KindFilterProvider } from '@/providers/KindFilterProvider'
import { SocialGraphFilterProvider } from '@/providers/SocialGraphFilterProvider'
import { MediaUploadServiceProvider } from '@/providers/MediaUploadServiceProvider'
import { MuteListProvider } from '@/providers/MuteListProvider'
import { NostrProvider } from '@/providers/NostrProvider'
import { NRCProvider } from '@/providers/NRCProvider'
import { PasswordPromptProvider } from '@/providers/PasswordPromptProvider'
import { PinListProvider } from '@/providers/PinListProvider'
import { PinnedUsersProvider } from '@/providers/PinnedUsersProvider'
import { RepositoryProvider } from '@/providers/RepositoryProvider'
import { ScreenSizeProvider } from '@/providers/ScreenSizeProvider'
import { SettingsSyncProvider } from '@/providers/SettingsSyncProvider'
import { ThemeProvider } from '@/providers/ThemeProvider'
import { UserPreferencesProvider } from '@/providers/UserPreferencesProvider'
import { UserTrustProvider } from '@/providers/UserTrustProvider'
import { ZapProvider } from '@/providers/ZapProvider'
import { ComposeProvider } from '@/providers/ComposeProvider'
import { PageManager } from './PageManager'

export default function App(): JSX.Element {
  return (
    <ScreenSizeProvider>
      <EventHandlerProvider>
      <UserPreferencesProvider>
        <ThemeProvider>
          <ContentPolicyProvider>
            <DeletedEventProvider>
              <PasswordPromptProvider>
                <NostrProvider>
                <NRCProvider>
                <RepositoryProvider>
                <SettingsSyncProvider>
                  <ZapProvider>
                    <FavoriteRelaysProvider>
                      <FollowListProvider>
                        <MuteListProvider>
                          <DMProvider>
                            <UserTrustProvider>
                              <BookmarksProvider>
                              <EmojiPackProvider>
                                <PinListProvider>
                                  <PinnedUsersProvider>
                                    <FeedProvider>
                                      <MediaUploadServiceProvider>
                                        <SocialGraphFilterProvider>
                                          <KindFilterProvider>
                                          <ComposeProvider>
                                            <UpdateNotification />
                                            <BackgroundRelayDiscovery />
                                            <PageManager />
                                            <Toaster />
                                          </ComposeProvider>
                                          </KindFilterProvider>
                                        </SocialGraphFilterProvider>
                                      </MediaUploadServiceProvider>
                                    </FeedProvider>
                                  </PinnedUsersProvider>
                                </PinListProvider>
                              </EmojiPackProvider>
                            </BookmarksProvider>
                            </UserTrustProvider>
                          </DMProvider>
                        </MuteListProvider>
                      </FollowListProvider>
                    </FavoriteRelaysProvider>
                  </ZapProvider>
                </SettingsSyncProvider>
                </RepositoryProvider>
                </NRCProvider>
                </NostrProvider>
              </PasswordPromptProvider>
            </DeletedEventProvider>
          </ContentPolicyProvider>
        </ThemeProvider>
      </UserPreferencesProvider>
      </EventHandlerProvider>
    </ScreenSizeProvider>
  )
}
