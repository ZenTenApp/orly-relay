# Applesauce Library Reference

A collection of TypeScript libraries for building Nostr web clients. Powers the noStrudel client.

**Repository:** https://github.com/hzrd149/applesauce
**Documentation:** https://hzrd149.github.io/applesauce/

---

## Packages Overview

| Package | Description |
|---------|-------------|
| `applesauce-core` | Event utilities, key management, protocols, event storage |
| `applesauce-relay` | Relay connection management with auto-reconnect |
| `applesauce-signers` | Signing interfaces for multiple providers |
| `applesauce-loaders` | High-level data loading for common Nostr patterns |
| `applesauce-factory` | Event creation and manipulation utilities |
| `applesauce-react` | React hooks and providers |

## Installation

```bash
# Core package
npm install applesauce-core

# With React support
npm install applesauce-core applesauce-react

# Full stack
npm install applesauce-core applesauce-relay applesauce-signers applesauce-loaders applesauce-factory
```

---

## Core Concepts

### Philosophy
- **Reactive Architecture**: Built on RxJS observables for event-driven programming
- **No Vendor Lock-in**: Generic interfaces compatible with other Nostr libraries
- **Modularity**: Tree-shakeable packages - include only what you need

---

## EventStore

The foundational class for managing Nostr event state.

### Creation

```typescript
import { EventStore } from "applesauce-core";

// Memory-only store
const eventStore = new EventStore();

// With persistent database
import { BetterSqlite3EventDatabase } from "applesauce-core/database";
const database = new BetterSqlite3EventDatabase("./events.db");
const eventStore = new EventStore(database);
```

### Event Management Methods

```typescript
// Add event (returns existing if duplicate, null if rejected)
eventStore.add(event, relay?);

// Remove events
eventStore.remove(id);
eventStore.remove(event);
eventStore.removeByFilters(filters);

// Update event (notify store of modifications)
eventStore.update(event);
```

### Query Methods

```typescript
// Check existence
eventStore.hasEvent(id);

// Get single event
eventStore.getEvent(id);

// Get by filters
eventStore.getByFilters(filters);

// Get sorted timeline (newest first)
eventStore.getTimeline(filters);

// Replaceable events
eventStore.hasReplaceable(kind, pubkey);
eventStore.getReplaceable(kind, pubkey, identifier?);
eventStore.getReplaceableHistory(kind, pubkey, identifier?); // requires keepOldVersions: true
```

### Observable Subscriptions

```typescript
// Single event updates
eventStore.event(id).subscribe(event => { ... });

// All matching events
eventStore.filters(filters, onlyNew?).subscribe(events => { ... });

// Sorted event arrays
eventStore.timeline(filters, onlyNew?).subscribe(events => { ... });

// Replaceable events
eventStore.replaceable(kind, pubkey).subscribe(event => { ... });

// Addressable events
eventStore.addressable(kind, pubkey, identifier).subscribe(event => { ... });
```

### Helper Subscriptions

```typescript
// Profile (kind 0)
eventStore.profile(pubkey).subscribe(profile => { ... });

// Contacts (kind 3)
eventStore.contacts(pubkey).subscribe(contacts => { ... });

// Mutes (kind 10000)
eventStore.mutes(pubkey).subscribe(mutes => { ... });

// Mailboxes/NIP-65 relays (kind 10002)
eventStore.mailboxes(pubkey).subscribe(mailboxes => { ... });

// Blossom servers (kind 10063)
eventStore.blossomServers(pubkey).subscribe(servers => { ... });

// Reactions (kind 7)
eventStore.reactions(event).subscribe(reactions => { ... });

// Thread replies
eventStore.thread(eventId).subscribe(thread => { ... });

// Comments
eventStore.comments(event).subscribe(comments => { ... });
```

### NIP-91 AND Operators

```typescript
// Use & prefix for tags requiring ALL values
eventStore.filters({
  kinds: [1],
  "&t": ["meme", "cat"],    // Must have BOTH tags
  "#t": ["black", "white"]  // Must have black OR white
});
```

### Fallback Loaders

```typescript
// Custom async loaders for missing events
eventStore.eventLoader = async (pointer) => {
  // Fetch from relay and return event
};

eventStore.replaceableLoader = async (pointer) => { ... };
eventStore.addressableLoader = async (pointer) => { ... };
```

### Configuration

```typescript
const eventStore = new EventStore();

// Keep all versions of replaceable events
eventStore.keepOldVersions = true;

// Keep expired events (default: removes them)
eventStore.keepExpired = true;

// Custom verification
eventStore.verifyEvent = (event) => verifySignature(event);

// Model memory duration (default: 60000ms)
eventStore.modelKeepWarm = 60000;
```

### Memory Management

```typescript
// Mark event as in-use
eventStore.claim(event, claimId);

// Check if claimed
eventStore.isClaimed(event);

// Remove claims
eventStore.removeClaim(event, claimId);
eventStore.clearClaim(event);

// Prune unclaimed events
eventStore.prune(count?);

// Iterate unclaimed (LRU ordered)
for (const event of eventStore.unclaimed()) { ... }
```

### Observable Streams

```typescript
// New events added
eventStore.insert$.subscribe(event => { ... });

// Events modified
eventStore.update$.subscribe(event => { ... });

// Events deleted
eventStore.remove$.subscribe(event => { ... });
```

---

## EventFactory

Primary interface for creating, building, and modifying Nostr events.

### Initialization

```typescript
import { EventFactory } from "applesauce-factory";

// Basic
const factory = new EventFactory();

// With signer
const factory = new EventFactory({ signer: mySigner });

// Full configuration
const factory = new EventFactory({
  signer: { getPublicKey, signEvent, nip04?, nip44? },
  client: { name: "MyApp", address: "31990:..." },
  getEventRelayHint: (eventId) => "wss://relay.example.com",
  getPubkeyRelayHint: (pubkey) => "wss://relay.example.com",
  emojis: emojiArray
});
```

### Blueprint-Based Creation

```typescript
import { NoteBlueprint, ReactionBlueprint } from "applesauce-factory/blueprints";

// Pattern 1: Constructor + arguments
const note = await factory.create(NoteBlueprint, "Hello Nostr!");
const reaction = await factory.create(ReactionBlueprint, event, "+");

// Pattern 2: Direct blueprint call
const note = await factory.create(NoteBlueprint("Hello Nostr!"));
```

### Custom Event Building

```typescript
import { setContent, includeNameValueTag, includeSingletonTag } from "applesauce-factory/operations";

const event = await factory.build(
  { kind: 30023 },
  setContent("Article content..."),
  includeNameValueTag(["title", "My Title"]),
  includeSingletonTag(["d", "article-id"])
);
```

### Event Modification

```typescript
import { addPubkeyTag } from "applesauce-factory/operations";

// Full modification
const modified = await factory.modify(existingEvent, operations);

// Tags only
const updated = await factory.modifyTags(existingEvent, addPubkeyTag("pubkey"));
```

### Helper Methods

```typescript
// Short text note (kind 1)
await factory.note("Hello world!", options?);

// Reply to note
await factory.noteReply(parentEvent, "My reply");

// Reaction (kind 7)
await factory.reaction(event, "🔥");

// Event deletion
await factory.delete(events, reason?);

// Repost/share
await factory.share(event);

// NIP-22 comment
await factory.comment(article, "Great article!");
```

### Available Blueprints

| Blueprint | Description |
|-----------|-------------|
| `NoteBlueprint(content, options?)` | Standard text notes (kind 1) |
| `CommentBlueprint(parent, content, options?)` | Comments on events |
| `NoteReplyBlueprint(parent, content, options?)` | Replies to notes |
| `ReactionBlueprint(event, emoji?)` | Emoji reactions (kind 7) |
| `ShareBlueprint(event, options?)` | Event shares/reposts |
| `PicturePostBlueprint(pictures, content, options?)` | Image posts |
| `FileMetadataBlueprint(file, options?)` | File metadata |
| `DeleteBlueprint(events)` | Event deletion |
| `LiveStreamBlueprint(title, options?)` | Live streams |

---

## Models

Pre-built reactive models for common data patterns.

### Built-in Models

```typescript
import { ProfileModel, TimelineModel, RepliesModel } from "applesauce-core/models";

// Profile subscription (kind 0)
const profile$ = eventStore.model(ProfileModel, pubkey);

// Timeline subscription
const timeline$ = eventStore.model(TimelineModel, { kinds: [1] });

// Replies subscription (NIP-10 and NIP-22)
const replies$ = eventStore.model(RepliesModel, event);
```

### Custom Models

```typescript
import { Model } from "applesauce-core";

const AppSettingsModel: Model<AppSettings, [string]> = (appId) => {
  return (store) => {
    return store.addressable(30078, store.pubkey, appId).pipe(
      map(event => event ? JSON.parse(event.content) : null)
    );
  };
};

// Usage
const settings$ = eventStore.model(AppSettingsModel, "my-app");
```

---

## Helper Functions

### Event Utilities

```typescript
import {
  isEvent,
  markFromCache,
  isFromCache,
  getTagValue,
  getIndexableTags
} from "applesauce-core/helpers";
```

### Profile Management

```typescript
import { getProfileContent, isValidProfile } from "applesauce-core/helpers";

const profile = getProfileContent(kind0Event);
const valid = isValidProfile(profile);
```

### Relay Configuration

```typescript
import { getInboxes, getOutboxes } from "applesauce-core/helpers";

const inboxRelays = getInboxes(kind10002Event);
const outboxRelays = getOutboxes(kind10002Event);
```

### Zap Processing

```typescript
import {
  isValidZap,
  getZapSender,
  getZapRecipient,
  getZapPayment
} from "applesauce-core/helpers";

if (isValidZap(zapEvent)) {
  const sender = getZapSender(zapEvent);
  const recipient = getZapRecipient(zapEvent);
  const payment = getZapPayment(zapEvent);
}
```

### Lightning Parsing

```typescript
import { parseBolt11, parseLNURLOrAddress } from "applesauce-core/helpers";

const invoice = parseBolt11(bolt11String);
const lnurl = parseLNURLOrAddress(addressOrUrl);
```

### Pointer Creation

```typescript
import {
  getEventPointerFromETag,
  getAddressPointerFromATag,
  getProfilePointerFromPTag,
  getAddressPointerForEvent
} from "applesauce-core/helpers";
```

### Tag Validation

```typescript
import { isETag, isATag, isPTag, isDTag, isRTag, isTTag } from "applesauce-core/helpers";
```

### Media Detection

```typescript
import { isAudioURL, isVideoURL, isImageURL, isStreamURL } from "applesauce-core/helpers";

if (isImageURL(url)) {
  // Handle image
}
```

### Hidden Tags (NIP-51/60)

```typescript
import {
  canHaveHiddenTags,
  hasHiddenTags,
  getHiddenTags,
  unlockHiddenTags,
  modifyEventTags
} from "applesauce-core/helpers";
```

### Comment Operations

```typescript
import { getCommentRootPointer, getCommentReplyPointer } from "applesauce-core/helpers";
```

### Deletion Handling

```typescript
import { getDeleteIds, getDeleteCoordinates } from "applesauce-core/helpers";
```

---

## Common Patterns

### Basic Nostr Client Setup

```typescript
import { EventStore } from "applesauce-core";
import { EventFactory } from "applesauce-factory";
import { NoteBlueprint } from "applesauce-factory/blueprints";

// Initialize stores
const eventStore = new EventStore();
const factory = new EventFactory({ signer: mySigner });

// Subscribe to timeline
eventStore.timeline({ kinds: [1], limit: 50 }).subscribe(notes => {
  renderNotes(notes);
});

// Create a new note
const note = await factory.create(NoteBlueprint, "Hello Nostr!");

// Add to store
eventStore.add(note);
```

### Profile Display

```typescript
// Subscribe to profile updates
eventStore.profile(pubkey).subscribe(event => {
  if (event) {
    const profile = getProfileContent(event);
    displayProfile(profile);
  }
});
```

### Reactive Reactions

```typescript
// Subscribe to reactions on an event
eventStore.reactions(targetEvent).subscribe(reactions => {
  const likeCount = reactions.filter(r => r.content === "+").length;
  updateLikeButton(likeCount);
});

// Add a reaction
const reaction = await factory.reaction(targetEvent, "🔥");
eventStore.add(reaction);
```

### Thread Loading

```typescript
eventStore.thread(rootEventId).subscribe(thread => {
  renderThread(thread);
});
```

---

## Nostr Event Kinds Reference

| Kind | Description |
|------|-------------|
| 0 | Profile metadata |
| 1 | Short text note |
| 3 | Contact list |
| 7 | Reaction |
| 10000 | Mute list |
| 10002 | Relay list (NIP-65) |
| 10063 | Blossom servers |
| 30023 | Long-form content |
| 30078 | App-specific data (NIP-78) |

---

## Resources

- **Documentation:** https://hzrd149.github.io/applesauce/
- **GitHub:** https://github.com/hzrd149/applesauce
- **TypeDoc API:** Check the repository for full API documentation
- **Example App:** noStrudel client demonstrates real-world usage
