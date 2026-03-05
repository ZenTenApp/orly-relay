# NIRC - Relay-Local IRC for Smesh

NIRC is relay-scoped public chat built on Nostr event kinds 40-44. It follows the IRC model: the relay is chanserv, npubs are identity (no nickserv), and channel owners have absolute authority over membership.

## Design Principles

- Channels are local to a single relay, not distributed
- Channels are invite-only by default
- Channel owner controls membership and moderation
- Owner can delegate mod power (mods can hide messages and block users)
- Auth is mandatory — npubs provide identity, no anonymous access
- No encryption — the relay enforces read/write access per channel owner's rules

## Event Kinds

### Kind 40 — Create Channel

Published once to create a channel. The event ID becomes the channel ID.

```json
{
  "kind": 40,
  "content": "{\"name\":\"general\",\"about\":\"General chat\",\"invite_only\":true}",
  "tags": []
}
```

Content JSON fields:
- `name` (string, required) — channel display name
- `about` (string, optional) — description
- `picture` (string, optional) — channel avatar URL
- `invite_only` (boolean, default true) — whether membership requires approval

The pubkey that signs this event is the channel owner forever.

### Kind 41 — Channel Metadata Update

Published by the channel owner to manage the channel ACL. Each new kind 41 replaces the previous one — it's a full snapshot of the current state.

```json
{
  "kind": 41,
  "content": "{\"name\":\"general\",\"about\":\"Updated description\",\"invite_only\":true}",
  "tags": [
    ["e", "<channel_id>", "<relay_url>", "root"],
    ["p", "<mod_pubkey>", "mod"],
    ["p", "<member_pubkey>", "member"],
    ["p", "<blocked_pubkey>", "blocked"]
  ]
}
```

p-tag roles:
- `mod` — can hide messages (kind 43) and block users (kind 44)
- `member` — approved to read and write in invite-only channels
- `blocked` — denied access

The channel owner is implicitly a mod and does not need to be listed.

### Kind 42 — Channel Message

Standard chat message in a channel.

```json
{
  "kind": 42,
  "content": "hello world",
  "tags": [["e", "<channel_id>", "<relay_url>", "root"]]
}
```

The `e` tag with `root` marker links the message to its parent channel.

### Kind 43 — Hide Message

Published by a mod or owner to remove a specific message from the channel.

```json
{
  "kind": 43,
  "content": "spam",
  "tags": [["e", "<message_event_id>", "<relay_url>", "root"]]
}
```

Content is an optional reason. Only kind 43 events authored by channel mods/owner are authoritative. Clients filter hidden messages from the feed.

### Kind 44 — Block User

Published by a mod or owner to block a user from the channel.

```json
{
  "kind": 44,
  "content": "disruptive behavior",
  "tags": [
    ["e", "<channel_id>", "<relay_url>", "root"],
    ["p", "<blocked_pubkey>"]
  ]
}
```

Content is an optional reason. Blocked users' messages are hidden client-side. The relay should refuse further kind 42 events from blocked users in that channel.

## Architecture

### Client Components

```
ChatProvider            — state management: channels, messages, notifications, moderation
chat.service.ts         — event construction and relay queries for kinds 40-44
ChannelList             — sidebar with channels, unread badges, mute indicators
ChannelView             — message feed, composer, mute toggle, mod actions
ChannelSettingsPanel    — owner/mod panel: invite-only toggle, member/mod/blocked management
CreateChannelDialog     — channel creation modal
ChatButton (sidebar)    — navigation with unread notification dot
ChatPage                — split layout: channel list + active channel view
```

### Notification System

- Per-channel unread counts tracked via a global WebSocket subscription to all known channels
- Unread badges shown in the channel list (numbered) and sidebar (dot)
- Muting a channel suppresses its unread count and notification dot
- Entering a channel marks it as seen
- Notification state persisted in localStorage per user pubkey:
  - `nirc:lastSeen:<pubkey>` — JSON map of channelId to timestamp
  - `nirc:muted:<pubkey>` — JSON array of muted channel IDs

### Moderation Flow

1. Channel created (kind 40) — creator is owner
2. Owner publishes kind 41 to add mods and approve members
3. Mods can hide individual messages (kind 43) or block users (kind 44)
4. Owner can toggle invite-only, add/remove mods and members, unblock users
5. All kind 41 changes are published as complete snapshots (not diffs)

### Permission Model

| Action | Owner | Mod | Member | Non-member |
|--------|-------|-----|--------|------------|
| Send message | yes | yes | yes | no (invite-only) / yes (open) |
| Hide message | yes | yes | no | no |
| Block user | yes | yes | no | no |
| Add/remove mod | yes | no | no | no |
| Add/remove member | yes | yes | no | no |
| Toggle invite-only | yes | no | no | no |
| Change channel metadata | yes | no | no | no |

## Relay Enforcement

The client publishes the intent via event kinds. Full enforcement requires the relay to:

1. Check kind 40 creator pubkey for ownership of moderation events
2. Verify kind 41 is signed by the channel owner
3. Verify kind 43/44 is signed by an owner or listed mod
4. In invite-only channels, reject kind 42 from non-members
5. Reject kind 42 from blocked users

Until relay-side enforcement is implemented, moderation is client-side only (hidden messages are filtered in the UI but still stored on the relay).
