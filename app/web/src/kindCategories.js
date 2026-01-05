/**
 * Kind categories for curating mode.
 * These define predefined groups of event kinds that can be enabled/disabled together.
 * The categories match the server-side definitions in pkg/database/curating-acl.go.
 */

export const curationKindCategories = [
  {
    id: "social",
    name: "Social/Notes",
    description: "User profiles, notes, follows, reposts, reactions, and relay lists",
    kinds: [0, 1, 3, 6, 7, 10002],
  },
  {
    id: "dm",
    name: "Direct Messages",
    description: "Encrypted direct messages (legacy and NIP-17 gift-wrapped)",
    kinds: [4, 14, 1059],
  },
  {
    id: "longform",
    name: "Long-form Content",
    description: "Blog posts and article drafts",
    kinds: [30023, 30024],
  },
  {
    id: "media",
    name: "Media",
    description: "File metadata and media attachments",
    kinds: [1063, 20, 21, 22],
  },
  {
    id: "marketplace",
    name: "Marketplace",
    description: "Product listings, stalls, and marketplace events",
    kinds: [30017, 30018, 30019, 30020],
  },
  {
    id: "groups_nip29",
    name: "Group Messaging (NIP-29)",
    description: "Simple relay-based group chat messages",
    kinds: [9, 10, 11, 12],
  },
  {
    id: "groups_nip72",
    name: "Communities (NIP-72)",
    description: "Community definitions and threaded discussions",
    kinds: [34550, 1111, 4550],
  },
  {
    id: "lists",
    name: "Lists/Bookmarks",
    description: "Mute lists, pin lists, and parameterized list events",
    kinds: [10000, 10001, 30000, 30001],
  },
];

/**
 * Get all kinds from selected categories.
 * @param {string[]} categoryIds - Array of category IDs
 * @returns {number[]} - Array of unique kind numbers
 */
export function getKindsFromCategories(categoryIds) {
  const kinds = new Set();
  for (const id of categoryIds) {
    const category = curationKindCategories.find((c) => c.id === id);
    if (category) {
      category.kinds.forEach((k) => kinds.add(k));
    }
  }
  return Array.from(kinds).sort((a, b) => a - b);
}

/**
 * Get category IDs that contain a given kind.
 * @param {number} kind - The kind number to look up
 * @returns {string[]} - Array of category IDs containing this kind
 */
export function getCategoriesForKind(kind) {
  return curationKindCategories
    .filter((c) => c.kinds.includes(kind))
    .map((c) => c.id);
}

/**
 * Parse a custom kinds string (e.g., "100, 200-300, 500") into an array of kinds.
 * @param {string} customKinds - Comma-separated list of kinds and ranges
 * @returns {number[]} - Array of individual kind numbers
 */
export function parseCustomKinds(customKinds) {
  if (!customKinds || !customKinds.trim()) return [];

  const kinds = new Set();
  const parts = customKinds.split(",").map((p) => p.trim());

  for (const part of parts) {
    if (!part) continue;

    // Check if it's a range (e.g., "100-200")
    if (part.includes("-")) {
      const [start, end] = part.split("-").map((n) => parseInt(n.trim(), 10));
      if (!isNaN(start) && !isNaN(end) && start <= end) {
        // Don't expand ranges > 1000 to avoid memory issues
        if (end - start <= 1000) {
          for (let i = start; i <= end; i++) {
            kinds.add(i);
          }
        }
      }
    } else {
      const num = parseInt(part, 10);
      if (!isNaN(num)) {
        kinds.add(num);
      }
    }
  }

  return Array.from(kinds).sort((a, b) => a - b);
}

/**
 * Format a list of kinds into a compact string with ranges.
 * @param {number[]} kinds - Array of kind numbers
 * @returns {string} - Formatted string like "1, 3, 5-10, 15"
 */
export function formatKindsCompact(kinds) {
  if (!kinds || kinds.length === 0) return "";

  const sorted = [...kinds].sort((a, b) => a - b);
  const ranges = [];
  let rangeStart = sorted[0];
  let rangeEnd = sorted[0];

  for (let i = 1; i < sorted.length; i++) {
    if (sorted[i] === rangeEnd + 1) {
      rangeEnd = sorted[i];
    } else {
      if (rangeEnd > rangeStart + 1) {
        ranges.push(`${rangeStart}-${rangeEnd}`);
      } else if (rangeEnd === rangeStart + 1) {
        ranges.push(`${rangeStart}, ${rangeEnd}`);
      } else {
        ranges.push(`${rangeStart}`);
      }
      rangeStart = sorted[i];
      rangeEnd = sorted[i];
    }
  }

  // Push the last range
  if (rangeEnd > rangeStart + 1) {
    ranges.push(`${rangeStart}-${rangeEnd}`);
  } else if (rangeEnd === rangeStart + 1) {
    ranges.push(`${rangeStart}, ${rangeEnd}`);
  } else {
    ranges.push(`${rangeStart}`);
  }

  return ranges.join(", ");
}
