export type Nip07Method =
  | 'signEvent'
  | 'getPublicKey'
  | 'getRelays'
  | 'nip04.encrypt'
  | 'nip04.decrypt'
  | 'nip44.encrypt'
  | 'nip44.decrypt'
  | 'mls.init'
  | 'mls.sendDM'
  | 'mls.subscribe'
  | 'mls.publishKP'
  | 'mls.listGroups'
  | 'mls.deliverEvent'
  | 'mls.backupGroups'
  | 'mls.restoreGroups'
  | 'mls.ratchetGroup'
  | 'mls.*';

export type Nip07MethodPolicy = 'allow' | 'deny';

export type WeblnMethod =
  | 'webln.enable'
  | 'webln.getInfo'
  | 'webln.sendPayment'
  | 'webln.makeInvoice'
  | 'webln.keysend';

export type ExtensionMethod = Nip07Method | WeblnMethod;
