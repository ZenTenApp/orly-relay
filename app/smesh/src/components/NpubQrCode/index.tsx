import { Dialog, DialogContent, DialogTrigger } from '@/components/ui/dialog'
import { Drawer, DrawerContent, DrawerTrigger } from '@/components/ui/drawer'
import { useScreenSize } from '@/providers/ScreenSizeProvider'
import { QrCodeIcon } from 'lucide-react'
import { nip19 } from 'nostr-tools'
import { useCallback, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import Nip05 from '../Nip05'
import QrCode from '../QrCode'
import UserAvatar from '../UserAvatar'
import Username from '../Username'

export default function NpubQrCode({ pubkey }: { pubkey: string }) {
  const { t } = useTranslation()
  const { isSmallScreen } = useScreenSize()
  const [open, setOpen] = useState(false)
  const npub = useMemo(() => {
    // Validate pubkey is a 64-character hex string before encoding
    if (!pubkey || !/^[0-9a-f]{64}$/i.test(pubkey)) return ''
    try {
      return nip19.npubEncode(pubkey)
    } catch {
      return ''
    }
  }, [pubkey])

  const handleQrClick = useCallback(() => {
    navigator.clipboard.writeText(npub)
    toast.success(t('Copied npub to clipboard'))
    setOpen(false)
  }, [npub, t])

  if (!npub) return null

  const trigger = (
    <button
      className="bg-muted rounded-full h-5 w-5 flex flex-col items-center justify-center text-muted-foreground hover:text-foreground"
      onClick={(e) => e.stopPropagation()}
    >
      <QrCodeIcon size={14} />
    </button>
  )

  const content = (
    <div className="w-full flex flex-col items-center gap-4 p-8">
      <div className="flex items-center w-full gap-2 pointer-events-none px-1">
        <UserAvatar size="big" userId={pubkey} />
        <div className="flex-1 w-0">
          <Username userId={pubkey} className="text-2xl font-semibold truncate" showQrCode={false} />
          <Nip05 pubkey={pubkey} />
        </div>
      </div>
      <button
        onClick={handleQrClick}
        className="cursor-pointer hover:opacity-90 transition-opacity"
        title={t('Click to copy npub')}
      >
        <QrCode size={512} value={`nostr:${npub}`} />
      </button>
      <div className="text-sm text-muted-foreground">{t('Click QR code to copy npub')}</div>
    </div>
  )

  if (isSmallScreen) {
    return (
      <Drawer open={open} onOpenChange={setOpen}>
        <DrawerTrigger asChild>{trigger}</DrawerTrigger>
        <DrawerContent>{content}</DrawerContent>
      </Drawer>
    )
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>{trigger}</DialogTrigger>
      <DialogContent className="w-80 p-0 m-0" onOpenAutoFocus={(e) => e.preventDefault()}>
        {content}
      </DialogContent>
    </Dialog>
  )
}
