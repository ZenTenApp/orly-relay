// TinyJS Runtime — Crypto stub for Service Worker
// Service workers cannot use dynamic import(), and the SW doesn't need crypto.

export async function init() {}
export function PubKeyFromSecKey() { return null; }
export function SignSchnorr() { return null; }
export function VerifySchnorr() { return false; }
