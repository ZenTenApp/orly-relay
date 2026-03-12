import AboutInfoDialog from '@/components/AboutInfoDialog'
import QrScannerModal from '@/components/QrScannerModal'
import Donation from '@/components/Donation'
import RelayDiscovery from '@/components/RelayDiscovery'
import Emoji from '@/components/Emoji'
import EmojiPackList from '@/components/EmojiPackList'
import EmojiPickerDialog from '@/components/EmojiPickerDialog'
import FavoriteRelaysSetting from '@/components/FavoriteRelaysSetting'
import CacheRelaysSetting from '@/components/CacheRelaysSetting'
import MailboxSetting from '@/components/MailboxSetting'
import NRCSettings from '@/components/NRCSettings'
import NoteList from '@/components/NoteList'
import Tabs from '@/components/Tabs'
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger
} from '@/components/ui/accordion'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Tabs as RadixTabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  DEFAULT_FAVICON_URL_TEMPLATE,
  MEDIA_AUTO_LOAD_POLICY,
  NSFW_DISPLAY_POLICY,
  PRIMARY_COLORS,
  TPrimaryColor
} from '@/constants'
import client from '@/services/client.service'
import { LocalizedLanguageNames, TLanguage } from '@/i18n'
import { cn, isSupportCheckConnectionType } from '@/lib/utils'
import LlmSetting from '@/pages/secondary/PostSettingsPage/LlmSetting'
import MediaUploadServiceSetting from '@/pages/secondary/PostSettingsPage/MediaUploadServiceSetting'
import DefaultZapAmountInput from '@/pages/secondary/WalletPage/DefaultZapAmountInput'
import DefaultZapCommentInput from '@/pages/secondary/WalletPage/DefaultZapCommentInput'
import LightningAddressInput from '@/pages/secondary/WalletPage/LightningAddressInput'
import QuickZapSwitch from '@/pages/secondary/WalletPage/QuickZapSwitch'
import { useContentPolicy } from '@/providers/ContentPolicyProvider'
import { useNostr } from '@/providers/NostrProvider'
import { useScreenSize } from '@/providers/ScreenSizeProvider'
import { useTheme } from '@/providers/ThemeProvider'
import { useUserPreferences } from '@/providers/UserPreferencesProvider'
import { useUserTrust } from '@/providers/UserTrustProvider'
import { useZap } from '@/providers/ZapProvider'
import storage, { dispatchSettingsChanged } from '@/services/local-storage.service'
import { TMediaAutoLoadPolicy, TNsfwDisplayPolicy } from '@/types'
import { connectNWC, disconnect, launchModal } from '@getalby/bitcoin-connect-react'
import {
  Check,
  Cog,
  Columns2,
  Copy,
  Info,
  KeyRound,
  LayoutList,
  List,
  MessageSquare,
  Monitor,
  Moon,
  Palette,
  PanelLeft,
  PencilLine,
  RotateCcw,
  ScanLine,
  RefreshCw,
  Server,
  Settings2,
  Smile,
  Sun,
  Wallet,
  Wrench
} from 'lucide-react'
import { kinds } from 'nostr-tools'
import { forwardRef, HTMLProps, useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useKeyboardNavigation, useNavigationRegion, NavigationIntent } from '@/providers/KeyboardNavigationProvider'
import { usePrimaryPage } from '@/PageManager'

type TEmojiTab = 'my-packs' | 'explore'

const THEMES = [
  { key: 'system', label: 'System', icon: <Monitor className="size-5" /> },
  { key: 'light', label: 'Light', icon: <Sun className="size-5" /> },
  { key: 'dark', label: 'Dark', icon: <Moon className="size-5" /> },
] as const

const LAYOUTS = [
  { key: false, label: 'Two-column', icon: <Columns2 className="size-5" /> },
  { key: true, label: 'Single-column', icon: <PanelLeft className="size-5" /> }
] as const

const NOTIFICATION_STYLES = [
  { key: 'detailed', label: 'Detailed', icon: <LayoutList className="size-5" /> },
  { key: 'compact', label: 'Compact', icon: <List className="size-5" /> }
] as const

// Accordion item values for keyboard navigation
const ACCORDION_ITEMS = ['general', 'appearance', 'relays', 'sync', 'wallet', 'posts', 'emoji-packs', 'messaging', 'system', 'tools']

export default function Settings() {
  const { t, i18n } = useTranslation()
  const { pubkey, nsec, ncryptsec } = useNostr()
  const { isSmallScreen } = useScreenSize()
  const [copiedNsec, setCopiedNsec] = useState(false)
  const [copiedNcryptsec, setCopiedNcryptsec] = useState(false)
  const [openSection, setOpenSection] = useState<string>('')
  const [selectedAccordionIndex, setSelectedAccordionIndex] = useState(-1)
  const accordionRefs = useRef<(HTMLDivElement | null)[]>([])

  const { activeColumn, scrollToCenter } = useKeyboardNavigation()
  const { current: currentPage } = usePrimaryPage()

  // Get the visible accordion items based on pubkey availability
  const visibleAccordionItems = pubkey
    ? ACCORDION_ITEMS
    : ACCORDION_ITEMS.filter((item) => !['sync', 'wallet', 'posts', 'emoji-packs', 'messaging'].includes(item))

  // Register as a navigation region - Settings decides what "up/down" means
  const handleSettingsIntent = useCallback(
    (intent: NavigationIntent): boolean => {
      switch (intent) {
        case 'up':
          setSelectedAccordionIndex((prev) => {
            const newIndex = prev <= 0 ? 0 : prev - 1
            setTimeout(() => {
              const el = accordionRefs.current[newIndex]
              if (el) scrollToCenter(el)
            }, 0)
            return newIndex
          })
          return true

        case 'down':
          setSelectedAccordionIndex((prev) => {
            const newIndex = prev < 0 ? 0 : Math.min(prev + 1, visibleAccordionItems.length - 1)
            setTimeout(() => {
              const el = accordionRefs.current[newIndex]
              if (el) scrollToCenter(el)
            }, 0)
            return newIndex
          })
          return true

        case 'activate':
          if (selectedAccordionIndex >= 0 && selectedAccordionIndex < visibleAccordionItems.length) {
            const value = visibleAccordionItems[selectedAccordionIndex]
            setOpenSection((prev) => (prev === value ? '' : value))
            return true
          }
          return false

        case 'cancel':
          if (openSection) {
            setOpenSection('')
            return true
          }
          return false

        default:
          return false
      }
    },
    [selectedAccordionIndex, openSection, visibleAccordionItems, scrollToCenter]
  )

  // Register this component as a navigation region when it's active
  useNavigationRegion(
    'settings-accordion',
    100, // High priority - handle intents before default handlers
    () => activeColumn === 1 && currentPage === 'settings', // Only active when settings is displayed
    handleSettingsIntent,
    [handleSettingsIntent, activeColumn, currentPage]
  )

  // Reset selection when column changes
  useEffect(() => {
    if (activeColumn !== 1) {
      setSelectedAccordionIndex(-1)
    }
  }, [activeColumn])

  // Helper to get accordion index and check selection
  const getAccordionIndex = useCallback(
    (value: string) => visibleAccordionItems.indexOf(value),
    [visibleAccordionItems]
  )

  const isAccordionSelected = useCallback(
    (value: string) => selectedAccordionIndex === getAccordionIndex(value),
    [selectedAccordionIndex, getAccordionIndex]
  )

  const setAccordionRef = useCallback((value: string) => (el: HTMLDivElement | null) => {
    const idx = visibleAccordionItems.indexOf(value)
    if (idx !== -1) {
      accordionRefs.current[idx] = el
    }
  }, [visibleAccordionItems])

  // General settings
  const [language, setLanguage] = useState<TLanguage>(i18n.language as TLanguage)
  const {
    autoplay,
    setAutoplay,
    nsfwDisplayPolicy,
    setNsfwDisplayPolicy,
    hideContentMentioningMutedUsers,
    setHideContentMentioningMutedUsers,
    mediaAutoLoadPolicy,
    setMediaAutoLoadPolicy,
    faviconUrlTemplate,
    setFaviconUrlTemplate
  } = useContentPolicy()
  const {
    hideUntrustedNotes,
    updateHideUntrustedNotes,
    hideUntrustedInteractions,
    updateHideUntrustedInteractions,
    hideUntrustedNotifications,
    updateHideUntrustedNotifications
  } = useUserTrust()
  const {
    quickReaction,
    updateQuickReaction,
    quickReactionEmoji,
    updateQuickReactionEmoji,
    enableSingleColumnLayout,
    updateEnableSingleColumnLayout,
    autoInsertNewNotes,
    updateAutoInsertNewNotes,
    notificationListStyle,
    updateNotificationListStyle
  } = useUserPreferences()

  // Appearance settings
  const { themeSetting, setThemeSetting, primaryColor, setPrimaryColor } = useTheme()

  // Wallet settings
  const { isWalletConnected, walletInfo } = useZap()

  // Relay settings
  const [relayTabValue, setRelayTabValue] = useState('favorite-relays')

  // Emoji settings
  const [emojiTab, setEmojiTab] = useState<TEmojiTab>('my-packs')

  // System settings
  const [filterOutOnionRelays, setFilterOutOnionRelays] = useState(storage.getFilterOutOnionRelays())
  const [graphQueriesEnabled, setGraphQueriesEnabled] = useState(storage.getGraphQueriesEnabled())

  // Messaging settings
  const [preferNip44, setPreferNip44] = useState(storage.getPreferNip44())

  // Post settings
  const [addClientTag, setAddClientTag] = useState(storage.getAddClientTag())

  // Wallet QR scanner
  const [showWalletScanner, setShowWalletScanner] = useState(false)

  const handleWalletScan = useCallback((result: string) => {
    // Check if it's a valid NWC URI
    if (result.startsWith('nostr+walletconnect://')) {
      connectNWC(result)
    }
  }, [])

  const handleLanguageChange = (value: TLanguage) => {
    i18n.changeLanguage(value)
    setLanguage(value)
  }

  const handleAccordionChange = useCallback((value: string) => {
    // Prevent auto-scroll when opening accordion sections
    const scrollY = window.scrollY
    setOpenSection(value)
    requestAnimationFrame(() => {
      window.scrollTo(0, scrollY)
    })
  }, [])

  return (
    <div>
      <Accordion
        type="single"
        collapsible
        value={openSection}
        onValueChange={handleAccordionChange}
        className="w-full"
      >
        {/* General */}
        <NavigableAccordionItem ref={setAccordionRef('general')} isSelected={isAccordionSelected('general')}>
          <AccordionItem value="general">
            <AccordionTrigger className="px-4 hover:no-underline">
              <div className="flex items-center gap-4">
                <Settings2 className="size-4" />
                <span>{t('General')}</span>
              </div>
            </AccordionTrigger>
          <AccordionContent className="px-4 space-y-4">
            <SettingItem>
              <Label htmlFor="languages" className="text-base font-normal">
                {t('Languages')}
              </Label>
              <Select defaultValue="en" value={language} onValueChange={handleLanguageChange}>
                <SelectTrigger id="languages" className="w-48">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {Object.entries(LocalizedLanguageNames).map(([key, value]) => (
                    <SelectItem key={key} value={key}>
                      {value}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </SettingItem>
            <SettingItem>
              <Label htmlFor="media-auto-load-policy" className="text-base font-normal">
                {t('Auto-load media')}
              </Label>
              <Select
                defaultValue="wifi-only"
                value={mediaAutoLoadPolicy}
                onValueChange={(value: TMediaAutoLoadPolicy) => setMediaAutoLoadPolicy(value)}
              >
                <SelectTrigger id="media-auto-load-policy" className="w-48">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={MEDIA_AUTO_LOAD_POLICY.ALWAYS}>{t('Always')}</SelectItem>
                  {isSupportCheckConnectionType() && (
                    <SelectItem value={MEDIA_AUTO_LOAD_POLICY.WIFI_ONLY}>{t('Wi-Fi only')}</SelectItem>
                  )}
                  <SelectItem value={MEDIA_AUTO_LOAD_POLICY.NEVER}>{t('Never')}</SelectItem>
                </SelectContent>
              </Select>
            </SettingItem>
            <SettingItem>
              <Label htmlFor="autoplay" className="text-base font-normal">
                <div>{t('Autoplay')}</div>
                <div className="text-muted-foreground">{t('Enable video autoplay on this device')}</div>
              </Label>
              <Switch id="autoplay" checked={autoplay} onCheckedChange={setAutoplay} />
            </SettingItem>
            <SettingItem>
              <Label htmlFor="auto-insert-new-notes" className="text-base font-normal">
                <div>{t('Live feed')}</div>
                <div className="text-muted-foreground">{t('Automatically insert new notes into the feed')}</div>
              </Label>
              <Switch id="auto-insert-new-notes" checked={autoInsertNewNotes} onCheckedChange={updateAutoInsertNewNotes} />
            </SettingItem>
            <SettingItem>
              <Label htmlFor="hide-untrusted-notes" className="text-base font-normal">
                {t('Hide untrusted notes')}
              </Label>
              <Switch
                id="hide-untrusted-notes"
                checked={hideUntrustedNotes}
                onCheckedChange={updateHideUntrustedNotes}
              />
            </SettingItem>
            <SettingItem>
              <Label htmlFor="hide-untrusted-interactions" className="text-base font-normal">
                {t('Hide untrusted interactions')}
              </Label>
              <Switch
                id="hide-untrusted-interactions"
                checked={hideUntrustedInteractions}
                onCheckedChange={updateHideUntrustedInteractions}
              />
            </SettingItem>
            <SettingItem>
              <Label htmlFor="hide-untrusted-notifications" className="text-base font-normal">
                {t('Hide untrusted notifications')}
              </Label>
              <Switch
                id="hide-untrusted-notifications"
                checked={hideUntrustedNotifications}
                onCheckedChange={updateHideUntrustedNotifications}
              />
            </SettingItem>
            <SettingItem>
              <Label htmlFor="hide-content-mentioning-muted-users" className="text-base font-normal">
                {t('Hide content mentioning muted users')}
              </Label>
              <Switch
                id="hide-content-mentioning-muted-users"
                checked={hideContentMentioningMutedUsers}
                onCheckedChange={setHideContentMentioningMutedUsers}
              />
            </SettingItem>
            <SettingItem>
              <Label htmlFor="nsfw-display-policy" className="text-base font-normal">
                {t('NSFW content display')}
              </Label>
              <Select
                value={nsfwDisplayPolicy}
                onValueChange={(value: TNsfwDisplayPolicy) => setNsfwDisplayPolicy(value)}
              >
                <SelectTrigger id="nsfw-display-policy" className="w-48">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={NSFW_DISPLAY_POLICY.HIDE}>{t('Hide completely')}</SelectItem>
                  <SelectItem value={NSFW_DISPLAY_POLICY.HIDE_CONTENT}>{t('Show but hide content')}</SelectItem>
                  <SelectItem value={NSFW_DISPLAY_POLICY.SHOW}>{t('Show directly')}</SelectItem>
                </SelectContent>
              </Select>
            </SettingItem>
            <SettingItem>
              <Label htmlFor="quick-reaction" className="text-base font-normal">
                <div>{t('Quick reaction')}</div>
                <div className="text-muted-foreground">
                  {t('If enabled, you can react with a single click. Click and hold for more options')}
                </div>
              </Label>
              <Switch id="quick-reaction" checked={quickReaction} onCheckedChange={updateQuickReaction} />
            </SettingItem>
            {quickReaction && (
              <SettingItem>
                <Label htmlFor="quick-reaction-emoji" className="text-base font-normal">
                  {t('Quick reaction emoji')}
                </Label>
                <div className="flex items-center gap-2">
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => updateQuickReactionEmoji('+')}
                    className="text-muted-foreground hover:text-foreground"
                  >
                    <RotateCcw />
                  </Button>
                  <EmojiPickerDialog
                    onEmojiClick={(emoji) => {
                      if (!emoji) return
                      updateQuickReactionEmoji(emoji)
                    }}
                  >
                    <Button variant="ghost" size="icon" className="border">
                      <Emoji emoji={quickReactionEmoji} />
                    </Button>
                  </EmojiPickerDialog>
                </div>
              </SettingItem>
            )}
          </AccordionContent>
          </AccordionItem>
        </NavigableAccordionItem>

        {/* Appearance */}
        <NavigableAccordionItem ref={setAccordionRef('appearance')} isSelected={isAccordionSelected('appearance')}>
          <AccordionItem value="appearance">
          <AccordionTrigger className="px-4 hover:no-underline">
            <div className="flex items-center gap-4">
              <Palette className="size-4" />
              <span>{t('Appearance')}</span>
            </div>
          </AccordionTrigger>
          <AccordionContent className="px-4 space-y-4">
            <div className="flex flex-col gap-2">
              <Label className="text-base">{t('Theme')}</Label>
              <div className="grid grid-cols-2 md:grid-cols-4 gap-4 w-full">
                {THEMES.map(({ key, label, icon }) => (
                  <OptionButton
                    key={key}
                    isSelected={themeSetting === key}
                    icon={icon}
                    label={t(label)}
                    onClick={() => setThemeSetting(key)}
                  />
                ))}
              </div>
            </div>
            {!isSmallScreen && (
              <div className="flex flex-col gap-2">
                <Label className="text-base">{t('Layout')}</Label>
                <div className="grid grid-cols-2 gap-4 w-full">
                  {LAYOUTS.map(({ key, label, icon }) => (
                    <OptionButton
                      key={key.toString()}
                      isSelected={enableSingleColumnLayout === key}
                      icon={icon}
                      label={t(label)}
                      onClick={() => updateEnableSingleColumnLayout(key)}
                    />
                  ))}
                </div>
              </div>
            )}
            <div className="flex flex-col gap-2">
              <Label className="text-base">{t('Notification list style')}</Label>
              <div className="grid grid-cols-2 gap-4 w-full">
                {NOTIFICATION_STYLES.map(({ key, label, icon }) => (
                  <OptionButton
                    key={key}
                    isSelected={notificationListStyle === key}
                    icon={icon}
                    label={t(label)}
                    onClick={() => updateNotificationListStyle(key)}
                  />
                ))}
              </div>
            </div>
            <div className="flex flex-col gap-2">
              <Label className="text-base">{t('Primary color')}</Label>
              <div className="grid grid-cols-4 gap-4 w-full">
                {Object.entries(PRIMARY_COLORS).map(([key, config]) => (
                  <OptionButton
                    key={key}
                    isSelected={primaryColor === key}
                    icon={
                      <div
                        className="size-8 rounded-full shadow-md"
                        style={{ backgroundColor: `hsl(${config.light.primary})` }}
                      />
                    }
                    label={t(config.name)}
                    onClick={() => setPrimaryColor(key as TPrimaryColor)}
                  />
                ))}
              </div>
            </div>
          </AccordionContent>
          </AccordionItem>
        </NavigableAccordionItem>

        {/* Relays */}
        <NavigableAccordionItem ref={setAccordionRef('relays')} isSelected={isAccordionSelected('relays')}>
          <AccordionItem value="relays">
          <AccordionTrigger className="px-4 hover:no-underline">
            <div className="flex items-center gap-4">
              <Server className="size-4" />
              <span>{t('Relays')}</span>
            </div>
          </AccordionTrigger>
          <AccordionContent className="px-4">
            <RadixTabs value={relayTabValue} onValueChange={setRelayTabValue} className="space-y-4">
              <TabsList>
                <TabsTrigger value="favorite-relays">{t('Favorite Relays')}</TabsTrigger>
                <TabsTrigger value="mailbox">{t('Read & Write Relays')}</TabsTrigger>
                <TabsTrigger value="cache-relays">{t('Cache Relays')}</TabsTrigger>
              </TabsList>
              <TabsContent value="favorite-relays">
                <FavoriteRelaysSetting />
              </TabsContent>
              <TabsContent value="mailbox">
                <MailboxSetting />
              </TabsContent>
              <TabsContent value="cache-relays">
                <CacheRelaysSetting />
              </TabsContent>
            </RadixTabs>
          </AccordionContent>
          </AccordionItem>
        </NavigableAccordionItem>

        {/* Sync (NRC) */}
        {!!pubkey && (
          <NavigableAccordionItem ref={setAccordionRef('sync')} isSelected={isAccordionSelected('sync')}>
            <AccordionItem value="sync">
            <AccordionTrigger className="px-4 hover:no-underline">
              <div className="flex items-center gap-4">
                <RefreshCw className="size-4" />
                <span>{t('Device Sync')}</span>
              </div>
            </AccordionTrigger>
            <AccordionContent className="px-4">
              <NRCSettings />
            </AccordionContent>
            </AccordionItem>
          </NavigableAccordionItem>
        )}

        {/* Wallet */}
        {!!pubkey && (
          <NavigableAccordionItem ref={setAccordionRef('wallet')} isSelected={isAccordionSelected('wallet')}>
            <AccordionItem value="wallet">
            <AccordionTrigger className="px-4 hover:no-underline">
              <div className="flex items-center gap-4">
                <Wallet className="size-4" />
                <span>{t('Wallet')}</span>
              </div>
            </AccordionTrigger>
            <AccordionContent className="px-4 space-y-4">
              {isWalletConnected ? (
                <>
                  <div>
                    {walletInfo?.node.alias && (
                      <div className="mb-2">
                        {t('Connected to')} <strong>{walletInfo.node.alias}</strong>
                      </div>
                    )}
                    <AlertDialog>
                      <AlertDialogTrigger asChild>
                        <Button variant="destructive">{t('Disconnect Wallet')}</Button>
                      </AlertDialogTrigger>
                      <AlertDialogContent>
                        <AlertDialogHeader>
                          <AlertDialogTitle>{t('Are you absolutely sure?')}</AlertDialogTitle>
                          <AlertDialogDescription>
                            {t('You will not be able to send zaps to others.')}
                          </AlertDialogDescription>
                        </AlertDialogHeader>
                        <AlertDialogFooter>
                          <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
                          <AlertDialogAction variant="destructive" onClick={() => disconnect()}>
                            {t('Disconnect')}
                          </AlertDialogAction>
                        </AlertDialogFooter>
                      </AlertDialogContent>
                    </AlertDialog>
                  </div>
                  <DefaultZapAmountInput />
                  <DefaultZapCommentInput />
                  <QuickZapSwitch />
                  <LightningAddressInput />
                </>
              ) : (
                <>
                  {showWalletScanner && (
                    <QrScannerModal
                      onScan={handleWalletScan}
                      onClose={() => setShowWalletScanner(false)}
                    />
                  )}
                  <div className="flex items-center gap-2">
                    <Button className="bg-foreground hover:bg-foreground/90" onClick={() => launchModal()}>
                      {t('Connect Wallet')}
                    </Button>
                    <Button
                      variant="outline"
                      size="icon"
                      onClick={() => setShowWalletScanner(true)}
                      title={t('Scan NWC QR code')}
                    >
                      <ScanLine className="h-4 w-4" />
                    </Button>
                  </div>
                </>
              )}
            </AccordionContent>
            </AccordionItem>
          </NavigableAccordionItem>
        )}

        {/* Post Settings */}
        {!!pubkey && (
          <NavigableAccordionItem ref={setAccordionRef('posts')} isSelected={isAccordionSelected('posts')}>
            <AccordionItem value="posts">
              <AccordionTrigger className="px-4 hover:no-underline">
                <div className="flex items-center gap-4">
                  <PencilLine className="size-4" />
                  <span>{t('Post settings')}</span>
                </div>
              </AccordionTrigger>
              <AccordionContent className="px-4 space-y-4">
                <MediaUploadServiceSetting />
                <LlmSetting />
                <SettingItem>
                  <div>
                    <Label htmlFor="add-client-tag" className="text-base font-normal">
                      {t('Include client tag')}
                    </Label>
                    <p className="text-sm text-muted-foreground">
                      {t('Add a tag to identify posts as coming from smesh')}
                    </p>
                  </div>
                  <Switch
                    id="add-client-tag"
                    checked={addClientTag}
                    onCheckedChange={(checked) => {
                      storage.setAddClientTag(checked)
                      setAddClientTag(checked)
                      dispatchSettingsChanged()
                    }}
                  />
                </SettingItem>
              </AccordionContent>
            </AccordionItem>
          </NavigableAccordionItem>
        )}

        {/* Emoji Packs */}
        {!!pubkey && (
          <NavigableAccordionItem ref={setAccordionRef('emoji-packs')} isSelected={isAccordionSelected('emoji-packs')}>
            <AccordionItem value="emoji-packs">
            <AccordionTrigger className="px-4 hover:no-underline">
              <div className="flex items-center gap-4">
                <Smile className="size-4" />
                <span>{t('Emoji Packs')}</span>
              </div>
            </AccordionTrigger>
            <AccordionContent className="px-4">
              <Tabs
                value={emojiTab}
                tabs={[
                  { value: 'my-packs', label: 'My Packs' },
                  { value: 'explore', label: 'Explore' }
                ]}
                onTabChange={(tab) => setEmojiTab(tab as TEmojiTab)}
              />
              {emojiTab === 'my-packs' ? (
                <EmojiPackList />
              ) : (
                <NoteList
                  showKinds={[kinds.Emojisets]}
                  subRequests={[{ urls: client.currentRelays, filter: {} }]}
                  hideUntrustedNotes={hideUntrustedNotes}
                />
              )}
            </AccordionContent>
            </AccordionItem>
          </NavigableAccordionItem>
        )}

        {/* Messaging */}
        {!!pubkey && (
          <NavigableAccordionItem ref={setAccordionRef('messaging')} isSelected={isAccordionSelected('messaging')}>
            <AccordionItem value="messaging">
              <AccordionTrigger className="px-4 hover:no-underline">
                <div className="flex items-center gap-4">
                  <MessageSquare className="size-4" />
                  <span>{t('Messaging')}</span>
                </div>
              </AccordionTrigger>
              <AccordionContent className="px-4 space-y-4">
                <SettingItem>
                  <Label htmlFor="prefer-nip44" className="text-base font-normal">
                    <div>{t('Prefer NIP-44 encryption')}</div>
                    <div className="text-muted-foreground text-sm">
                      {t('Use modern encryption for new conversations')}
                    </div>
                  </Label>
                  <Switch
                    id="prefer-nip44"
                    checked={preferNip44}
                    onCheckedChange={(checked) => {
                      storage.setPreferNip44(checked)
                      setPreferNip44(checked)
                      dispatchSettingsChanged()
                    }}
                  />
                </SettingItem>
              </AccordionContent>
            </AccordionItem>
          </NavigableAccordionItem>
        )}

        {/* System */}
        <NavigableAccordionItem ref={setAccordionRef('system')} isSelected={isAccordionSelected('system')}>
          <AccordionItem value="system">
            <AccordionTrigger className="px-4 hover:no-underline">
              <div className="flex items-center gap-4">
                <Cog className="size-4" />
              <span>{t('System')}</span>
            </div>
          </AccordionTrigger>
          <AccordionContent className="px-4 space-y-4">
            <div className="space-y-2">
              <Label htmlFor="favicon-url" className="text-base font-normal">
                {t('Favicon URL')}
              </Label>
              <Input
                id="favicon-url"
                type="text"
                value={faviconUrlTemplate}
                onChange={(e) => setFaviconUrlTemplate(e.target.value)}
                placeholder={DEFAULT_FAVICON_URL_TEMPLATE}
              />
            </div>
            <SettingItem>
              <Label htmlFor="filter-out-onion-relays" className="text-base font-normal">
                {t('Filter out onion relays')}
              </Label>
              <Switch
                id="filter-out-onion-relays"
                checked={filterOutOnionRelays}
                onCheckedChange={(checked) => {
                  storage.setFilterOutOnionRelays(checked)
                  setFilterOutOnionRelays(checked)
                  dispatchSettingsChanged()
                }}
              />
            </SettingItem>
            <SettingItem>
              <div>
                <Label htmlFor="graph-queries-enabled" className="text-base font-normal">
                  {t('Graph query optimization')}
                </Label>
                <p className="text-sm text-muted-foreground">
                  {t('Use graph queries for faster follow/thread loading on supported relays')}
                </p>
              </div>
              <Switch
                id="graph-queries-enabled"
                checked={graphQueriesEnabled}
                onCheckedChange={(checked) => {
                  storage.setGraphQueriesEnabled(checked)
                  setGraphQueriesEnabled(checked)
                  dispatchSettingsChanged()
                }}
              />
            </SettingItem>
          </AccordionContent>
          </AccordionItem>
        </NavigableAccordionItem>

        {/* Tools */}
        <NavigableAccordionItem ref={setAccordionRef('tools')} isSelected={isAccordionSelected('tools')}>
          <AccordionItem value="tools">
            <AccordionTrigger className="px-4 hover:no-underline">
              <div className="flex items-center gap-4">
                <Wrench className="size-4" />
                <span>{t('Tools')}</span>
              </div>
            </AccordionTrigger>
            <AccordionContent className="px-4 space-y-4">
              <div className="space-y-2">
                <h4 className="font-medium">{t('Relay Discovery')}</h4>
                <RelayDiscovery />
              </div>
            </AccordionContent>
          </AccordionItem>
        </NavigableAccordionItem>
      </Accordion>

      {/* Non-accordion items */}
      {!!nsec && (
        <SettingItem
          className="clickable"
          onClick={() => {
            navigator.clipboard.writeText(nsec)
            setCopiedNsec(true)
            setTimeout(() => setCopiedNsec(false), 2000)
          }}
        >
          <div className="flex items-center gap-4">
            <KeyRound />
            <div>{t('Copy private key')} (nsec)</div>
          </div>
          {copiedNsec ? <Check /> : <Copy />}
        </SettingItem>
      )}
      {!!ncryptsec && (
        <SettingItem
          className="clickable"
          onClick={() => {
            navigator.clipboard.writeText(ncryptsec)
            setCopiedNcryptsec(true)
            setTimeout(() => setCopiedNcryptsec(false), 2000)
          }}
        >
          <div className="flex items-center gap-4">
            <KeyRound />
            <div>{t('Copy private key')} (ncryptsec)</div>
          </div>
          {copiedNcryptsec ? <Check /> : <Copy />}
        </SettingItem>
      )}
      <AboutInfoDialog>
        <SettingItem className="clickable">
          <div className="flex items-center gap-4">
            <Info />
            <div>{t('About')}</div>
          </div>
          <div className="flex gap-2 items-center">
            <div className="text-muted-foreground">
              v{import.meta.env.APP_VERSION} ({import.meta.env.GIT_COMMIT})
            </div>
          </div>
        </SettingItem>
      </AboutInfoDialog>
      <div className="p-4">
        <Donation />
      </div>
    </div>
  )
}

const SettingItem = forwardRef<HTMLDivElement, HTMLProps<HTMLDivElement>>(
  ({ children, className, ...props }, ref) => {
    return (
      <div
        className={cn(
          'flex justify-between select-none items-center px-4 min-h-9 [&_svg]:size-4 [&_svg]:shrink-0',
          className
        )}
        {...props}
        ref={ref}
      >
        {children}
      </div>
    )
  }
)
SettingItem.displayName = 'SettingItem'

const OptionButton = ({
  isSelected,
  onClick,
  icon,
  label
}: {
  isSelected: boolean
  onClick: () => void
  icon: React.ReactNode
  label: string
}) => {
  return (
    <button
      onClick={onClick}
      className={cn(
        'flex flex-col items-center gap-2 py-4 rounded-lg border-2 transition-all',
        isSelected ? 'border-primary' : 'border-border hover:border-muted-foreground/40'
      )}
    >
      <div className="flex items-center justify-center w-8 h-8">{icon}</div>
      <span className="text-xs font-medium">{label}</span>
    </button>
  )
}

// Wrapper for keyboard-navigable accordion items
const NavigableAccordionItem = forwardRef<
  HTMLDivElement,
  {
    isSelected: boolean
    children: React.ReactNode
  }
>(({ isSelected, children }, ref) => {
  return (
    <div
      ref={ref}
      className={cn(
        'rounded-lg transition-all',
        isSelected && 'ring-2 ring-primary ring-offset-2 ring-offset-background'
      )}
    >
      {children}
    </div>
  )
})
NavigableAccordionItem.displayName = 'NavigableAccordionItem'
