/**
 * Comprehensive Nostr Event Kinds Database
 * Based on NIPs (Nostr Implementation Possibilities) from https://github.com/nostr-protocol/nips
 *
 * Each entry contains:
 * - kind: The numeric event kind
 * - name: Human-readable name
 * - description: Brief description of the event type
 * - nip: Related NIP number(s)
 * - template: Event template with placeholder fields
 * - isReplaceable: Whether the event is replaceable (kinds 0, 3, 10000-19999)
 * - isAddressable: Whether the event is addressable/parameterized replaceable (kinds 30000-39999)
 * - isEphemeral: Whether the event is ephemeral (kinds 20000-29999)
 */

export const eventKinds = [
    // Regular Events (0-9999)
    {
        kind: 0,
        name: "User Metadata",
        description: "Profile metadata (name, about, picture, etc.)",
        nip: "01",
        isReplaceable: true,
        template: {
            kind: 0,
            content: JSON.stringify({
                name: "Your Name",
                about: "A brief description about yourself",
                picture: "https://example.com/avatar.jpg",
                banner: "https://example.com/banner.jpg",
                nip05: "you@example.com",
                lud16: "you@walletofsatoshi.com",
                website: "https://example.com"
            }, null, 2),
            tags: []
        }
    },
    {
        kind: 1,
        name: "Short Text Note",
        description: "Plain text note (like a tweet)",
        nip: "01",
        template: {
            kind: 1,
            content: "Your note content here",
            tags: []
        }
    },
    {
        kind: 2,
        name: "Recommend Relay",
        description: "Recommend a relay to followers (deprecated)",
        nip: "01",
        template: {
            kind: 2,
            content: "wss://relay.example.com",
            tags: []
        }
    },
    {
        kind: 3,
        name: "Follows",
        description: "Contact list / follow list",
        nip: "02",
        isReplaceable: true,
        template: {
            kind: 3,
            content: "",
            tags: [
                ["p", "<pubkey_hex_to_follow>", "wss://relay.example.com", "nickname"]
            ]
        }
    },
    {
        kind: 4,
        name: "Encrypted Direct Messages",
        description: "NIP-04 encrypted direct message (deprecated, use kind 14)",
        nip: "04",
        template: {
            kind: 4,
            content: "<encrypted_content>",
            tags: [
                ["p", "<recipient_pubkey_hex>"]
            ]
        }
    },
    {
        kind: 5,
        name: "Event Deletion Request",
        description: "Request to delete events",
        nip: "09",
        template: {
            kind: 5,
            content: "Reason for deletion (optional)",
            tags: [
                ["e", "<event_id_to_delete>"],
                ["k", "<kind_of_event_to_delete>"]
            ]
        }
    },
    {
        kind: 6,
        name: "Repost",
        description: "Repost/boost a kind 1 note",
        nip: "18",
        template: {
            kind: 6,
            content: "",
            tags: [
                ["e", "<event_id_to_repost>", "wss://relay.example.com"],
                ["p", "<original_author_pubkey>"]
            ]
        }
    },
    {
        kind: 7,
        name: "Reaction",
        description: "Reaction to an event (like, emoji, etc.)",
        nip: "25",
        template: {
            kind: 7,
            content: "+",
            tags: [
                ["e", "<event_id_to_react_to>"],
                ["p", "<author_of_event>"]
            ]
        }
    },
    {
        kind: 8,
        name: "Badge Award",
        description: "Award a badge to a user",
        nip: "58",
        template: {
            kind: 8,
            content: "",
            tags: [
                ["a", "30009:<badge_creator_pubkey>:<badge_d_tag>"],
                ["p", "<recipient_pubkey>"]
            ]
        }
    },
    {
        kind: 9,
        name: "Chat Message",
        description: "Group chat message",
        nip: "29",
        template: {
            kind: 9,
            content: "Your chat message",
            tags: [
                ["h", "<group_id>"]
            ]
        }
    },
    {
        kind: 10,
        name: "Group Chat Threaded Reply",
        description: "Threaded reply in group chat",
        nip: "29",
        template: {
            kind: 10,
            content: "Your reply",
            tags: [
                ["h", "<group_id>"],
                ["e", "<root_event_id>", "", "root"],
                ["e", "<reply_to_event_id>", "", "reply"]
            ]
        }
    },
    {
        kind: 11,
        name: "Thread",
        description: "Thread starter",
        nip: "29",
        template: {
            kind: 11,
            content: "Thread content",
            tags: [
                ["h", "<group_id>"]
            ]
        }
    },
    {
        kind: 12,
        name: "Group Thread Reply",
        description: "Reply in a group thread",
        nip: "29",
        template: {
            kind: 12,
            content: "Reply content",
            tags: [
                ["h", "<group_id>"],
                ["e", "<thread_root_id>", "", "root"]
            ]
        }
    },
    {
        kind: 13,
        name: "Seal",
        description: "Encrypted wrapper for gift wraps",
        nip: "59",
        template: {
            kind: 13,
            content: "<encrypted_rumor>",
            tags: []
        }
    },
    {
        kind: 14,
        name: "Direct Message",
        description: "NIP-17 private direct message (recommended)",
        nip: "17",
        template: {
            kind: 14,
            content: "Your private message content",
            tags: [
                ["p", "<recipient_pubkey>"]
            ]
        }
    },
    {
        kind: 15,
        name: "File Message",
        description: "File attachment in DM",
        nip: "17",
        template: {
            kind: 15,
            content: "Optional caption",
            tags: [
                ["p", "<recipient_pubkey>"],
                ["url", "https://example.com/file.pdf"],
                ["m", "application/pdf"],
                ["size", "12345"]
            ]
        }
    },
    {
        kind: 16,
        name: "Generic Repost",
        description: "Repost any event kind",
        nip: "18",
        template: {
            kind: 16,
            content: "",
            tags: [
                ["e", "<event_id_to_repost>", "wss://relay.example.com"],
                ["p", "<original_author_pubkey>"],
                ["k", "<original_event_kind>"]
            ]
        }
    },
    {
        kind: 17,
        name: "Reaction to Website",
        description: "React to a web page URL",
        nip: "25",
        template: {
            kind: 17,
            content: "+",
            tags: [
                ["r", "https://example.com/page"]
            ]
        }
    },
    {
        kind: 20,
        name: "Picture",
        description: "Picture post",
        nip: "68",
        template: {
            kind: 20,
            content: "Photo description",
            tags: [
                ["url", "https://example.com/photo.jpg"],
                ["m", "image/jpeg"],
                ["dim", "1920x1080"],
                ["blurhash", "<blurhash_string>"]
            ]
        }
    },
    {
        kind: 21,
        name: "Video Event",
        description: "Video post (horizontal)",
        nip: "71",
        template: {
            kind: 21,
            content: "Video description",
            tags: [
                ["url", "https://example.com/video.mp4"],
                ["m", "video/mp4"],
                ["dim", "1920x1080"],
                ["duration", "120"],
                ["thumb", "https://example.com/thumbnail.jpg"]
            ]
        }
    },
    {
        kind: 22,
        name: "Short-form Portrait Video",
        description: "Short vertical video (like TikTok/Reels)",
        nip: "71",
        template: {
            kind: 22,
            content: "Short video description",
            tags: [
                ["url", "https://example.com/short.mp4"],
                ["m", "video/mp4"],
                ["dim", "1080x1920"],
                ["duration", "30"]
            ]
        }
    },
    {
        kind: 40,
        name: "Channel Creation",
        description: "Create a public chat channel",
        nip: "28",
        template: {
            kind: 40,
            content: JSON.stringify({
                name: "Channel Name",
                about: "Channel description",
                picture: "https://example.com/channel-pic.jpg"
            }, null, 2),
            tags: []
        }
    },
    {
        kind: 41,
        name: "Channel Metadata",
        description: "Update channel metadata",
        nip: "28",
        template: {
            kind: 41,
            content: JSON.stringify({
                name: "Updated Channel Name",
                about: "Updated description",
                picture: "https://example.com/new-pic.jpg"
            }, null, 2),
            tags: [
                ["e", "<channel_create_event_id>"]
            ]
        }
    },
    {
        kind: 42,
        name: "Channel Message",
        description: "Message in a public channel",
        nip: "28",
        template: {
            kind: 42,
            content: "Your message in the channel",
            tags: [
                ["e", "<channel_create_event_id>", "wss://relay.example.com", "root"]
            ]
        }
    },
    {
        kind: 43,
        name: "Channel Hide Message",
        description: "Hide a message in a channel (moderator action)",
        nip: "28",
        template: {
            kind: 43,
            content: JSON.stringify({ reason: "Spam" }, null, 2),
            tags: [
                ["e", "<message_event_id_to_hide>"]
            ]
        }
    },
    {
        kind: 44,
        name: "Channel Mute User",
        description: "Mute a user in a channel (moderator action)",
        nip: "28",
        template: {
            kind: 44,
            content: JSON.stringify({ reason: "Harassment" }, null, 2),
            tags: [
                ["p", "<pubkey_to_mute>"]
            ]
        }
    },
    {
        kind: 62,
        name: "Request to Vanish",
        description: "Request relays to delete all your events",
        nip: "62",
        template: {
            kind: 62,
            content: "",
            tags: [
                ["relay", "wss://relay1.example.com"],
                ["relay", "wss://relay2.example.com"]
            ]
        }
    },
    {
        kind: 64,
        name: "Chess (PGN)",
        description: "Chess game in PGN format",
        nip: "64",
        template: {
            kind: 64,
            content: "1. e4 e5 2. Nf3 Nc6 3. Bb5",
            tags: [
                ["p", "<white_player_pubkey>"],
                ["p", "<black_player_pubkey>"]
            ]
        }
    },
    {
        kind: 443,
        name: "KeyPackage",
        description: "MLS KeyPackage for group messaging",
        nip: "104",
        template: {
            kind: 443,
            content: "<base64_encoded_keypackage>",
            tags: []
        }
    },
    {
        kind: 444,
        name: "Welcome Message",
        description: "MLS Welcome message for group join",
        nip: "104",
        template: {
            kind: 444,
            content: "<base64_encoded_welcome>",
            tags: [
                ["p", "<recipient_pubkey>"]
            ]
        }
    },
    {
        kind: 445,
        name: "Group Event",
        description: "MLS group event (commit, proposal)",
        nip: "104",
        template: {
            kind: 445,
            content: "<base64_encoded_group_event>",
            tags: [
                ["h", "<group_id>"]
            ]
        }
    },
    {
        kind: 818,
        name: "Merge Requests",
        description: "Git merge request",
        nip: "34",
        template: {
            kind: 818,
            content: "Merge request description",
            tags: [
                ["a", "30617:<repo_owner_pubkey>:<repo_identifier>"],
                ["p", "<repo_owner_pubkey>"],
                ["branch", "feature-branch"],
                ["base", "main"]
            ]
        }
    },
    {
        kind: 1018,
        name: "Poll Response",
        description: "Response to a poll",
        nip: "88",
        template: {
            kind: 1018,
            content: "",
            tags: [
                ["e", "<poll_event_id>"],
                ["response", "0"]
            ]
        }
    },
    {
        kind: 1021,
        name: "Bid",
        description: "Bid on an auction",
        nip: "15",
        template: {
            kind: 1021,
            content: "",
            tags: [
                ["a", "30020:<seller_pubkey>:<product_id>"],
                ["amount", "10000", "sat"]
            ]
        }
    },
    {
        kind: 1022,
        name: "Bid Confirmation",
        description: "Confirm a winning bid",
        nip: "15",
        template: {
            kind: 1022,
            content: "",
            tags: [
                ["e", "<bid_event_id>"],
                ["p", "<winning_bidder_pubkey>"]
            ]
        }
    },
    {
        kind: 1040,
        name: "OpenTimestamps",
        description: "OpenTimestamps attestation",
        nip: "03",
        template: {
            kind: 1040,
            content: "<base64_ots_proof>",
            tags: [
                ["e", "<event_id_to_timestamp>"]
            ]
        }
    },
    {
        kind: 1059,
        name: "Gift Wrap",
        description: "Wrapper for private events",
        nip: "59",
        template: {
            kind: 1059,
            content: "<encrypted_seal>",
            tags: [
                ["p", "<recipient_pubkey>"]
            ]
        }
    },
    {
        kind: 1063,
        name: "File Metadata",
        description: "Metadata for a file",
        nip: "94",
        template: {
            kind: 1063,
            content: "File description",
            tags: [
                ["url", "https://example.com/file.pdf"],
                ["m", "application/pdf"],
                ["x", "<sha256_hash>"],
                ["size", "1048576"],
                ["dim", "800x600"],
                ["blurhash", "<blurhash>"]
            ]
        }
    },
    {
        kind: 1068,
        name: "Poll",
        description: "Create a poll",
        nip: "88",
        template: {
            kind: 1068,
            content: "What is your favorite color?",
            tags: [
                ["option", "0", "Red"],
                ["option", "1", "Blue"],
                ["option", "2", "Green"],
                ["poll_type", "single"],
                ["closed_at", "1704067200"]
            ]
        }
    },
    {
        kind: 1111,
        name: "Comment",
        description: "Comment on any addressable event or URL",
        nip: "22",
        template: {
            kind: 1111,
            content: "Your comment here",
            tags: [
                ["K", "<root_event_kind>"],
                ["E", "<root_event_id>", "wss://relay.example.com", "<root_pubkey>"],
                ["A", "<root_event_kind>:<pubkey>:<d_tag>", "wss://relay.example.com"],
                ["k", "<parent_event_kind>"],
                ["e", "<parent_event_id>", "wss://relay.example.com", "<parent_pubkey>"]
            ]
        }
    },
    {
        kind: 1222,
        name: "Voice Message",
        description: "Audio voice message",
        nip: "88",
        template: {
            kind: 1222,
            content: "Transcription (optional)",
            tags: [
                ["url", "https://example.com/voice.ogg"],
                ["m", "audio/ogg"],
                ["duration", "30"]
            ]
        }
    },
    {
        kind: 1244,
        name: "Voice Message Comment",
        description: "Voice message reply to content",
        nip: "88",
        template: {
            kind: 1244,
            content: "",
            tags: [
                ["url", "https://example.com/reply.ogg"],
                ["e", "<original_event_id>"]
            ]
        }
    },
    {
        kind: 1311,
        name: "Live Chat Message",
        description: "Chat message during a live stream",
        nip: "53",
        template: {
            kind: 1311,
            content: "Live chat message",
            tags: [
                ["a", "30311:<streamer_pubkey>:<stream_d_tag>", "wss://relay.example.com"]
            ]
        }
    },
    {
        kind: 1337,
        name: "Code Snippet",
        description: "Share a code snippet",
        nip: "XX",
        template: {
            kind: 1337,
            content: "console.log('Hello, World!');",
            tags: [
                ["language", "javascript"],
                ["filename", "hello.js"]
            ]
        }
    },
    {
        kind: 1617,
        name: "Patches",
        description: "Git patches",
        nip: "34",
        template: {
            kind: 1617,
            content: "diff --git a/file.txt b/file.txt\n...",
            tags: [
                ["a", "30617:<repo_owner_pubkey>:<repo_identifier>"],
                ["commit", "<commit_hash>"]
            ]
        }
    },
    {
        kind: 1618,
        name: "Pull Requests",
        description: "Git pull request issues",
        nip: "34",
        template: {
            kind: 1618,
            content: "Pull request description",
            tags: [
                ["a", "30617:<repo_owner_pubkey>:<repo_identifier>"],
                ["branch", "feature-branch"],
                ["base", "main"]
            ]
        }
    },
    {
        kind: 1619,
        name: "Pull Request Updates",
        description: "Updates to pull requests",
        nip: "34",
        template: {
            kind: 1619,
            content: "Update description",
            tags: [
                ["e", "<original_pr_event_id>"]
            ]
        }
    },
    {
        kind: 1621,
        name: "Issues",
        description: "Git repository issues",
        nip: "34",
        template: {
            kind: 1621,
            content: "Issue description",
            tags: [
                ["a", "30617:<repo_owner_pubkey>:<repo_identifier>"],
                ["title", "Issue title"],
                ["t", "bug"]
            ]
        }
    },
    {
        kind: 1622,
        name: "Git Replies",
        description: "Replies to git issues/PRs",
        nip: "34",
        template: {
            kind: 1622,
            content: "Reply content",
            tags: [
                ["e", "<issue_or_pr_event_id>", "", "root"],
                ["p", "<original_author_pubkey>"]
            ]
        }
    },
    {
        kind: 1630,
        name: "Status (General)",
        description: "General status update",
        nip: "38",
        template: {
            kind: 1630,
            content: "Currently working on something cool",
            tags: [
                ["d", "general"]
            ]
        }
    },
    {
        kind: 1971,
        name: "Problem Tracker",
        description: "Track problems/bugs",
        nip: "XX",
        template: {
            kind: 1971,
            content: "Problem description",
            tags: [
                ["title", "Problem title"],
                ["status", "open"]
            ]
        }
    },
    {
        kind: 1984,
        name: "Reporting",
        description: "Report user or content",
        nip: "56",
        template: {
            kind: 1984,
            content: "Reason for report",
            tags: [
                ["p", "<pubkey_to_report>", "spam"],
                ["e", "<event_to_report>", "spam"]
            ]
        }
    },
    {
        kind: 1985,
        name: "Label",
        description: "Label/tag content",
        nip: "32",
        template: {
            kind: 1985,
            content: "",
            tags: [
                ["L", "ugc"],
                ["l", "nsfw", "ugc"],
                ["e", "<event_to_label>"]
            ]
        }
    },
    {
        kind: 1986,
        name: "Relay Reviews",
        description: "Review a relay",
        nip: "XX",
        template: {
            kind: 1986,
            content: "Great relay! Fast and reliable.",
            tags: [
                ["r", "wss://relay.example.com"],
                ["rating", "5"]
            ]
        }
    },
    {
        kind: 1987,
        name: "AI Embeddings / Vector Lists",
        description: "AI vector embeddings for content",
        nip: "XX",
        template: {
            kind: 1987,
            content: "",
            tags: [
                ["e", "<event_with_embedding>"],
                ["embedding", "[0.1, 0.2, 0.3, ...]"]
            ]
        }
    },
    {
        kind: 2003,
        name: "Torrent",
        description: "Torrent metadata",
        nip: "35",
        template: {
            kind: 2003,
            content: "Torrent description",
            tags: [
                ["title", "Torrent Title"],
                ["x", "<info_hash>"],
                ["file", "filename.ext", "1048576"]
            ]
        }
    },
    {
        kind: 2004,
        name: "Torrent Comment",
        description: "Comment on a torrent",
        nip: "35",
        template: {
            kind: 2004,
            content: "Comment on the torrent",
            tags: [
                ["e", "<torrent_event_id>"]
            ]
        }
    },
    {
        kind: 2022,
        name: "Coinjoin Pool",
        description: "Coinjoin coordination",
        nip: "XX",
        template: {
            kind: 2022,
            content: "",
            tags: [
                ["amount", "100000"],
                ["expiry", "1704067200"]
            ]
        }
    },
    {
        kind: 4550,
        name: "Community Post Approval",
        description: "Approve a post for a community",
        nip: "72",
        template: {
            kind: 4550,
            content: "",
            tags: [
                ["a", "34550:<community_owner>:<community_d_tag>"],
                ["e", "<post_event_id>"],
                ["p", "<post_author_pubkey>"]
            ]
        }
    },
    // Job Request Range (5000-5999)
    {
        kind: 5000,
        name: "Job Request (Text Generation)",
        description: "Request AI text generation",
        nip: "90",
        template: {
            kind: 5000,
            content: "Generate a poem about nostr",
            tags: [
                ["i", "prompt text", "text"],
                ["output", "text/plain"]
            ]
        }
    },
    {
        kind: 5100,
        name: "Job Request (Text-to-Image)",
        description: "Request AI image generation",
        nip: "90",
        template: {
            kind: 5100,
            content: "",
            tags: [
                ["i", "A sunset over mountains", "text"],
                ["output", "image/png"]
            ]
        }
    },
    // Job Result Range (6000-6999)
    {
        kind: 6000,
        name: "Job Result (Text)",
        description: "Result of text generation job",
        nip: "90",
        template: {
            kind: 6000,
            content: "Generated text result",
            tags: [
                ["e", "<job_request_event_id>"],
                ["p", "<requester_pubkey>"],
                ["status", "success"]
            ]
        }
    },
    {
        kind: 6100,
        name: "Job Result (Image)",
        description: "Result of image generation job",
        nip: "90",
        template: {
            kind: 6100,
            content: "",
            tags: [
                ["e", "<job_request_event_id>"],
                ["p", "<requester_pubkey>"],
                ["url", "https://example.com/generated.png"],
                ["status", "success"]
            ]
        }
    },
    {
        kind: 7000,
        name: "Job Feedback",
        description: "Feedback on a job result",
        nip: "90",
        template: {
            kind: 7000,
            content: "Great work!",
            tags: [
                ["e", "<job_result_event_id>"],
                ["p", "<service_provider_pubkey>"],
                ["status", "success"],
                ["amount", "1000", "sat"]
            ]
        }
    },
    {
        kind: 7374,
        name: "Reserved Cashu Wallet Tokens",
        description: "Reserved Cashu tokens",
        nip: "60",
        template: {
            kind: 7374,
            content: "<encrypted_token_data>",
            tags: []
        }
    },
    {
        kind: 7375,
        name: "Cashu Wallet Tokens",
        description: "Cashu wallet token storage",
        nip: "60",
        template: {
            kind: 7375,
            content: "<encrypted_token_data>",
            tags: [
                ["mint", "https://mint.example.com"]
            ]
        }
    },
    {
        kind: 7376,
        name: "Cashu Wallet History",
        description: "Cashu wallet transaction history",
        nip: "60",
        template: {
            kind: 7376,
            content: "<encrypted_history>",
            tags: []
        }
    },
    {
        kind: 7516,
        name: "Geocache Log",
        description: "Log a geocache find",
        nip: "XX",
        template: {
            kind: 7516,
            content: "Found it! Great hide.",
            tags: [
                ["a", "37516:<cache_creator>:<cache_d_tag>"]
            ]
        }
    },
    {
        kind: 7517,
        name: "Geocache Proof of Find",
        description: "Proof of geocache discovery",
        nip: "XX",
        template: {
            kind: 7517,
            content: "",
            tags: [
                ["a", "37516:<cache_creator>:<cache_d_tag>"],
                ["proof", "<proof_hash>"]
            ]
        }
    },
    {
        kind: 8000,
        name: "Add User (NIP-43)",
        description: "Add a user to relay access",
        nip: "43",
        template: {
            kind: 8000,
            content: "",
            tags: [
                ["p", "<pubkey_to_add>"],
                ["role", "member"]
            ]
        }
    },
    {
        kind: 8001,
        name: "Remove User (NIP-43)",
        description: "Remove a user from relay access",
        nip: "43",
        template: {
            kind: 8001,
            content: "",
            tags: [
                ["p", "<pubkey_to_remove>"]
            ]
        }
    },
    // Group Control Events (9000-9030)
    {
        kind: 9000,
        name: "Group Add User",
        description: "Add user to a group",
        nip: "29",
        template: {
            kind: 9000,
            content: "",
            tags: [
                ["h", "<group_id>"],
                ["p", "<pubkey_to_add>"]
            ]
        }
    },
    {
        kind: 9001,
        name: "Group Remove User",
        description: "Remove user from a group",
        nip: "29",
        template: {
            kind: 9001,
            content: "",
            tags: [
                ["h", "<group_id>"],
                ["p", "<pubkey_to_remove>"]
            ]
        }
    },
    {
        kind: 9041,
        name: "Zap Goal",
        description: "Fundraising goal",
        nip: "75",
        template: {
            kind: 9041,
            content: "Help me reach my goal!",
            tags: [
                ["amount", "1000000"],
                ["relays", "wss://relay.example.com"],
                ["closed_at", "1704067200"]
            ]
        }
    },
    {
        kind: 9321,
        name: "Nutzap",
        description: "Cashu-based zap",
        nip: "61",
        template: {
            kind: 9321,
            content: "<encrypted_cashu_token>",
            tags: [
                ["p", "<recipient_pubkey>"],
                ["e", "<event_to_zap>"],
                ["mint", "https://mint.example.com"]
            ]
        }
    },
    {
        kind: 9467,
        name: "Tidal Login",
        description: "Tidal music service authentication",
        nip: "XX",
        template: {
            kind: 9467,
            content: "",
            tags: []
        }
    },
    {
        kind: 9734,
        name: "Zap Request",
        description: "Request a Lightning zap",
        nip: "57",
        template: {
            kind: 9734,
            content: "Zap comment (optional)",
            tags: [
                ["p", "<recipient_pubkey>"],
                ["e", "<event_to_zap>"],
                ["amount", "1000"],
                ["relays", "wss://relay.example.com"]
            ]
        }
    },
    {
        kind: 9735,
        name: "Zap",
        description: "Lightning zap receipt",
        nip: "57",
        template: {
            kind: 9735,
            content: "",
            tags: [
                ["p", "<recipient_pubkey>"],
                ["e", "<zapped_event_id>"],
                ["bolt11", "<lightning_invoice>"],
                ["description", "<zap_request_json>"],
                ["preimage", "<payment_preimage>"]
            ]
        }
    },
    {
        kind: 9802,
        name: "Highlights",
        description: "Highlight text from content",
        nip: "84",
        template: {
            kind: 9802,
            content: "The highlighted text portion",
            tags: [
                ["a", "30023:<author_pubkey>:<article_d_tag>"],
                ["context", "surrounding text for context"]
            ]
        }
    },
    // Replaceable Events (10000-19999)
    {
        kind: 10000,
        name: "Mute List",
        description: "List of muted users/events/words",
        nip: "51",
        isReplaceable: true,
        template: {
            kind: 10000,
            content: "",
            tags: [
                ["p", "<pubkey_to_mute>"],
                ["e", "<event_to_mute>"],
                ["word", "spamword"],
                ["t", "hashtag_to_mute"]
            ]
        }
    },
    {
        kind: 10001,
        name: "Pin List",
        description: "List of pinned events",
        nip: "51",
        isReplaceable: true,
        template: {
            kind: 10001,
            content: "",
            tags: [
                ["e", "<event_to_pin>"]
            ]
        }
    },
    {
        kind: 10002,
        name: "Relay List Metadata",
        description: "User's preferred relays",
        nip: "65",
        isReplaceable: true,
        template: {
            kind: 10002,
            content: "",
            tags: [
                ["r", "wss://relay1.example.com", "read"],
                ["r", "wss://relay2.example.com", "write"],
                ["r", "wss://relay3.example.com"]
            ]
        }
    },
    {
        kind: 10003,
        name: "Bookmark List",
        description: "Bookmarked events",
        nip: "51",
        isReplaceable: true,
        template: {
            kind: 10003,
            content: "",
            tags: [
                ["e", "<event_to_bookmark>"],
                ["a", "30023:<author>:<d_tag>"]
            ]
        }
    },
    {
        kind: 10004,
        name: "Communities List",
        description: "Communities the user belongs to",
        nip: "51",
        isReplaceable: true,
        template: {
            kind: 10004,
            content: "",
            tags: [
                ["a", "34550:<community_owner>:<community_d_tag>"]
            ]
        }
    },
    {
        kind: 10005,
        name: "Public Chats List",
        description: "Public chat channels user follows",
        nip: "51",
        isReplaceable: true,
        template: {
            kind: 10005,
            content: "",
            tags: [
                ["e", "<channel_create_event_id>"]
            ]
        }
    },
    {
        kind: 10006,
        name: "Blocked Relays List",
        description: "Relays the user has blocked",
        nip: "51",
        isReplaceable: true,
        template: {
            kind: 10006,
            content: "",
            tags: [
                ["r", "wss://blocked-relay.example.com"]
            ]
        }
    },
    {
        kind: 10007,
        name: "Search Relays List",
        description: "Preferred relays for search",
        nip: "51",
        isReplaceable: true,
        template: {
            kind: 10007,
            content: "",
            tags: [
                ["r", "wss://search-relay.example.com"]
            ]
        }
    },
    {
        kind: 10009,
        name: "User Groups",
        description: "Groups the user is part of",
        nip: "51",
        isReplaceable: true,
        template: {
            kind: 10009,
            content: "",
            tags: [
                ["h", "<group_id>", "wss://group-relay.example.com"]
            ]
        }
    },
    {
        kind: 10012,
        name: "Favorite Relays List",
        description: "User's favorite relays",
        nip: "51",
        isReplaceable: true,
        template: {
            kind: 10012,
            content: "",
            tags: [
                ["r", "wss://favorite-relay.example.com"]
            ]
        }
    },
    {
        kind: 10013,
        name: "Private Event Relay List",
        description: "Relays for private events",
        nip: "51",
        isReplaceable: true,
        template: {
            kind: 10013,
            content: "",
            tags: [
                ["r", "wss://private-relay.example.com"]
            ]
        }
    },
    {
        kind: 10015,
        name: "Interests List",
        description: "User's interests/topics",
        nip: "51",
        isReplaceable: true,
        template: {
            kind: 10015,
            content: "",
            tags: [
                ["t", "bitcoin"],
                ["t", "nostr"],
                ["t", "programming"]
            ]
        }
    },
    {
        kind: 10019,
        name: "Nutzap Mint Recommendation",
        description: "Recommended mints for Nutzaps",
        nip: "61",
        isReplaceable: true,
        template: {
            kind: 10019,
            content: "",
            tags: [
                ["mint", "https://mint.example.com"]
            ]
        }
    },
    {
        kind: 10020,
        name: "Media Follows",
        description: "Media creators the user follows",
        nip: "51",
        isReplaceable: true,
        template: {
            kind: 10020,
            content: "",
            tags: [
                ["p", "<media_creator_pubkey>"]
            ]
        }
    },
    {
        kind: 10030,
        name: "User Emoji List",
        description: "Custom emoji shortcuts",
        nip: "30",
        isReplaceable: true,
        template: {
            kind: 10030,
            content: "",
            tags: [
                ["emoji", "sats", "https://example.com/sats.png"],
                ["emoji", "nostr", "https://example.com/nostr.gif"]
            ]
        }
    },
    {
        kind: 10050,
        name: "Relay List to Receive DMs",
        description: "Relays where user receives DMs",
        nip: "17",
        isReplaceable: true,
        template: {
            kind: 10050,
            content: "",
            tags: [
                ["r", "wss://dm-relay.example.com"]
            ]
        }
    },
    {
        kind: 10051,
        name: "KeyPackage Relays List",
        description: "Relays for MLS KeyPackages",
        nip: "104",
        isReplaceable: true,
        template: {
            kind: 10051,
            content: "",
            tags: [
                ["r", "wss://mls-relay.example.com"]
            ]
        }
    },
    {
        kind: 10063,
        name: "User Server List",
        description: "User's file servers",
        nip: "96",
        isReplaceable: true,
        template: {
            kind: 10063,
            content: "",
            tags: [
                ["server", "https://fileserver.example.com"]
            ]
        }
    },
    {
        kind: 10096,
        name: "File Storage Server List",
        description: "Preferred file storage servers",
        nip: "96",
        isReplaceable: true,
        template: {
            kind: 10096,
            content: "",
            tags: [
                ["server", "https://storage.example.com"]
            ]
        }
    },
    {
        kind: 10166,
        name: "Relay Monitor Announcement",
        description: "Announce relay monitoring service",
        nip: "66",
        isReplaceable: true,
        template: {
            kind: 10166,
            content: "",
            tags: [
                ["frequency", "3600"],
                ["t", "<geohash>"]
            ]
        }
    },
    {
        kind: 10312,
        name: "Room Presence",
        description: "User presence in a room/space",
        nip: "XX",
        isReplaceable: true,
        template: {
            kind: 10312,
            content: "",
            tags: [
                ["d", "<room_id>"],
                ["status", "online"]
            ]
        }
    },
    {
        kind: 10377,
        name: "Proxy Announcement",
        description: "Announce proxy service",
        nip: "XX",
        isReplaceable: true,
        template: {
            kind: 10377,
            content: "",
            tags: []
        }
    },
    {
        kind: 11111,
        name: "Transport Method Announcement",
        description: "Announce transport method availability",
        nip: "XX",
        isReplaceable: true,
        template: {
            kind: 11111,
            content: "",
            tags: []
        }
    },
    {
        kind: 13194,
        name: "Wallet Info",
        description: "NWC wallet service info",
        nip: "47",
        isReplaceable: true,
        template: {
            kind: 13194,
            content: "",
            tags: [
                ["capabilities", "pay_invoice", "get_balance"],
                ["name", "My Wallet"]
            ]
        }
    },
    {
        kind: 13534,
        name: "Membership Lists",
        description: "List of relay members",
        nip: "43",
        isReplaceable: true,
        template: {
            kind: 13534,
            content: "",
            tags: [
                ["p", "<member_pubkey>", "member"],
                ["p", "<admin_pubkey>", "admin"]
            ]
        }
    },
    {
        kind: 17375,
        name: "Cashu Wallet Event",
        description: "Cashu wallet state event",
        nip: "60",
        isReplaceable: true,
        template: {
            kind: 17375,
            content: "<encrypted_wallet_data>",
            tags: []
        }
    },
    {
        kind: 21000,
        name: "Lightning Pub RPC",
        description: "Lightning.pub RPC call",
        nip: "XX",
        template: {
            kind: 21000,
            content: "<rpc_payload>",
            tags: []
        }
    },
    {
        kind: 22242,
        name: "Client Authentication",
        description: "NIP-42 client auth event",
        nip: "42",
        template: {
            kind: 22242,
            content: "",
            tags: [
                ["relay", "wss://relay.example.com"],
                ["challenge", "<challenge_string>"]
            ]
        }
    },
    {
        kind: 23194,
        name: "Wallet Request",
        description: "NWC wallet request",
        nip: "47",
        template: {
            kind: 23194,
            content: "<encrypted_request>",
            tags: [
                ["p", "<wallet_service_pubkey>"]
            ]
        }
    },
    {
        kind: 23195,
        name: "Wallet Response",
        description: "NWC wallet response",
        nip: "47",
        template: {
            kind: 23195,
            content: "<encrypted_response>",
            tags: [
                ["p", "<requester_pubkey>"],
                ["e", "<request_event_id>"]
            ]
        }
    },
    {
        kind: 24133,
        name: "Nostr Connect",
        description: "NIP-46 remote signing request/response",
        nip: "46",
        template: {
            kind: 24133,
            content: "<encrypted_payload>",
            tags: [
                ["p", "<target_pubkey>"]
            ]
        }
    },
    {
        kind: 24242,
        name: "Blobs Stored on Mediaservers",
        description: "Reference to blob storage",
        nip: "XX",
        template: {
            kind: 24242,
            content: "",
            tags: [
                ["x", "<sha256_hash>"],
                ["url", "https://cdn.example.com/blob"],
                ["m", "application/octet-stream"]
            ]
        }
    },
    {
        kind: 27235,
        name: "HTTP Auth",
        description: "NIP-98 HTTP authentication",
        nip: "98",
        template: {
            kind: 27235,
            content: "",
            tags: [
                ["u", "https://api.example.com/endpoint"],
                ["method", "POST"]
            ]
        }
    },
    {
        kind: 28934,
        name: "Join Request",
        description: "Request to join a group/community",
        nip: "XX",
        template: {
            kind: 28934,
            content: "Please let me join!",
            tags: [
                ["h", "<group_id>"]
            ]
        }
    },
    {
        kind: 28935,
        name: "Invite Request",
        description: "Invite someone to group/community",
        nip: "XX",
        template: {
            kind: 28935,
            content: "",
            tags: [
                ["h", "<group_id>"],
                ["p", "<invitee_pubkey>"]
            ]
        }
    },
    {
        kind: 28936,
        name: "Leave Request",
        description: "Request to leave a group",
        nip: "XX",
        template: {
            kind: 28936,
            content: "",
            tags: [
                ["h", "<group_id>"]
            ]
        }
    },
    // Addressable Events (30000-39999)
    {
        kind: 30000,
        name: "Follow Sets",
        description: "Named list of followed pubkeys",
        nip: "51",
        isAddressable: true,
        template: {
            kind: 30000,
            content: "",
            tags: [
                ["d", "my-follow-list"],
                ["p", "<pubkey_to_include>"]
            ]
        }
    },
    {
        kind: 30001,
        name: "Generic Lists",
        description: "Generic named list",
        nip: "51",
        isAddressable: true,
        template: {
            kind: 30001,
            content: "",
            tags: [
                ["d", "my-list"],
                ["e", "<event_id>"],
                ["p", "<pubkey>"],
                ["t", "tag"]
            ]
        }
    },
    {
        kind: 30002,
        name: "Relay Sets",
        description: "Named list of relays",
        nip: "51",
        isAddressable: true,
        template: {
            kind: 30002,
            content: "",
            tags: [
                ["d", "my-relays"],
                ["r", "wss://relay.example.com"]
            ]
        }
    },
    {
        kind: 30003,
        name: "Bookmark Sets",
        description: "Named bookmark collection",
        nip: "51",
        isAddressable: true,
        template: {
            kind: 30003,
            content: "",
            tags: [
                ["d", "favorites"],
                ["e", "<bookmarked_event_id>"]
            ]
        }
    },
    {
        kind: 30004,
        name: "Curation Sets",
        description: "Curated content collection",
        nip: "51",
        isAddressable: true,
        template: {
            kind: 30004,
            content: "",
            tags: [
                ["d", "best-of-2024"],
                ["a", "30023:<author>:<article_d_tag>"]
            ]
        }
    },
    {
        kind: 30005,
        name: "Video Sets",
        description: "Video playlist",
        nip: "51",
        isAddressable: true,
        template: {
            kind: 30005,
            content: "",
            tags: [
                ["d", "my-playlist"],
                ["a", "34235:<creator>:<video_d_tag>"]
            ]
        }
    },
    {
        kind: 30007,
        name: "Kind Mute Sets",
        description: "Muted event kinds",
        nip: "51",
        isAddressable: true,
        template: {
            kind: 30007,
            content: "",
            tags: [
                ["d", "muted-kinds"],
                ["k", "1"],
                ["k", "7"]
            ]
        }
    },
    {
        kind: 30008,
        name: "Profile Badges",
        description: "Badges displayed on profile",
        nip: "58",
        isAddressable: true,
        template: {
            kind: 30008,
            content: "",
            tags: [
                ["d", "profile_badges"],
                ["a", "30009:<badge_creator>:<badge_d_tag>"],
                ["e", "<badge_award_event_id>"]
            ]
        }
    },
    {
        kind: 30009,
        name: "Badge Definition",
        description: "Define a badge",
        nip: "58",
        isAddressable: true,
        template: {
            kind: 30009,
            content: "",
            tags: [
                ["d", "my-badge"],
                ["name", "Badge Name"],
                ["description", "Badge description"],
                ["image", "https://example.com/badge.png"],
                ["thumb", "https://example.com/badge-thumb.png"]
            ]
        }
    },
    {
        kind: 30015,
        name: "Interest Sets",
        description: "Named interest collection",
        nip: "51",
        isAddressable: true,
        template: {
            kind: 30015,
            content: "",
            tags: [
                ["d", "tech-interests"],
                ["t", "bitcoin"],
                ["t", "lightning"]
            ]
        }
    },
    {
        kind: 30017,
        name: "Create or Update a Stall",
        description: "Marketplace stall definition",
        nip: "15",
        isAddressable: true,
        template: {
            kind: 30017,
            content: JSON.stringify({
                id: "stall-id",
                name: "My Stall",
                description: "Selling cool stuff",
                currency: "sat",
                shipping: [
                    { id: "shipping-1", name: "Standard", cost: 1000, regions: ["worldwide"] }
                ]
            }, null, 2),
            tags: [
                ["d", "my-stall"]
            ]
        }
    },
    {
        kind: 30018,
        name: "Create or Update a Product",
        description: "Marketplace product listing",
        nip: "15",
        isAddressable: true,
        template: {
            kind: 30018,
            content: JSON.stringify({
                id: "product-id",
                stall_id: "stall-id",
                name: "Product Name",
                description: "Product description",
                images: ["https://example.com/product.jpg"],
                currency: "sat",
                price: 10000,
                quantity: 10
            }, null, 2),
            tags: [
                ["d", "my-product"],
                ["t", "electronics"]
            ]
        }
    },
    {
        kind: 30019,
        name: "Marketplace UI/UX",
        description: "Marketplace display preferences",
        nip: "15",
        isAddressable: true,
        template: {
            kind: 30019,
            content: JSON.stringify({
                theme: "dark",
                layout: "grid"
            }, null, 2),
            tags: [
                ["d", "marketplace-settings"]
            ]
        }
    },
    {
        kind: 30020,
        name: "Product Sold as Auction",
        description: "Auction listing",
        nip: "15",
        isAddressable: true,
        template: {
            kind: 30020,
            content: JSON.stringify({
                id: "auction-id",
                stall_id: "stall-id",
                name: "Rare Item",
                description: "Auction description",
                images: ["https://example.com/item.jpg"],
                starting_bid: 1000,
                currency: "sat"
            }, null, 2),
            tags: [
                ["d", "my-auction"],
                ["start", "1704067200"],
                ["end", "1704153600"]
            ]
        }
    },
    {
        kind: 30023,
        name: "Long-form Content",
        description: "Blog post / article",
        nip: "23",
        isAddressable: true,
        template: {
            kind: 30023,
            content: "# Article Title\n\nYour long-form content in Markdown format...\n\n## Section 1\n\nContent here...",
            tags: [
                ["d", "article-identifier"],
                ["title", "Article Title"],
                ["summary", "A brief summary of the article"],
                ["image", "https://example.com/cover.jpg"],
                ["published_at", "1704067200"],
                ["t", "nostr"],
                ["t", "bitcoin"]
            ]
        }
    },
    {
        kind: 30024,
        name: "Draft Long-form Content",
        description: "Draft blog post / article",
        nip: "23",
        isAddressable: true,
        template: {
            kind: 30024,
            content: "# Draft Article\n\nWork in progress...",
            tags: [
                ["d", "draft-identifier"],
                ["title", "Draft Title"]
            ]
        }
    },
    {
        kind: 30030,
        name: "Emoji Sets",
        description: "Named emoji collection",
        nip: "30",
        isAddressable: true,
        template: {
            kind: 30030,
            content: "",
            tags: [
                ["d", "my-emojis"],
                ["emoji", "pepe", "https://example.com/pepe.png"],
                ["emoji", "wojak", "https://example.com/wojak.gif"]
            ]
        }
    },
    {
        kind: 30040,
        name: "Curated Publication Index",
        description: "Index for curated publication",
        nip: "XX",
        isAddressable: true,
        template: {
            kind: 30040,
            content: "",
            tags: [
                ["d", "publication-index"],
                ["title", "My Publication"],
                ["e", "<article_event_id>"]
            ]
        }
    },
    {
        kind: 30041,
        name: "Curated Publication Content",
        description: "Content for curated publication",
        nip: "XX",
        isAddressable: true,
        template: {
            kind: 30041,
            content: "Publication content",
            tags: [
                ["d", "publication-content"],
                ["a", "30040:<owner>:<index_d_tag>"]
            ]
        }
    },
    {
        kind: 30063,
        name: "Release Artifact Sets",
        description: "Software release artifacts",
        nip: "XX",
        isAddressable: true,
        template: {
            kind: 30063,
            content: "",
            tags: [
                ["d", "v1.0.0"],
                ["name", "Release v1.0.0"],
                ["url", "https://github.com/user/repo/releases/v1.0.0"],
                ["x", "<sha256_hash>"]
            ]
        }
    },
    {
        kind: 30078,
        name: "Application-specific Data",
        description: "App-specific user data",
        nip: "78",
        isAddressable: true,
        template: {
            kind: 30078,
            content: JSON.stringify({ setting1: "value1", setting2: true }, null, 2),
            tags: [
                ["d", "my-app-settings"]
            ]
        }
    },
    {
        kind: 30166,
        name: "Relay Discovery",
        description: "Relay discovery information",
        nip: "66",
        isAddressable: true,
        template: {
            kind: 30166,
            content: "",
            tags: [
                ["d", "<relay_pubkey>"],
                ["n", "clearnet"],
                ["N", "50"],
                ["R", "read"],
                ["R", "write"]
            ]
        }
    },
    {
        kind: 30267,
        name: "App Curation Sets",
        description: "Curated app collections",
        nip: "XX",
        isAddressable: true,
        template: {
            kind: 30267,
            content: "",
            tags: [
                ["d", "nostr-apps"],
                ["a", "32267:<app_creator>:<app_d_tag>"]
            ]
        }
    },
    {
        kind: 30311,
        name: "Live Event",
        description: "Live streaming event",
        nip: "53",
        isAddressable: true,
        template: {
            kind: 30311,
            content: "",
            tags: [
                ["d", "stream-id"],
                ["title", "My Live Stream"],
                ["summary", "Stream description"],
                ["image", "https://example.com/thumbnail.jpg"],
                ["streaming", "https://stream.example.com/live.m3u8"],
                ["status", "live"],
                ["starts", "1704067200"],
                ["p", "<host_pubkey>", "host"]
            ]
        }
    },
    {
        kind: 30312,
        name: "Interactive Room",
        description: "Interactive audio/video room",
        nip: "XX",
        isAddressable: true,
        template: {
            kind: 30312,
            content: "",
            tags: [
                ["d", "room-id"],
                ["name", "Discussion Room"],
                ["about", "Room description"]
            ]
        }
    },
    {
        kind: 30313,
        name: "Conference Event",
        description: "Conference/meetup event",
        nip: "XX",
        isAddressable: true,
        template: {
            kind: 30313,
            content: "",
            tags: [
                ["d", "conference-id"],
                ["name", "Bitcoin Conference 2024"],
                ["about", "Annual Bitcoin conference"],
                ["location", "Miami, FL"],
                ["start", "1704067200"],
                ["end", "1704326400"]
            ]
        }
    },
    {
        kind: 30315,
        name: "User Statuses",
        description: "User status updates",
        nip: "38",
        isAddressable: true,
        template: {
            kind: 30315,
            content: "Building on Nostr",
            tags: [
                ["d", "general"],
                ["expiration", "1704153600"]
            ]
        }
    },
    {
        kind: 30388,
        name: "Slide Set",
        description: "Presentation slides",
        nip: "XX",
        isAddressable: true,
        template: {
            kind: 30388,
            content: "",
            tags: [
                ["d", "presentation-id"],
                ["title", "My Presentation"],
                ["slide", "1", "# Slide 1"],
                ["slide", "2", "## Slide 2"]
            ]
        }
    },
    {
        kind: 30402,
        name: "Classified Listing",
        description: "Classified ad",
        nip: "99",
        isAddressable: true,
        template: {
            kind: 30402,
            content: "Full description of the item or service",
            tags: [
                ["d", "listing-id"],
                ["title", "Item for Sale"],
                ["price", "100000", "sat"],
                ["location", "New York, NY"],
                ["image", "https://example.com/item.jpg"],
                ["t", "electronics"]
            ]
        }
    },
    {
        kind: 30403,
        name: "Draft Classified Listing",
        description: "Draft classified ad",
        nip: "99",
        isAddressable: true,
        template: {
            kind: 30403,
            content: "Draft description",
            tags: [
                ["d", "draft-listing-id"],
                ["title", "Draft Listing"]
            ]
        }
    },
    {
        kind: 30617,
        name: "Repository Announcements",
        description: "Git repository announcement",
        nip: "34",
        isAddressable: true,
        template: {
            kind: 30617,
            content: "Repository description",
            tags: [
                ["d", "repo-identifier"],
                ["name", "my-project"],
                ["description", "Project description"],
                ["clone", "https://github.com/user/repo.git"],
                ["web", "https://github.com/user/repo"],
                ["relays", "wss://relay.example.com"]
            ]
        }
    },
    {
        kind: 30618,
        name: "Repository State Announcements",
        description: "Git repository state/refs",
        nip: "34",
        isAddressable: true,
        template: {
            kind: 30618,
            content: "",
            tags: [
                ["d", "repo-identifier"],
                ["r", "refs/heads/main", "<commit_hash>"],
                ["r", "refs/tags/v1.0.0", "<tag_hash>"]
            ]
        }
    },
    {
        kind: 30818,
        name: "Wiki Article",
        description: "Wiki/knowledge base article",
        nip: "54",
        isAddressable: true,
        template: {
            kind: 30818,
            content: "# Wiki Article\n\nArticle content in markdown...",
            tags: [
                ["d", "article-slug"],
                ["title", "Article Title"],
                ["summary", "Brief summary"]
            ]
        }
    },
    {
        kind: 30819,
        name: "Redirects",
        description: "Wiki redirect",
        nip: "54",
        isAddressable: true,
        template: {
            kind: 30819,
            content: "",
            tags: [
                ["d", "old-slug"],
                ["a", "30818:<author>:<new-slug>"]
            ]
        }
    },
    {
        kind: 31234,
        name: "Draft Event",
        description: "Generic draft event",
        nip: "XX",
        isAddressable: true,
        template: {
            kind: 31234,
            content: "Draft content",
            tags: [
                ["d", "draft-id"],
                ["k", "<target_kind>"]
            ]
        }
    },
    {
        kind: 31388,
        name: "Link Set",
        description: "Collection of links",
        nip: "XX",
        isAddressable: true,
        template: {
            kind: 31388,
            content: "",
            tags: [
                ["d", "my-links"],
                ["r", "https://example.com", "Example Site"],
                ["r", "https://nostr.com", "Nostr"]
            ]
        }
    },
    {
        kind: 31890,
        name: "Feed",
        description: "Custom feed definition",
        nip: "XX",
        isAddressable: true,
        template: {
            kind: 31890,
            content: JSON.stringify({
                name: "My Custom Feed",
                description: "A feed of my favorite content"
            }, null, 2),
            tags: [
                ["d", "my-feed"],
                ["p", "<included_pubkey>"],
                ["t", "bitcoin"]
            ]
        }
    },
    {
        kind: 31922,
        name: "Date-Based Calendar Event",
        description: "All-day calendar event",
        nip: "52",
        isAddressable: true,
        template: {
            kind: 31922,
            content: "Event description",
            tags: [
                ["d", "event-id"],
                ["title", "Event Title"],
                ["start", "2024-01-01"],
                ["end", "2024-01-02"],
                ["location", "Conference Center"]
            ]
        }
    },
    {
        kind: 31923,
        name: "Time-Based Calendar Event",
        description: "Timed calendar event",
        nip: "52",
        isAddressable: true,
        template: {
            kind: 31923,
            content: "Meeting agenda",
            tags: [
                ["d", "meeting-id"],
                ["title", "Team Meeting"],
                ["start", "1704067200"],
                ["end", "1704070800"],
                ["start_tzid", "America/New_York"],
                ["location", "Zoom"],
                ["p", "<participant_pubkey>"]
            ]
        }
    },
    {
        kind: 31924,
        name: "Calendar",
        description: "Calendar definition",
        nip: "52",
        isAddressable: true,
        template: {
            kind: 31924,
            content: "",
            tags: [
                ["d", "my-calendar"],
                ["title", "Personal Calendar"],
                ["a", "31923:<owner>:<event_d_tag>"]
            ]
        }
    },
    {
        kind: 31925,
        name: "Calendar Event RSVP",
        description: "RSVP to a calendar event",
        nip: "52",
        isAddressable: true,
        template: {
            kind: 31925,
            content: "Looking forward to it!",
            tags: [
                ["d", "<event_d_tag>"],
                ["a", "31923:<organizer>:<event_d_tag>"],
                ["status", "accepted"]
            ]
        }
    },
    {
        kind: 31989,
        name: "Handler Recommendation",
        description: "Recommend an app for event kind",
        nip: "89",
        isAddressable: true,
        template: {
            kind: 31989,
            content: "",
            tags: [
                ["d", "1"],
                ["a", "31990:<handler_author>:<handler_d_tag>", "web", "https://app.example.com"]
            ]
        }
    },
    {
        kind: 31990,
        name: "Handler Information",
        description: "App handler definition",
        nip: "89",
        isAddressable: true,
        template: {
            kind: 31990,
            content: "",
            tags: [
                ["d", "my-app"],
                ["name", "My Nostr App"],
                ["about", "App description"],
                ["picture", "https://example.com/app-icon.png"],
                ["web", "https://app.example.com"],
                ["k", "1"],
                ["k", "30023"]
            ]
        }
    },
    {
        kind: 32267,
        name: "Software Application",
        description: "Software application definition",
        nip: "XX",
        isAddressable: true,
        template: {
            kind: 32267,
            content: "Application description",
            tags: [
                ["d", "app-identifier"],
                ["name", "App Name"],
                ["version", "1.0.0"],
                ["url", "https://app.example.com"],
                ["icon", "https://example.com/icon.png"]
            ]
        }
    },
    {
        kind: 34550,
        name: "Community Definition",
        description: "Define a community",
        nip: "72",
        isAddressable: true,
        template: {
            kind: 34550,
            content: "",
            tags: [
                ["d", "community-name"],
                ["name", "My Community"],
                ["description", "Community description"],
                ["image", "https://example.com/community.jpg"],
                ["p", "<moderator_pubkey>", "moderator"],
                ["relay", "wss://community-relay.example.com"]
            ]
        }
    },
    {
        kind: 37516,
        name: "Geocache Listing",
        description: "Geocache location listing",
        nip: "XX",
        isAddressable: true,
        template: {
            kind: 37516,
            content: "Geocache description and hints",
            tags: [
                ["d", "cache-id"],
                ["name", "Secret Spot"],
                ["g", "u4pruydqqvj"],
                ["difficulty", "3"],
                ["terrain", "2"]
            ]
        }
    },
    {
        kind: 38172,
        name: "Cashu Mint Announcement",
        description: "Announce a Cashu mint",
        nip: "XX",
        isAddressable: true,
        template: {
            kind: 38172,
            content: "",
            tags: [
                ["d", "<mint_pubkey>"],
                ["mint", "https://mint.example.com"],
                ["name", "My Cashu Mint"]
            ]
        }
    },
    {
        kind: 38173,
        name: "Fedimint Announcement",
        description: "Announce a Fedimint federation",
        nip: "XX",
        isAddressable: true,
        template: {
            kind: 38173,
            content: "",
            tags: [
                ["d", "<federation_id>"],
                ["name", "My Federation"],
                ["invite", "<invite_code>"]
            ]
        }
    },
    {
        kind: 38383,
        name: "Peer-to-peer Order Events",
        description: "P2P trading order",
        nip: "XX",
        isAddressable: true,
        template: {
            kind: 38383,
            content: "",
            tags: [
                ["d", "order-id"],
                ["type", "buy"],
                ["amount", "100000"],
                ["currency", "USD"],
                ["price", "50000"]
            ]
        }
    },
    // Group Metadata (39000-39009)
    {
        kind: 39000,
        name: "Group Metadata",
        description: "Group/channel metadata",
        nip: "29",
        isAddressable: true,
        template: {
            kind: 39000,
            content: "",
            tags: [
                ["d", "<group_id>"],
                ["name", "Group Name"],
                ["about", "Group description"],
                ["picture", "https://example.com/group.jpg"]
            ]
        }
    },
    {
        kind: 39001,
        name: "Group Admins",
        description: "List of group admins",
        nip: "29",
        isAddressable: true,
        template: {
            kind: 39001,
            content: "",
            tags: [
                ["d", "<group_id>"],
                ["p", "<admin_pubkey>", "admin"]
            ]
        }
    },
    {
        kind: 39002,
        name: "Group Members",
        description: "List of group members",
        nip: "29",
        isAddressable: true,
        template: {
            kind: 39002,
            content: "",
            tags: [
                ["d", "<group_id>"],
                ["p", "<member_pubkey>"]
            ]
        }
    },
    {
        kind: 39089,
        name: "Starter Packs",
        description: "Starter pack of accounts to follow",
        nip: "XX",
        isAddressable: true,
        template: {
            kind: 39089,
            content: "",
            tags: [
                ["d", "pack-id"],
                ["name", "Bitcoin Developers"],
                ["description", "Must-follow Bitcoin developers"],
                ["p", "<developer_pubkey>"]
            ]
        }
    },
    {
        kind: 39092,
        name: "Media Starter Packs",
        description: "Starter pack of media creators",
        nip: "XX",
        isAddressable: true,
        template: {
            kind: 39092,
            content: "",
            tags: [
                ["d", "media-pack-id"],
                ["name", "Photography Creators"],
                ["p", "<creator_pubkey>"]
            ]
        }
    },
    {
        kind: 39701,
        name: "Web Bookmarks",
        description: "Bookmarked web pages",
        nip: "XX",
        isAddressable: true,
        template: {
            kind: 39701,
            content: "",
            tags: [
                ["d", "bookmarks"],
                ["r", "https://example.com", "Example Site"],
                ["t", "reference"]
            ]
        }
    }
];

// Helper function to get event kind by number
export function getEventKind(kindNumber) {
    return eventKinds.find(k => k.kind === kindNumber);
}

// Helper function to search event kinds by name or description
export function searchEventKinds(query) {
    const lowerQuery = query.toLowerCase();
    return eventKinds.filter(k =>
        k.name.toLowerCase().includes(lowerQuery) ||
        k.description.toLowerCase().includes(lowerQuery) ||
        k.kind.toString().includes(query)
    );
}

// Helper function to get all event kinds grouped by category
export function getEventKindsByCategory() {
    return {
        regular: eventKinds.filter(k => k.kind < 10000 && !k.isReplaceable),
        replaceable: eventKinds.filter(k => k.isReplaceable),
        ephemeral: eventKinds.filter(k => k.kind >= 20000 && k.kind < 30000),
        addressable: eventKinds.filter(k => k.isAddressable)
    };
}

// Helper function to create a template event with current timestamp
export function createTemplateEvent(kindNumber, userPubkey = null) {
    const kindInfo = getEventKind(kindNumber);
    if (!kindInfo) {
        return {
            kind: kindNumber,
            content: "",
            tags: [],
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

// Export kind categories for filtering in UI
export const kindCategories = [
    { id: "all", name: "All Kinds", filter: () => true },
    { id: "regular", name: "Regular Events (0-9999)", filter: k => k.kind < 10000 && !k.isReplaceable },
    { id: "replaceable", name: "Replaceable (10000-19999)", filter: k => k.isReplaceable },
    { id: "ephemeral", name: "Ephemeral (20000-29999)", filter: k => k.kind >= 20000 && k.kind < 30000 },
    { id: "addressable", name: "Addressable (30000-39999)", filter: k => k.isAddressable },
    { id: "social", name: "Social", filter: k => [0, 1, 3, 6, 7].includes(k.kind) },
    { id: "messaging", name: "Messaging", filter: k => [4, 9, 10, 11, 12, 14, 15, 40, 41, 42].includes(k.kind) },
    { id: "lists", name: "Lists", filter: k => k.name.toLowerCase().includes("list") || k.name.toLowerCase().includes("set") },
    { id: "marketplace", name: "Marketplace", filter: k => [30017, 30018, 30019, 30020, 1021, 1022, 30402, 30403].includes(k.kind) },
    { id: "lightning", name: "Lightning/Zaps", filter: k => [9734, 9735, 9041, 9321, 7374, 7375, 7376].includes(k.kind) },
    { id: "media", name: "Media", filter: k => [20, 21, 22, 1063, 1222, 1244].includes(k.kind) },
    { id: "git", name: "Git/Code", filter: k => [818, 1337, 1617, 1618, 1619, 1621, 1622, 30617, 30618].includes(k.kind) },
    { id: "calendar", name: "Calendar", filter: k => [31922, 31923, 31924, 31925].includes(k.kind) },
    { id: "groups", name: "Groups", filter: k => k.kind >= 9000 && k.kind <= 9030 || k.kind >= 39000 && k.kind <= 39009 },
];
