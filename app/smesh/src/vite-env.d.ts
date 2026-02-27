/// <reference types="vite/client" />
/// <reference types="vite-plugin-pwa/react" />
import { TNip07 } from '@/types'

declare global {
  interface Window {
    nostr?: TNip07
  }
}
