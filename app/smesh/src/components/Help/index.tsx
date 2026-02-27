import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger
} from '@/components/ui/accordion'
import { Keyboard, Layout, MessageSquare, Settings, User, Zap } from 'lucide-react'
import { useTranslation } from 'react-i18next'

export default function Help() {
  const { t } = useTranslation()

  return (
    <div className="px-4 py-4">
      <Accordion type="single" collapsible className="space-y-2">
        <AccordionItem value="keyboard" className="border rounded-lg px-4">
          <AccordionTrigger className="py-3">
            <div className="flex items-center gap-3">
              <Keyboard className="size-5 text-muted-foreground" />
              <span className="font-medium">{t('Keyboard Navigation')}</span>
            </div>
          </AccordionTrigger>
          <AccordionContent className="pb-4">
            <div className="space-y-4 text-sm text-muted-foreground">
              <p>{t('Navigate the app entirely with your keyboard:')}</p>
              <p className="font-medium">{t('Toggle Keyboard Mode:')}</p>
              <div className="space-y-2">
                <KeyBinding keys={['⇧K']} description={t('Toggle keyboard navigation on/off')} />
                <KeyBinding keys={['Esc', 'Esc', 'Esc']} description={t('Triple-Escape to quickly exit keyboard mode')} />
              </div>
              <p className="text-xs opacity-70">{t('You can also click the keyboard button in the sidebar to toggle.')}</p>
              <p className="font-medium mt-4">{t('Movement:')}</p>
              <div className="space-y-2">
                <KeyBinding keys={['↑', '↓']} altKeys={['k', 'j']} description={t('Move between items in a list')} />
                <KeyBinding keys={['Tab']} description={t('Switch to next column (Shift+Tab for previous)')} />
                <KeyBinding keys={['Page Up']} description={t('Jump to top and focus first item')} />
              </div>
              <p className="font-medium mt-4">{t('Actions:')}</p>
              <div className="space-y-2">
                <KeyBinding keys={['→', 'Enter']} altKeys={['l']} description={t('Activate the selected item')} />
                <KeyBinding keys={['←']} altKeys={['h']} description={t('Go back (close panel or move to sidebar)')} />
                <KeyBinding keys={['Escape']} description={t('Close current view or cancel')} />
              </div>
              <p className="font-medium mt-4">{t('Note Actions (when a note is selected):')}</p>
              <div className="space-y-2">
                <KeyBinding keys={['r']} description={t('Reply')} />
                <KeyBinding keys={['p']} description={t('Repost')} />
                <KeyBinding keys={['q']} description={t('Quote')} />
                <KeyBinding keys={['R']} description={t('React with emoji')} />
                <KeyBinding keys={['z']} description={t('Zap (send sats)')} />
              </div>
              <p className="text-xs opacity-70 pt-2">{t('Selected items are centered on screen for easy viewing.')}</p>
            </div>
          </AccordionContent>
        </AccordionItem>

        <AccordionItem value="layout" className="border rounded-lg px-4">
          <AccordionTrigger className="py-3">
            <div className="flex items-center gap-3">
              <Layout className="size-5 text-muted-foreground" />
              <span className="font-medium">{t('Layout & Navigation')}</span>
            </div>
          </AccordionTrigger>
          <AccordionContent className="pb-4">
            <div className="space-y-3 text-sm text-muted-foreground">
              <p>{t('The app uses a multi-column layout:')}</p>
              <ul className="list-disc list-inside space-y-1.5 ml-2">
                <li>{t('Sidebar: Quick access to main sections')}</li>
                <li>{t('Primary column: Feed, notifications, inbox, search')}</li>
                <li>{t('Secondary column: Note details, user profiles, relay info')}</li>
              </ul>
              <p>{t('On mobile or single-column mode, pages stack on top of each other.')}</p>
              <p>{t('Use the columns button at the bottom of the sidebar to switch between layouts.')}</p>
            </div>
          </AccordionContent>
        </AccordionItem>

        <AccordionItem value="posting" className="border rounded-lg px-4">
          <AccordionTrigger className="py-3">
            <div className="flex items-center gap-3">
              <MessageSquare className="size-5 text-muted-foreground" />
              <span className="font-medium">{t('Posting & Interactions')}</span>
            </div>
          </AccordionTrigger>
          <AccordionContent className="pb-4">
            <div className="space-y-3 text-sm text-muted-foreground">
              <p><strong>{t('Creating Posts:')}</strong></p>
              <ul className="list-disc list-inside space-y-1.5 ml-2">
                <li>{t('Click the post button in the sidebar to compose a new note')}</li>
                <li>{t('Use @ to mention users and # for hashtags')}</li>
                <li>{t('Drag and drop images or use the attachment button')}</li>
              </ul>
              <p className="pt-2"><strong>{t('Interacting with Notes:')}</strong></p>
              <ul className="list-disc list-inside space-y-1.5 ml-2">
                <li>{t('Reply: Continue the conversation')}</li>
                <li>{t('Repost: Share to your followers')}</li>
                <li>{t('Quote: Repost with your own comment')}</li>
                <li>{t('React: Like or add emoji reactions')}</li>
                <li>{t('Zap: Send Bitcoin tips via Lightning')}</li>
              </ul>
            </div>
          </AccordionContent>
        </AccordionItem>

        <AccordionItem value="zaps" className="border rounded-lg px-4">
          <AccordionTrigger className="py-3">
            <div className="flex items-center gap-3">
              <Zap className="size-5 text-muted-foreground" />
              <span className="font-medium">{t('Zaps & Lightning')}</span>
            </div>
          </AccordionTrigger>
          <AccordionContent className="pb-4">
            <div className="space-y-3 text-sm text-muted-foreground">
              <p>{t('Zaps are Bitcoin tips sent via the Lightning Network:')}</p>
              <ul className="list-disc list-inside space-y-1.5 ml-2">
                <li>{t('To receive zaps, add a Lightning address to your profile')}</li>
                <li>{t('To send zaps, connect a Lightning wallet in Settings')}</li>
                <li>{t('Click the zap icon on any note to send sats')}</li>
                <li>{t('Long-press for custom zap amounts')}</li>
              </ul>
              <p className="pt-2">{t('Supported wallets include Alby, NWC-compatible wallets, and Cashu mints.')}</p>
            </div>
          </AccordionContent>
        </AccordionItem>

        <AccordionItem value="accounts" className="border rounded-lg px-4">
          <AccordionTrigger className="py-3">
            <div className="flex items-center gap-3">
              <User className="size-5 text-muted-foreground" />
              <span className="font-medium">{t('Account & Login')}</span>
            </div>
          </AccordionTrigger>
          <AccordionContent className="pb-4">
            <div className="space-y-3 text-sm text-muted-foreground">
              <p>{t('Nostr uses public/private key pairs for identity:')}</p>
              <ul className="list-disc list-inside space-y-1.5 ml-2">
                <li><strong>npub</strong>: {t('Your public key (share freely)')}</li>
                <li><strong>nsec</strong>: {t('Your private key (keep secret!)')}</li>
              </ul>
              <p className="pt-2"><strong>{t('Login Methods:')}</strong></p>
              <ul className="list-disc list-inside space-y-1.5 ml-2">
                <li><strong>{t('Browser Extension (NIP-07)')}</strong>: {t('Recommended. Uses extensions like Alby or nos2x')}</li>
                <li><strong>{t('Remote Signer (NIP-46)')}</strong>: {t('Connect to bunker signers like Amber or nsecBunker')}</li>
                <li><strong>{t('Private Key')}</strong>: {t('Enter nsec directly (less secure)')}</li>
                <li><strong>{t('View Only')}</strong>: {t('Browse with an npub without signing')}</li>
              </ul>
            </div>
          </AccordionContent>
        </AccordionItem>

        <AccordionItem value="settings" className="border rounded-lg px-4">
          <AccordionTrigger className="py-3">
            <div className="flex items-center gap-3">
              <Settings className="size-5 text-muted-foreground" />
              <span className="font-medium">{t('Settings Overview')}</span>
            </div>
          </AccordionTrigger>
          <AccordionContent className="pb-4">
            <div className="space-y-3 text-sm text-muted-foreground">
              <ul className="list-disc list-inside space-y-1.5 ml-2">
                <li><strong>{t('General')}</strong>: {t('Language, content preferences, mutes')}</li>
                <li><strong>{t('Appearance')}</strong>: {t('Theme, layout, visual options')}</li>
                <li><strong>{t('Relays')}</strong>: {t('Configure which relays to read from and write to')}</li>
                <li><strong>{t('Posts')}</strong>: {t('Posting preferences and default settings')}</li>
                <li><strong>{t('Wallet')}</strong>: {t('Lightning wallet connection for zaps')}</li>
                <li><strong>{t('Emoji Packs')}</strong>: {t('Custom emoji sets')}</li>
                <li><strong>{t('System')}</strong>: {t('Debug tools and app information')}</li>
              </ul>
            </div>
          </AccordionContent>
        </AccordionItem>
      </Accordion>
    </div>
  )
}

function KeyBinding({
  keys,
  altKeys,
  description
}: {
  keys: string[]
  altKeys?: string[]
  description: string
}) {
  return (
    <div className="flex items-center gap-3">
      <div className="flex items-center gap-1">
        {keys.map((key) => (
          <kbd key={key} className="px-2 py-1 text-xs font-mono bg-muted border rounded">
            {key}
          </kbd>
        ))}
        {altKeys && (
          <>
            <span className="text-xs text-muted-foreground mx-1">/</span>
            {altKeys.map((key) => (
              <kbd key={key} className="px-2 py-1 text-xs font-mono bg-muted border rounded">
                {key}
              </kbd>
            ))}
          </>
        )}
      </div>
      <span>{description}</span>
    </div>
  )
}
