// TinyJS Runtime — NIP-07 Browser Signer Bridge

export function HasSigner() {
  return typeof window !== 'undefined' && !!window.nostr;
}

export function GetPublicKey(fn) {
  if (!window.nostr) { fn(''); return; }
  window.nostr.getPublicKey()
    .then(pubkey => fn(pubkey))
    .catch(() => fn(''));
}

export function SignEvent(eventJSON, fn) {
  if (!window.nostr) { console.error('signer: window.nostr not available'); fn(''); return; }
  try {
    const ev = JSON.parse(eventJSON);
    window.nostr.signEvent(ev)
      .then(signed => fn(JSON.stringify(signed)))
      .catch(e => { console.error('signer: signEvent rejected:', e); fn(''); });
  } catch(e) { console.error('signer: signEvent exception:', e); fn(''); }
}

export function Nip04Decrypt(peerPubkey, ciphertext, fn) {
  if (!window.nostr || !window.nostr.nip04) { fn(''); return; }
  window.nostr.nip04.decrypt(peerPubkey, ciphertext)
    .then(plain => fn(plain))
    .catch(() => fn(''));
}

export function Nip04Encrypt(peerPubkey, plaintext, fn) {
  if (!window.nostr || !window.nostr.nip04) { fn(''); return; }
  window.nostr.nip04.encrypt(peerPubkey, plaintext)
    .then(ct => fn(ct))
    .catch(() => fn(''));
}

export function Nip44Decrypt(peerPubkey, ciphertext, fn) {
  if (!window.nostr || !window.nostr.nip44) { fn(''); return; }
  window.nostr.nip44.decrypt(peerPubkey, ciphertext)
    .then(plain => fn(plain))
    .catch(() => fn(''));
}

export function Nip44Encrypt(peerPubkey, plaintext, fn) {
  if (!window.nostr || !window.nostr.nip44) { fn(''); return; }
  window.nostr.nip44.encrypt(peerPubkey, plaintext)
    .then(ct => fn(ct))
    .catch(() => fn(''));
}
