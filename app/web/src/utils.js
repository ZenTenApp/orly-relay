import { kindNames } from './constants.js';

/**
 * Get human-readable name for a Nostr event kind
 * @param {number} kind - The event kind number
 * @returns {string} Human readable kind name
 */
export function getKindName(kind) {
    return kindNames[kind] || `Kind ${kind}`;
}

/**
 * Truncate a pubkey for display
 * @param {string} pubkey - The full pubkey hex string
 * @returns {string} Truncated pubkey
 */
export function truncatePubkey(pubkey) {
    if (!pubkey) return "unknown";
    return pubkey.slice(0, 8) + "..." + pubkey.slice(-8);
}

/**
 * Truncate content for preview display
 * @param {string} content - The content to truncate
 * @param {number} maxLength - Maximum length before truncation
 * @returns {string} Truncated content
 */
export function truncateContent(content, maxLength = 100) {
    if (!content) return "";
    return content.length > maxLength
        ? content.slice(0, maxLength) + "..."
        : content;
}

/**
 * Format a Unix timestamp for display
 * @param {number} timestamp - Unix timestamp in seconds
 * @returns {string} Formatted date/time string
 */
export function formatTimestamp(timestamp) {
    if (!timestamp) return "";
    return new Date(timestamp * 1000).toLocaleString();
}

/**
 * Escape HTML special characters to prevent XSS
 * @param {string} str - String to escape
 * @returns {string} Escaped string
 */
export function escapeHtml(str) {
    if (!str) return "";
    return str
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#039;");
}

/**
 * Convert "about" text to safe HTML with line breaks
 * @param {string} text - The about text
 * @returns {string} HTML with line breaks
 */
export function aboutToHtml(text) {
    if (!text) return "";
    return escapeHtml(text).replace(/\n\n/g, "<br>");
}

/**
 * Copy text to clipboard with fallback for older browsers
 * @param {string} text - Text to copy
 * @returns {Promise<boolean>} Whether copy succeeded
 */
export async function copyToClipboard(text) {
    try {
        await navigator.clipboard.writeText(text);
        return true;
    } catch (error) {
        console.error("Failed to copy to clipboard:", error);
        // Fallback for older browsers
        try {
            const textArea = document.createElement("textarea");
            textArea.value = text;
            document.body.appendChild(textArea);
            textArea.select();
            document.execCommand("copy");
            document.body.removeChild(textArea);
            return true;
        } catch (fallbackError) {
            console.error("Fallback copy also failed:", fallbackError);
            return false;
        }
    }
}

/**
 * Show copy feedback on a button element
 * @param {HTMLElement} button - The button element
 * @param {boolean} success - Whether copy succeeded
 */
export function showCopyFeedback(button, success = true) {
    if (!button) return;
    const originalText = button.textContent;
    const originalBg = button.style.backgroundColor;

    if (success) {
        button.textContent = "";
        button.style.backgroundColor = "#4CAF50";
    } else {
        button.textContent = "L";
        button.style.backgroundColor = "#f44336";
    }

    setTimeout(() => {
        button.textContent = originalText;
        button.style.backgroundColor = originalBg;
    }, 2000);
}
