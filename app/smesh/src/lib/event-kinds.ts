/**
 * Nostr Event Kinds Database
 * Ported from app/web/src/eventKinds.js
 */

export interface EventKind {
  kind: number
  name: string
  description: string
  nip?: string | null
  spec?: string
  isReplaceable?: boolean
  isEphemeral?: boolean
  isAddressable?: boolean
  deprecated?: boolean
  template?: {
    kind: number
    content: string
    tags: string[][]
  }
}

export const eventKinds: EventKind[] = [
  {
    "kind": 0,
    "name": "Metadata",
    "description": "User profile information (name, about, picture, nip05, etc.)",
    "nip": "01",
    "isReplaceable": true,
    "template": {
      "kind": 0,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 1,
    "name": "Short Text Note",
    "description": "Short-form text post (like a tweet)",
    "nip": "01",
    "template": {
      "kind": 1,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 2,
    "name": "Recommend Relay",
    "description": "Relay recommendation",
    "nip": "01",
    "deprecated": true,
    "template": {
      "kind": 2,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 3,
    "name": "Follows",
    "description": "Following list with optional relay hints",
    "nip": "02",
    "isReplaceable": true,
    "template": {
      "kind": 3,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 4,
    "name": "Encrypted Direct Message",
    "description": "Private message using NIP-04 encryption",
    "nip": "04",
    "deprecated": true,
    "template": {
      "kind": 4,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 5,
    "name": "Event Deletion Request",
    "description": "Request to delete events",
    "nip": "09",
    "template": {
      "kind": 5,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 6,
    "name": "Repost",
    "description": "Share/repost another text note",
    "nip": "18",
    "template": {
      "kind": 6,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 7,
    "name": "Reaction",
    "description": "Like, emoji reaction to an event",
    "nip": "25",
    "template": {
      "kind": 7,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 8,
    "name": "Badge Award",
    "description": "Award a badge to someone",
    "nip": "58",
    "template": {
      "kind": 8,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 9,
    "name": "Chat Message",
    "description": "Chat message",
    "nip": "C7",
    "template": {
      "kind": 9,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 10,
    "name": "Group Chat Threaded Reply",
    "description": "Threaded reply in group chat",
    "nip": "29",
    "deprecated": true,
    "template": {
      "kind": 10,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 11,
    "name": "Thread",
    "description": "Thread event",
    "nip": "7D",
    "template": {
      "kind": 11,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 12,
    "name": "Group Thread Reply",
    "description": "Reply in group thread",
    "nip": "29",
    "deprecated": true,
    "template": {
      "kind": 12,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 13,
    "name": "Seal",
    "description": "Sealed/encrypted event wrapper",
    "nip": "59",
    "template": {
      "kind": 13,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 14,
    "name": "Direct Message",
    "description": "Private direct message using NIP-17",
    "nip": "17",
    "template": {
      "kind": 14,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 15,
    "name": "File Message",
    "description": "File message in DMs",
    "nip": "17",
    "template": {
      "kind": 15,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 16,
    "name": "Generic Repost",
    "description": "Repost any event kind",
    "nip": "18",
    "template": {
      "kind": 16,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 17,
    "name": "Reaction to Website",
    "description": "Reaction to a website URL",
    "nip": "25",
    "template": {
      "kind": 17,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 20,
    "name": "Picture",
    "description": "Picture-first feed post",
    "nip": "68",
    "template": {
      "kind": 20,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 21,
    "name": "Video Event",
    "description": "Horizontal video event",
    "nip": "71",
    "template": {
      "kind": 21,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 22,
    "name": "Short-form Video",
    "description": "Short-form portrait video (like TikTok)",
    "nip": "71",
    "template": {
      "kind": 22,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 40,
    "name": "Channel Creation",
    "description": "Create a public chat channel",
    "nip": "28",
    "template": {
      "kind": 40,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 41,
    "name": "Channel Metadata",
    "description": "Set channel name, about, picture",
    "nip": "28",
    "template": {
      "kind": 41,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 42,
    "name": "Channel Message",
    "description": "Post message in channel",
    "nip": "28",
    "template": {
      "kind": 42,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 43,
    "name": "Channel Hide Message",
    "description": "Hide a message in channel",
    "nip": "28",
    "template": {
      "kind": 43,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 44,
    "name": "Channel Mute User",
    "description": "Mute a user in channel",
    "nip": "28",
    "template": {
      "kind": 44,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 62,
    "name": "Request to Vanish",
    "description": "Request permanent deletion of all user data",
    "nip": "62",
    "template": {
      "kind": 62,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 64,
    "name": "Chess (PGN)",
    "description": "Chess game in PGN format",
    "nip": "64",
    "template": {
      "kind": 64,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 443,
    "name": "KeyPackage",
    "description": "Marmot protocol key package",
    "nip": null,
    "spec": "Marmot",
    "template": {
      "kind": 443,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 444,
    "name": "Welcome Message",
    "description": "Marmot protocol welcome message",
    "nip": null,
    "spec": "Marmot",
    "template": {
      "kind": 444,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 445,
    "name": "Group Event",
    "description": "Marmot protocol group event",
    "nip": null,
    "spec": "Marmot",
    "template": {
      "kind": 445,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 818,
    "name": "Merge Requests",
    "description": "Git merge request",
    "nip": "54",
    "template": {
      "kind": 818,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 1018,
    "name": "Poll Response",
    "description": "Response to a poll",
    "nip": "88",
    "template": {
      "kind": 1018,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 1021,
    "name": "Bid",
    "description": "Auction bid",
    "nip": "15",
    "template": {
      "kind": 1021,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 1022,
    "name": "Bid Confirmation",
    "description": "Confirmation of auction bid",
    "nip": "15",
    "template": {
      "kind": 1022,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 1040,
    "name": "OpenTimestamps",
    "description": "OpenTimestamps attestation",
    "nip": "03",
    "template": {
      "kind": 1040,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 1059,
    "name": "Gift Wrap",
    "description": "Encrypted gift-wrapped event",
    "nip": "59",
    "template": {
      "kind": 1059,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 1060,
    "name": "Gift Wrap (Kind 4)",
    "description": "Gift wrap variant for NIP-04 compatibility",
    "nip": "59",
    "template": {
      "kind": 1060,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 1063,
    "name": "File Metadata",
    "description": "Metadata for shared files",
    "nip": "94",
    "template": {
      "kind": 1063,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 1068,
    "name": "Poll",
    "description": "Create a poll",
    "nip": "88",
    "template": {
      "kind": 1068,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 1111,
    "name": "Comment",
    "description": "Comment on events or external content",
    "nip": "22",
    "template": {
      "kind": 1111,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 1222,
    "name": "Voice Message",
    "description": "Voice message",
    "nip": "A0",
    "template": {
      "kind": 1222,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 1244,
    "name": "Voice Message Comment",
    "description": "Comment on voice message",
    "nip": "A0",
    "template": {
      "kind": 1244,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 1311,
    "name": "Live Chat Message",
    "description": "Message in live stream chat",
    "nip": "53",
    "template": {
      "kind": 1311,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 1337,
    "name": "Code Snippet",
    "description": "Code snippet post",
    "nip": "C0",
    "template": {
      "kind": 1337,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 1517,
    "name": "Bitcoin Block",
    "description": "Bitcoin block data",
    "nip": null,
    "spec": "Nostrocket",
    "template": {
      "kind": 1517,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 1617,
    "name": "Patches",
    "description": "Git patches",
    "nip": "34",
    "template": {
      "kind": 1617,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 1618,
    "name": "Pull Requests",
    "description": "Git pull request",
    "nip": "34",
    "template": {
      "kind": 1618,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 1619,
    "name": "Pull Request Updates",
    "description": "Updates to git pull request",
    "nip": "34",
    "template": {
      "kind": 1619,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 1621,
    "name": "Issues",
    "description": "Git issues",
    "nip": "34",
    "template": {
      "kind": 1621,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 1622,
    "name": "Git Replies",
    "description": "Replies on git objects",
    "nip": "34",
    "deprecated": true,
    "template": {
      "kind": 1622,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 1630,
    "name": "Status",
    "description": "Git status",
    "nip": "34",
    "template": {
      "kind": 1630,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 1631,
    "name": "Status",
    "description": "Git status",
    "nip": "34",
    "template": {
      "kind": 1631,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 1632,
    "name": "Status",
    "description": "Git status",
    "nip": "34",
    "template": {
      "kind": 1632,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 1633,
    "name": "Status",
    "description": "Git status",
    "nip": "34",
    "template": {
      "kind": 1633,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 1808,
    "name": "Live Stream",
    "description": "Live streaming event",
    "nip": null,
    "spec": "zap.stream",
    "template": {
      "kind": 1808,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 1971,
    "name": "Problem Tracker",
    "description": "Problem tracking",
    "nip": null,
    "spec": "Nostrocket",
    "template": {
      "kind": 1971,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 1984,
    "name": "Reporting",
    "description": "Report content or users",
    "nip": "56",
    "template": {
      "kind": 1984,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 1985,
    "name": "Label",
    "description": "Label/tag content with namespace",
    "nip": "32",
    "template": {
      "kind": 1985,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 1986,
    "name": "Relay Reviews",
    "description": "Reviews of relays",
    "nip": null,
    "template": {
      "kind": 1986,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 1987,
    "name": "AI Embeddings",
    "description": "AI embeddings/vector lists",
    "nip": null,
    "spec": "NKBIP-02",
    "template": {
      "kind": 1987,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 2003,
    "name": "Torrent",
    "description": "Torrent magnet link",
    "nip": "35",
    "template": {
      "kind": 2003,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 2004,
    "name": "Torrent Comment",
    "description": "Comment on torrent",
    "nip": "35",
    "template": {
      "kind": 2004,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 2022,
    "name": "Coinjoin Pool",
    "description": "Coinjoin coordination",
    "nip": null,
    "spec": "joinstr",
    "template": {
      "kind": 2022,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 4550,
    "name": "Community Post Approval",
    "description": "Approve post in community",
    "nip": "72",
    "template": {
      "kind": 4550,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 5000,
    "name": "Job Request",
    "description": "Data vending machine job request (start of range)",
    "nip": "90",
    "template": {
      "kind": 5000,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 6000,
    "name": "Job Result",
    "description": "Data vending machine job result (start of range)",
    "nip": "90",
    "template": {
      "kind": 6000,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 7000,
    "name": "Job Feedback",
    "description": "Feedback on job request/result",
    "nip": "90",
    "template": {
      "kind": 7000,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 7374,
    "name": "Reserved Cashu Wallet Tokens",
    "description": "Reserved Cashu wallet tokens",
    "nip": "60",
    "template": {
      "kind": 7374,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 7375,
    "name": "Cashu Wallet Tokens",
    "description": "Cashu wallet tokens",
    "nip": "60",
    "template": {
      "kind": 7375,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 7376,
    "name": "Cashu Wallet History",
    "description": "Cashu wallet transaction history",
    "nip": "60",
    "template": {
      "kind": 7376,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 7516,
    "name": "Geocache Log",
    "description": "Geocaching log entry",
    "nip": null,
    "spec": "geocaching",
    "template": {
      "kind": 7516,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 7517,
    "name": "Geocache Proof of Find",
    "description": "Proof of geocache find",
    "nip": null,
    "spec": "geocaching",
    "template": {
      "kind": 7517,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 8000,
    "name": "Add User",
    "description": "Add user to group",
    "nip": "43",
    "template": {
      "kind": 8000,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 8001,
    "name": "Remove User",
    "description": "Remove user from group",
    "nip": "43",
    "template": {
      "kind": 8001,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 9000,
    "name": "Group Control Events",
    "description": "Group control events (start of range)",
    "nip": "29",
    "template": {
      "kind": 9000,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 9041,
    "name": "Zap Goal",
    "description": "Fundraising goal for zaps",
    "nip": "75",
    "template": {
      "kind": 9041,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 9321,
    "name": "Nutzap",
    "description": "Cashu nutzap",
    "nip": "61",
    "template": {
      "kind": 9321,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 9467,
    "name": "Tidal Login",
    "description": "Tidal streaming login",
    "nip": null,
    "spec": "Tidal-nostr",
    "template": {
      "kind": 9467,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 9734,
    "name": "Zap Request",
    "description": "Request Lightning payment",
    "nip": "57",
    "template": {
      "kind": 9734,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 9735,
    "name": "Zap",
    "description": "Lightning payment receipt",
    "nip": "57",
    "template": {
      "kind": 9735,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 9802,
    "name": "Highlights",
    "description": "Highlighted text selection",
    "nip": "84",
    "template": {
      "kind": 9802,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 10000,
    "name": "Mute List",
    "description": "List of muted users/content",
    "nip": "51",
    "isReplaceable": true,
    "template": {
      "kind": 10000,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 10001,
    "name": "Pin List",
    "description": "Pinned events",
    "nip": "51",
    "isReplaceable": true,
    "template": {
      "kind": 10001,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 10002,
    "name": "Relay List Metadata",
    "description": "User's preferred relays for read/write",
    "nip": "65",
    "isReplaceable": true,
    "template": {
      "kind": 10002,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 10003,
    "name": "Bookmark List",
    "description": "Bookmarked events",
    "nip": "51",
    "isReplaceable": true,
    "template": {
      "kind": 10003,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 10004,
    "name": "Communities List",
    "description": "Communities user belongs to",
    "nip": "51",
    "isReplaceable": true,
    "template": {
      "kind": 10004,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 10005,
    "name": "Public Chats List",
    "description": "Public chats user is in",
    "nip": "51",
    "isReplaceable": true,
    "template": {
      "kind": 10005,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 10006,
    "name": "Blocked Relays List",
    "description": "Relays user has blocked",
    "nip": "51",
    "isReplaceable": true,
    "template": {
      "kind": 10006,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 10007,
    "name": "Search Relays List",
    "description": "Preferred search relays",
    "nip": "51",
    "isReplaceable": true,
    "template": {
      "kind": 10007,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 10008,
    "name": "Relay Group Configuration",
    "description": "Relay group configuration",
    "nip": null,
    "isReplaceable": true,
    "template": {
      "kind": 10008,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 10009,
    "name": "User Groups",
    "description": "Groups user belongs to",
    "nip": "29",
    "isReplaceable": true,
    "template": {
      "kind": 10009,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 10012,
    "name": "Favorite Relays List",
    "description": "User's favorite relays",
    "nip": "51",
    "isReplaceable": true,
    "template": {
      "kind": 10012,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 10013,
    "name": "Private Event Relay List",
    "description": "Relays for private events",
    "nip": "37",
    "isReplaceable": true,
    "template": {
      "kind": 10013,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 10015,
    "name": "Interests List",
    "description": "User interests/topics",
    "nip": "51",
    "isReplaceable": true,
    "template": {
      "kind": 10015,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 10019,
    "name": "Nutzap Mint Recommendation",
    "description": "Recommended Cashu mints for nutzaps",
    "nip": "61",
    "isReplaceable": true,
    "template": {
      "kind": 10019,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 10020,
    "name": "Media Follows",
    "description": "Followed media accounts",
    "nip": "51",
    "isReplaceable": true,
    "template": {
      "kind": 10020,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 10030,
    "name": "User Emoji List",
    "description": "Custom emoji list",
    "nip": "51",
    "isReplaceable": true,
    "template": {
      "kind": 10030,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 10050,
    "name": "DM Relays List",
    "description": "Relays to receive DMs on",
    "nip": "17",
    "isReplaceable": true,
    "template": {
      "kind": 10050,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 10051,
    "name": "KeyPackage Relays List",
    "description": "Marmot key package relays",
    "nip": null,
    "isReplaceable": true,
    "spec": "Marmot",
    "template": {
      "kind": 10051,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 10063,
    "name": "User Server List",
    "description": "Blossom server list",
    "nip": null,
    "isReplaceable": true,
    "spec": "Blossom",
    "template": {
      "kind": 10063,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 10096,
    "name": "File Storage Server List",
    "description": "File storage servers",
    "nip": "96",
    "isReplaceable": true,
    "deprecated": true,
    "template": {
      "kind": 10096,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 10166,
    "name": "Relay Monitor Announcement",
    "description": "Relay monitoring announcement",
    "nip": "66",
    "isReplaceable": true,
    "template": {
      "kind": 10166,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 10312,
    "name": "Room Presence",
    "description": "Presence in live room",
    "nip": "53",
    "isReplaceable": true,
    "template": {
      "kind": 10312,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 10377,
    "name": "Proxy Announcement",
    "description": "Nostr proxy announcement",
    "nip": null,
    "isReplaceable": true,
    "spec": "Nostr Epoxy",
    "template": {
      "kind": 10377,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 11111,
    "name": "Transport Method Announcement",
    "description": "Transport method announcement",
    "nip": null,
    "isReplaceable": true,
    "spec": "Nostr Epoxy",
    "template": {
      "kind": 11111,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 12345,
    "name": "Relay Policy Configuration",
    "description": "Relay-internal policy configuration (admin only)",
    "nip": null,
    "isReplaceable": true,
    "spec": "orly",
    "template": {
      "kind": 12345,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 13004,
    "name": "JWT Binding",
    "description": "Link between JWT certificate and pubkey",
    "nip": null,
    "isReplaceable": true,
    "template": {
      "kind": 13004,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 13194,
    "name": "Wallet Service Info",
    "description": "NWC wallet service information",
    "nip": "47",
    "isReplaceable": true,
    "template": {
      "kind": 13194,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 13534,
    "name": "Membership Lists",
    "description": "Group membership lists",
    "nip": "43",
    "isReplaceable": true,
    "template": {
      "kind": 13534,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 14388,
    "name": "User Sound Effect Lists",
    "description": "Sound effect lists",
    "nip": null,
    "isReplaceable": true,
    "spec": "Corny Chat",
    "template": {
      "kind": 14388,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 17375,
    "name": "Cashu Wallet Event",
    "description": "Cashu wallet event",
    "nip": "60",
    "isReplaceable": true,
    "template": {
      "kind": 17375,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 21000,
    "name": "Lightning Pub RPC",
    "description": "Lightning.Pub RPC",
    "nip": null,
    "isEphemeral": true,
    "spec": "Lightning.Pub",
    "template": {
      "kind": 21000,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 22242,
    "name": "Client Authentication",
    "description": "Authenticate to relay",
    "nip": "42",
    "isEphemeral": true,
    "template": {
      "kind": 22242,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 23194,
    "name": "Wallet Request",
    "description": "NWC wallet request",
    "nip": "47",
    "isEphemeral": true,
    "template": {
      "kind": 23194,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 23195,
    "name": "Wallet Response",
    "description": "NWC wallet response",
    "nip": "47",
    "isEphemeral": true,
    "template": {
      "kind": 23195,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 23196,
    "name": "Wallet Notification (NIP-04)",
    "description": "NWC wallet notification (NIP-04 encrypted)",
    "nip": "47",
    "isEphemeral": true,
    "template": {
      "kind": 23196,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 23197,
    "name": "Wallet Notification",
    "description": "NWC wallet notification",
    "nip": "47",
    "isEphemeral": true,
    "template": {
      "kind": 23197,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 24133,
    "name": "Nostr Connect",
    "description": "Remote signer connection",
    "nip": "46",
    "isEphemeral": true,
    "template": {
      "kind": 24133,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 24242,
    "name": "Blobs Stored on Mediaservers",
    "description": "Blossom blob storage",
    "nip": null,
    "isEphemeral": true,
    "spec": "Blossom",
    "template": {
      "kind": 24242,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 27235,
    "name": "HTTP Auth",
    "description": "Authenticate HTTP requests",
    "nip": "98",
    "isEphemeral": true,
    "template": {
      "kind": 27235,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 28934,
    "name": "Join Request",
    "description": "Request to join group",
    "nip": "43",
    "isEphemeral": true,
    "template": {
      "kind": 28934,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 28935,
    "name": "Invite Request",
    "description": "Invite to group",
    "nip": "43",
    "isEphemeral": true,
    "template": {
      "kind": 28935,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 28936,
    "name": "Leave Request",
    "description": "Leave group request",
    "nip": "43",
    "isEphemeral": true,
    "template": {
      "kind": 28936,
      "content": "",
      "tags": []
    }
  },
  {
    "kind": 30000,
    "name": "Follow Sets",
    "description": "Categorized people lists",
    "nip": "51",
    "isAddressable": true,
    "template": {
      "kind": 30000,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 30001,
    "name": "Generic Lists",
    "description": "Generic categorized lists",
    "nip": "51",
    "isAddressable": true,
    "deprecated": true,
    "template": {
      "kind": 30001,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 30002,
    "name": "Relay Sets",
    "description": "Categorized relay lists",
    "nip": "51",
    "isAddressable": true,
    "template": {
      "kind": 30002,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 30003,
    "name": "Bookmark Sets",
    "description": "Categorized bookmark lists",
    "nip": "51",
    "isAddressable": true,
    "template": {
      "kind": 30003,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 30004,
    "name": "Curation Sets",
    "description": "Curated content sets",
    "nip": "51",
    "isAddressable": true,
    "template": {
      "kind": 30004,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 30005,
    "name": "Video Sets",
    "description": "Video playlists",
    "nip": "51",
    "isAddressable": true,
    "template": {
      "kind": 30005,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 30007,
    "name": "Kind Mute Sets",
    "description": "Muted event kinds",
    "nip": "51",
    "isAddressable": true,
    "template": {
      "kind": 30007,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 30008,
    "name": "Profile Badges",
    "description": "Badges displayed on profile",
    "nip": "58",
    "isAddressable": true,
    "template": {
      "kind": 30008,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 30009,
    "name": "Badge Definition",
    "description": "Define a badge/achievement",
    "nip": "58",
    "isAddressable": true,
    "template": {
      "kind": 30009,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 30015,
    "name": "Interest Sets",
    "description": "Interest/topic sets",
    "nip": "51",
    "isAddressable": true,
    "template": {
      "kind": 30015,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 30017,
    "name": "Stall",
    "description": "Marketplace stall definition",
    "nip": "15",
    "isAddressable": true,
    "template": {
      "kind": 30017,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 30018,
    "name": "Product",
    "description": "Marketplace product listing",
    "nip": "15",
    "isAddressable": true,
    "template": {
      "kind": 30018,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 30019,
    "name": "Marketplace UI/UX",
    "description": "Marketplace interface settings",
    "nip": "15",
    "isAddressable": true,
    "template": {
      "kind": 30019,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 30020,
    "name": "Product Sold as Auction",
    "description": "Auction product listing",
    "nip": "15",
    "isAddressable": true,
    "template": {
      "kind": 30020,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 30023,
    "name": "Long-form Content",
    "description": "Blog post, article in markdown",
    "nip": "23",
    "isAddressable": true,
    "template": {
      "kind": 30023,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 30024,
    "name": "Draft Long-form Content",
    "description": "Draft article",
    "nip": "23",
    "isAddressable": true,
    "template": {
      "kind": 30024,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 30030,
    "name": "Emoji Sets",
    "description": "Custom emoji sets",
    "nip": "51",
    "isAddressable": true,
    "template": {
      "kind": 30030,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 30040,
    "name": "Curated Publication Index",
    "description": "Publication index",
    "nip": null,
    "isAddressable": true,
    "spec": "NKBIP-01",
    "template": {
      "kind": 30040,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 30041,
    "name": "Curated Publication Content",
    "description": "Publication content",
    "nip": null,
    "isAddressable": true,
    "spec": "NKBIP-01",
    "template": {
      "kind": 30041,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 30063,
    "name": "Release Artifact Sets",
    "description": "Software release artifacts",
    "nip": "51",
    "isAddressable": true,
    "template": {
      "kind": 30063,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 30078,
    "name": "Application-specific Data",
    "description": "App-specific key-value storage",
    "nip": "78",
    "isAddressable": true,
    "template": {
      "kind": 30078,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 30166,
    "name": "Relay Discovery",
    "description": "Relay discovery/monitoring",
    "nip": "66",
    "isAddressable": true,
    "template": {
      "kind": 30166,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 30267,
    "name": "App Curation Sets",
    "description": "Curated app sets",
    "nip": "51",
    "isAddressable": true,
    "template": {
      "kind": 30267,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 30311,
    "name": "Live Event",
    "description": "Live streaming event",
    "nip": "53",
    "isAddressable": true,
    "template": {
      "kind": 30311,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 30312,
    "name": "Interactive Room",
    "description": "Interactive live room",
    "nip": "53",
    "isAddressable": true,
    "template": {
      "kind": 30312,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 30313,
    "name": "Conference Event",
    "description": "Conference/meetup event",
    "nip": "53",
    "isAddressable": true,
    "template": {
      "kind": 30313,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 30315,
    "name": "User Statuses",
    "description": "User status updates",
    "nip": "38",
    "isAddressable": true,
    "template": {
      "kind": 30315,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 30388,
    "name": "Slide Set",
    "description": "Presentation slides",
    "nip": null,
    "isAddressable": true,
    "spec": "Corny Chat",
    "template": {
      "kind": 30388,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 30402,
    "name": "Classified Listing",
    "description": "Classified ad listing",
    "nip": "99",
    "isAddressable": true,
    "template": {
      "kind": 30402,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 30403,
    "name": "Draft Classified Listing",
    "description": "Draft classified ad",
    "nip": "99",
    "isAddressable": true,
    "template": {
      "kind": 30403,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 30617,
    "name": "Repository Announcements",
    "description": "Git repository announcement",
    "nip": "34",
    "isAddressable": true,
    "template": {
      "kind": 30617,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 30618,
    "name": "Repository State Announcements",
    "description": "Git repository state",
    "nip": "34",
    "isAddressable": true,
    "template": {
      "kind": 30618,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 30818,
    "name": "Wiki Article",
    "description": "Wiki article",
    "nip": "54",
    "isAddressable": true,
    "template": {
      "kind": 30818,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 30819,
    "name": "Redirects",
    "description": "URL redirects",
    "nip": "54",
    "isAddressable": true,
    "template": {
      "kind": 30819,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 31234,
    "name": "Draft Event",
    "description": "Draft of any event",
    "nip": "37",
    "isAddressable": true,
    "template": {
      "kind": 31234,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 31388,
    "name": "Link Set",
    "description": "Link collection",
    "nip": null,
    "isAddressable": true,
    "spec": "Corny Chat",
    "template": {
      "kind": 31388,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 31890,
    "name": "Feed",
    "description": "Custom feed definition",
    "nip": null,
    "isAddressable": true,
    "spec": "NUD: Custom Feeds",
    "template": {
      "kind": 31890,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 31922,
    "name": "Date-Based Calendar Event",
    "description": "All-day calendar event",
    "nip": "52",
    "isAddressable": true,
    "template": {
      "kind": 31922,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 31923,
    "name": "Time-Based Calendar Event",
    "description": "Calendar event with time",
    "nip": "52",
    "isAddressable": true,
    "template": {
      "kind": 31923,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 31924,
    "name": "Calendar",
    "description": "Calendar definition",
    "nip": "52",
    "isAddressable": true,
    "template": {
      "kind": 31924,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 31925,
    "name": "Calendar Event RSVP",
    "description": "RSVP to calendar event",
    "nip": "52",
    "isAddressable": true,
    "template": {
      "kind": 31925,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 31989,
    "name": "Handler Recommendation",
    "description": "Recommended app for event kind",
    "nip": "89",
    "isAddressable": true,
    "template": {
      "kind": 31989,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 31990,
    "name": "Handler Information",
    "description": "App handler declaration",
    "nip": "89",
    "isAddressable": true,
    "template": {
      "kind": 31990,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 32123,
    "name": "WaveLake Track",
    "description": "WaveLake music track",
    "nip": null,
    "isAddressable": true,
    "spec": "WaveLake",
    "template": {
      "kind": 32123,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 32267,
    "name": "Software Application",
    "description": "Software application listing",
    "nip": null,
    "isAddressable": true,
    "template": {
      "kind": 32267,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 32388,
    "name": "User Room Favorites",
    "description": "Favorite rooms",
    "nip": null,
    "isAddressable": true,
    "spec": "Corny Chat",
    "template": {
      "kind": 32388,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 33388,
    "name": "High Scores",
    "description": "Game high scores",
    "nip": null,
    "isAddressable": true,
    "spec": "Corny Chat",
    "template": {
      "kind": 33388,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 34235,
    "name": "Video Event Horizontal",
    "description": "Horizontal video event",
    "nip": "71",
    "isAddressable": true,
    "template": {
      "kind": 34235,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 34236,
    "name": "Video Event Vertical",
    "description": "Vertical video event",
    "nip": "71",
    "isAddressable": true,
    "template": {
      "kind": 34236,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 34388,
    "name": "Sound Effects",
    "description": "Sound effect definitions",
    "nip": null,
    "isAddressable": true,
    "spec": "Corny Chat",
    "template": {
      "kind": 34388,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 34550,
    "name": "Community Definition",
    "description": "Define a community",
    "nip": "72",
    "isAddressable": true,
    "template": {
      "kind": 34550,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 37516,
    "name": "Geocache Listing",
    "description": "Geocache location listing",
    "nip": null,
    "isAddressable": true,
    "spec": "geocaching",
    "template": {
      "kind": 37516,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 38172,
    "name": "Cashu Mint Announcement",
    "description": "Cashu mint announcement",
    "nip": "87",
    "isAddressable": true,
    "template": {
      "kind": 38172,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 38173,
    "name": "Fedimint Announcement",
    "description": "Fedimint announcement",
    "nip": "87",
    "isAddressable": true,
    "template": {
      "kind": 38173,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 38383,
    "name": "Peer-to-peer Order Events",
    "description": "P2P trading orders",
    "nip": "69",
    "isAddressable": true,
    "template": {
      "kind": 38383,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 39000,
    "name": "Group Metadata Events",
    "description": "Group metadata (start of range)",
    "nip": "29",
    "isAddressable": true,
    "template": {
      "kind": 39000,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 39089,
    "name": "Starter Packs",
    "description": "Starter pack lists",
    "nip": "51",
    "isAddressable": true,
    "template": {
      "kind": 39089,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 39092,
    "name": "Media Starter Packs",
    "description": "Media starter packs",
    "nip": "51",
    "isAddressable": true,
    "template": {
      "kind": 39092,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 39701,
    "name": "Web Bookmarks",
    "description": "Web URL bookmarks",
    "nip": "B0",
    "isAddressable": true,
    "template": {
      "kind": 39701,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  },
  {
    "kind": 39998,
    "name": "ACL Event",
    "description": "Access control list event",
    "nip": null,
    "isAddressable": true,
    "template": {
      "kind": 39998,
      "content": "",
      "tags": [
        [
          "d",
          "identifier"
        ]
      ]
    }
  }
];

// Kind ranges for classification
export const kindRanges = {
  "regular": {
    "start": 1000,
    "end": 9999,
    "description": "Regular events - all versions kept, never replaced"
  },
  "replaceable": {
    "start": 10000,
    "end": 19999,
    "description": "Replaceable events - only latest per pubkey kept"
  },
  "ephemeral": {
    "start": 20000,
    "end": 29999,
    "description": "Ephemeral events - forwarded but not stored"
  },
  "parameterized": {
    "start": 30000,
    "end": 39999,
    "description": "Parameterized replaceable - replaced by d tag value"
  }
};

// Privileged kinds (require auth)
export const privilegedKinds = [4,13,14,15,1059,1060,30078];

// Directory kinds (public discovery)
export const directoryKinds = [0,3,5,1984,10002,10000,10050];

// Kind aliases
export const kindAliases = {
  "SetMetadata": 0,
  "Follows": 3,
  "Contacts": 3,
  "Deletion": 5,
  "MemoryHole": 1984,
  "BlockList": 10000,
  "Article": 30023,
  "CategorizedPeopleList": 30000,
  "CategorizedBookmarksList": 30001
};

export function getEventKind(kindNumber: number): EventKind | undefined {
  return eventKinds.find(k => k.kind === kindNumber);
}

export function getKindInfo(kind: number): EventKind | undefined {
  return getEventKind(kind);
}

export function getKindName(kind: number): string {
  const info = getEventKind(kind);
  return info ? info.name : `Kind ${kind}`;
}

export function searchEventKinds(query: string): EventKind[] {
  const lowerQuery = query.toLowerCase();
  return eventKinds.filter(k =>
    k.name.toLowerCase().includes(lowerQuery) ||
    k.description.toLowerCase().includes(lowerQuery) ||
    k.kind.toString().includes(query)
  );
}

export function getEventKindsByCategory() {
  return {
    regular: eventKinds.filter(k => k.kind < 10000 && !k.isReplaceable),
    replaceable: eventKinds.filter(k => k.isReplaceable),
    ephemeral: eventKinds.filter(k => k.isEphemeral),
    addressable: eventKinds.filter(k => k.isAddressable)
  };
}

export function createTemplateEvent(kindNumber: number, userPubkey: string | null = null) {
  const kindInfo = getEventKind(kindNumber);
  if (!kindInfo) {
    return {
      kind: kindNumber,
      content: "",
      tags: [] as string[][],
      created_at: Math.floor(Date.now() / 1000),
      pubkey: userPubkey || "<your_pubkey_here>"
    };
  }

  return {
    ...kindInfo.template,
    created_at: Math.floor(Date.now() / 1000),
    pubkey: userPubkey || "<your_pubkey_here>"
  };
}

export function isReplaceable(kind: number): boolean {
  if (kind === 0 || kind === 3) return true;
  return kind >= 10000 && kind < 19999;
}

export function isEphemeral(kind: number): boolean {
  return kind >= 20000 && kind < 29999;
}

export function isAddressable(kind: number): boolean {
  return kind >= 30000 && kind <= 39999;
}

export function isPrivileged(kind: number): boolean {
  return privilegedKinds.includes(kind);
}

export const kindCategories = [
  { id: "all", name: "All Kinds", filter: (_k: EventKind) => true },
  { id: "regular", name: "Regular Events (0-9999)", filter: (k: EventKind) => k.kind < 10000 && !k.isReplaceable },
  { id: "replaceable", name: "Replaceable (10000-19999)", filter: (k: EventKind) => k.isReplaceable },
  { id: "ephemeral", name: "Ephemeral (20000-29999)", filter: (k: EventKind) => k.isEphemeral },
  { id: "addressable", name: "Addressable (30000-39999)", filter: (k: EventKind) => k.isAddressable },
  { id: "social", name: "Social", filter: (k: EventKind) => [0, 1, 3, 6, 7].includes(k.kind) },
  { id: "messaging", name: "Messaging", filter: (k: EventKind) => [4, 9, 10, 11, 12, 14, 15, 40, 41, 42].includes(k.kind) },
  { id: "lists", name: "Lists", filter: (k: EventKind) => k.name.toLowerCase().includes("list") || k.name.toLowerCase().includes("set") },
  { id: "marketplace", name: "Marketplace", filter: (k: EventKind) => [30017, 30018, 30019, 30020, 1021, 1022, 30402, 30403].includes(k.kind) },
  { id: "lightning", name: "Lightning/Zaps", filter: (k: EventKind) => [9734, 9735, 9041, 9321, 7374, 7375, 7376].includes(k.kind) },
  { id: "media", name: "Media", filter: (k: EventKind) => [20, 21, 22, 1063, 1222, 1244].includes(k.kind) },
  { id: "git", name: "Git/Code", filter: (k: EventKind) => [818, 1337, 1617, 1618, 1619, 1621, 1622, 30617, 30618].includes(k.kind) },
  { id: "calendar", name: "Calendar", filter: (k: EventKind) => [31922, 31923, 31924, 31925].includes(k.kind) },
  { id: "groups", name: "Groups", filter: (k: EventKind) => (k.kind >= 9000 && k.kind <= 9030) || (k.kind >= 39000 && k.kind <= 39009) },
];
