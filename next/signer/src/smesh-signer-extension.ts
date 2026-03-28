/* eslint-disable @typescript-eslint/no-explicit-any */
import { Event as NostrEvent, EventTemplate } from 'nostr-tools';
import { ExtensionMethod } from '@common';

// Extend Window interface for NIP-07 and WebLN
declare global {
  interface Window {
    nostr?: any;
    webln?: any;
  }
}

type Relays = Record<string, { read: boolean; write: boolean }>;

// Fallback UUID generator for contexts where crypto.randomUUID is unavailable
function generateUUID(): string {
  if (typeof crypto !== 'undefined' && crypto.randomUUID) {
    return crypto.randomUUID();
  }
  // Fallback using crypto.getRandomValues
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  bytes[6] = (bytes[6] & 0x0f) | 0x40; // Version 4
  bytes[8] = (bytes[8] & 0x3f) | 0x80; // Variant 10
  const hex = [...bytes].map(b => b.toString(16).padStart(2, '0')).join('');
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}

class Messenger {
  #requests = new Map<
    string,
    {
      resolve: (value: unknown) => void;
      reject: (reason: any) => void;
    }
  >();

  constructor() {
    window.addEventListener('message', this.#handleCallResponse.bind(this));
  }

  async request(method: ExtensionMethod, params: any): Promise<any> {
    const id = generateUUID();

    return new Promise((resolve, reject) => {
      this.#requests.set(id, { resolve, reject });
      window.postMessage(
        {
          id,
          ext: 'smesh-signer',
          method,
          params,
        },
        '*'
      );
    });
  }

  #handleCallResponse(message: MessageEvent) {
    // We also will receive our own messages, that we sent.
    // We have to ignore them (they will not have a response field).
    if (
      !message.data ||
      message.data.response === null ||
      message.data.response === undefined ||
      message.data.ext !== 'smesh-signer' ||
      !this.#requests.has(message.data.id)
    ) {
      return;
    }

    if (message.data.response.error) {
      this.#requests.get(message.data.id)?.reject(message.data.response.error);
    } else {
      this.#requests.get(message.data.id)?.resolve(message.data.response);
    }

    this.#requests.delete(message.data.id);
  }
}

const nostr = {
  messenger: new Messenger(),

  async getPublicKey(): Promise<string> {
    debug('getPublicKey received');
    const pubkey = await this.messenger.request('getPublicKey', {});
    debug(`getPublicKey response:`);
    debug(pubkey);
    return pubkey;
  },

  async signEvent(event: EventTemplate): Promise<NostrEvent> {
    debug('signEvent received');
    const signedEvent = await this.messenger.request('signEvent', event);
    debug('signEvent response:');
    debug(signedEvent);
    return signedEvent;
  },

  async getRelays(): Promise<Relays> {
    debug('getRelays received');
    const relays = (await this.messenger.request('getRelays', {})) as Relays;
    debug('getRelays response:');
    debug(relays);
    return relays;
  },

  nip04: {
    that: this,

    async encrypt(peerPubkey: string, plaintext: string): Promise<string> {
      debug('nip04.encrypt received');
      const ciphertext = (await nostr.messenger.request('nip04.encrypt', {
        peerPubkey,
        plaintext,
      })) as string;
      debug('nip04.encrypt response:');
      debug(ciphertext);
      return ciphertext;
    },

    async decrypt(peerPubkey: string, ciphertext: string): Promise<string> {
      debug('nip04.decrypt received');
      const plaintext = (await nostr.messenger.request('nip04.decrypt', {
        peerPubkey,
        ciphertext,
      })) as string;
      debug('nip04.decrypt response:');
      debug(plaintext);
      return plaintext;
    },
  },

  nip44: {
    async encrypt(peerPubkey: string, plaintext: string): Promise<string> {
      debug('nip44.encrypt received');
      const ciphertext = (await nostr.messenger.request('nip44.encrypt', {
        peerPubkey,
        plaintext,
      })) as string;
      debug('nip44.encrypt response:');
      debug(ciphertext);
      return ciphertext;
    },

    async decrypt(peerPubkey: string, ciphertext: string): Promise<string> {
      debug('nip44.decrypt received');
      const plaintext = (await nostr.messenger.request('nip44.decrypt', {
        peerPubkey,
        ciphertext,
      })) as string;
      debug('nip44.decrypt response:');
      debug(plaintext);
      return plaintext;
    },
  },

  mls: {
    async init(relayURLs: string[], lastEventTS?: number): Promise<string> {
      debug('mls.init received');
      // Persist relay URLs on window so mls-bridge.mjs can pick them up even if
      // it loaded after this CustomEvent fired (DOM events are not queued).
      (window as any)._nostrMlsRelays = relayURLs;
      window.dispatchEvent(new CustomEvent('nostr-mls', { detail: { cmd: 'relays', relays: relayURLs } }));
      const r = await nostr.messenger.request('mls.init' as any, { relayURLs, lastEventTS: lastEventTS || 0 });
      debug('mls.init result: ' + r);
      return r;
    },
    async sendDM(recipient: string, content: string): Promise<string> {
      debug('mls.sendDM received: ' + recipient.slice(0, 8) + '...');
      const r = await nostr.messenger.request('mls.sendDM' as any, { recipient, content });
      debug('mls.sendDM result: ' + r);
      return r;
    },
    async subscribe(): Promise<string> {
      debug('mls.subscribe received');
      return await nostr.messenger.request('mls.subscribe' as any, {});
    },
    async publishKP(): Promise<string> {
      debug('mls.publishKP received');
      return await nostr.messenger.request('mls.publishKP' as any, {});
    },
    async listGroups(): Promise<string[]> {
      return await nostr.messenger.request('mls.listGroups' as any, {});
    },
    async deliverEvent(subId: number, eventJSON: string): Promise<string> {
      return await nostr.messenger.request('mls.deliverEvent' as any, { subId, eventJSON });
    },
  },
};

window.nostr = nostr as any;

// WebLN types (inline to avoid build issues with @common types in injected script)
interface RequestInvoiceArgs {
  amount?: string | number;
  defaultAmount?: string | number;
  minimumAmount?: string | number;
  maximumAmount?: string | number;
  defaultMemo?: string;
}

interface KeysendArgs {
  destination: string;
  amount: string | number;
  customRecords?: Record<string, string>;
}

// Create a shared messenger instance for WebLN
const weblnMessenger = nostr.messenger;

const webln = {
  enabled: false,

  async enable(): Promise<void> {
    debug('webln.enable received');
    await weblnMessenger.request('webln.enable', {});
    this.enabled = true;
    debug('webln.enable completed');
    // Dispatch webln:enabled event as per WebLN spec
    window.dispatchEvent(new Event('webln:enabled'));
  },

  async getInfo(): Promise<{ node: { alias?: string; pubkey?: string; color?: string } }> {
    debug('webln.getInfo received');
    const info = await weblnMessenger.request('webln.getInfo', {});
    debug('webln.getInfo response:');
    debug(info);
    return info;
  },

  async sendPayment(paymentRequest: string): Promise<{ preimage: string }> {
    debug('webln.sendPayment received');
    const result = await weblnMessenger.request('webln.sendPayment', { paymentRequest });
    debug('webln.sendPayment response:');
    debug(result);
    return result;
  },

  async keysend(args: KeysendArgs): Promise<{ preimage: string }> {
    debug('webln.keysend received');
    const result = await weblnMessenger.request('webln.keysend', args);
    debug('webln.keysend response:');
    debug(result);
    return result;
  },

  async makeInvoice(
    args: string | number | RequestInvoiceArgs
  ): Promise<{ paymentRequest: string }> {
    debug('webln.makeInvoice received');
    // Normalize args to RequestInvoiceArgs
    let normalizedArgs: RequestInvoiceArgs;
    if (typeof args === 'string' || typeof args === 'number') {
      normalizedArgs = { amount: args };
    } else {
      normalizedArgs = args;
    }
    const result = await weblnMessenger.request('webln.makeInvoice', normalizedArgs);
    debug('webln.makeInvoice response:');
    debug(result);
    return result;
  },

  signMessage(): Promise<{ message: string; signature: string }> {
    throw new Error('signMessage is not supported - NWC does not provide node signing capabilities');
  },

  verifyMessage(): Promise<void> {
    throw new Error('verifyMessage is not supported - NWC does not provide message verification');
  },
};

window.webln = webln as any;

// Dispatch webln:ready event to signal that webln is available
// This is dispatched on document as per the WebLN standard
document.dispatchEvent(new Event('webln:ready'));

// Listen for MLS push messages from the extension background.
// These arrive via content script relay and are dispatched as custom events
// so the page app can handle them (publish events, display DMs, etc.).
window.addEventListener('message', (event: MessageEvent) => {
  if (event.data?.ext !== 'smesh-signer' || event.data?.type !== 'mls-push') return;
  window.dispatchEvent(new CustomEvent('nostr-mls', { detail: event.data.data }));
});

const debug = function (value: any) {
  console.log('[signer]', typeof value === 'string' ? value : JSON.stringify(value));
};
