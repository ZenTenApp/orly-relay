# Web UI Event Templates

The ORLY Web UI includes a comprehensive event template generator that helps users create properly-structured Nostr events for any of the 140+ defined event kinds.

## Overview

The Compose tab provides a "Generate Template" button that opens a searchable, categorized modal dialog. Users can browse or search for any Nostr event kind and instantly load a pre-filled template with the correct structure, tags, and example content.

## Features

### Event Kind Database (`app/web/src/eventKinds.js`)

A comprehensive JavaScript database containing:

- **140+ event kinds** from the NIPs (Nostr Implementation Possibilities) repository
- Each entry includes:
  - Kind number
  - Human-readable name
  - Description
  - NIP reference (where applicable)
  - Event type flags (replaceable, addressable, ephemeral)
  - Pre-built template with proper tag structure

### Template Selector Modal (`app/web/src/EventTemplateSelector.svelte`)

A user-friendly modal interface featuring:

- **Search functionality**: Find events by name, description, kind number, or NIP reference
- **Category filters**: Quick-filter buttons for event types:
  - All Kinds
  - Regular Events (0-9999)
  - Replaceable (10000-19999)
  - Ephemeral (20000-29999)
  - Addressable (30000-39999)
  - Domain-specific: Social, Messaging, Lists, Marketplace, Lightning, Media, Git, Calendar, Groups
- **Visual badges**: Color-coded indicators showing event type
- **NIP references**: Quick reference to the defining NIP
- **Keyboard navigation**: Escape key closes the modal

### Permission-Aware Error Handling

When publishing fails, the system provides detailed, actionable error messages:

| Error Type | Description | User Guidance |
|------------|-------------|---------------|
| Policy Error | Event kind blocked by relay policy | Contact relay administrator to allow the kind |
| Permission Error | User role insufficient | Shows current role, suggests permission upgrade |
| Kind Restriction | Event type not allowed | Policy configuration may need updating |
| Rate Limit | Too many requests | Wait before retrying |
| Size Limit | Event too large | Reduce content length |

## Usage

### Generating a Template

1. Navigate to the **Compose** tab in the Web UI
2. Click the **Generate Template** button (purple button)
3. In the modal:
   - Use the search box to find specific event types
   - Or click category tabs to filter by event type
   - Click on any event kind to select it
4. The template is loaded into the editor with:
   - Correct `kind` value
   - Proper tag structure with placeholder values
   - Example content (where applicable)
   - Current timestamp
   - Your pubkey (if logged in)

### Editing and Publishing

1. Replace placeholder values (marked with `<angle_brackets>`) with actual data
2. Click **Reformat** to clean up JSON formatting
3. Click **Sign** to sign the event with your key
4. Click **Publish** to send to the relay

### Understanding Templates

Templates use placeholder values in angle brackets that must be replaced:

```json
{
  "kind": 1,
  "content": "Your note content here",
  "tags": [
    ["p", "<pubkey_to_mention>"],
    ["e", "<event_id_to_reference>"]
  ],
  "created_at": 1702857600,
  "pubkey": "<your_pubkey_here>"
}
```

## Event Categories

### Regular Events (0-9999)
Standard events that are stored indefinitely. Examples:
- Kind 0: User Metadata
- Kind 1: Short Text Note
- Kind 7: Reaction
- Kind 1984: Reporting

### Replaceable Events (10000-19999)
Events where only the latest version is kept. Examples:
- Kind 10000: Mute List
- Kind 10002: Relay List Metadata
- Kind 13194: Wallet Info

### Ephemeral Events (20000-29999)
Events not intended for permanent storage. Examples:
- Kind 22242: Client Authentication
- Kind 24133: Nostr Connect

### Addressable Events (30000-39999)
Parameterized replaceable events identified by kind + pubkey + d-tag. Examples:
- Kind 30023: Long-form Content
- Kind 30311: Live Event
- Kind 34550: Community Definition

## API Reference

### Helper Functions in `eventKinds.js`

```javascript
import {
  eventKinds,           // Array of all event kinds
  kindCategories,       // Array of category filter definitions
  getEventKind,         // Get kind info by number
  searchEventKinds,     // Search by query string
  createTemplateEvent   // Generate template with current timestamp
} from './eventKinds.js';

// Get information about a specific kind
const kind1 = getEventKind(1);
// Returns: { kind: 1, name: "Short Text Note", description: "...", template: {...} }

// Search for kinds
const results = searchEventKinds("zap");
// Returns: Array of matching kinds

// Create a template event
const template = createTemplateEvent(1, "abc123...");
// Returns: Event object with current timestamp and provided pubkey
```

## Troubleshooting

### "Permission denied" error
Your user role does not allow publishing events. Check your role in the header badge and contact a relay administrator.

### "Policy Error: kind blocked"
The relay's policy configuration does not allow this event kind. If you're an administrator, check `ORLY_POLICY_PATH` or the Policy tab.

### "Event must be signed before publishing"
Click the **Sign** button before **Publish**. Events must be cryptographically signed before the relay will accept them.

### Template not loading
Ensure JavaScript is enabled and the page has fully loaded. Try refreshing the page.

## Related Documentation

- [POLICY_USAGE_GUIDE.md](./POLICY_USAGE_GUIDE.md) - Policy configuration for event restrictions
- [POLICY_CONFIGURATION_REFERENCE.md](./POLICY_CONFIGURATION_REFERENCE.md) - Policy rule reference
- [NIPs Repository](https://github.com/nostr-protocol/nips) - Official Nostr protocol specifications
