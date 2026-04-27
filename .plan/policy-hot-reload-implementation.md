# Implementation Plan: Policy Hot Reload, Follow List Whitelisting, and Web UI

**Issue:** https://git.nostrdev.com/mleku/git.smesh.lol/orly/issues/6

## Overview

This plan implements three interconnected features for ORLY's policy system:
1. **Dynamic Policy Configuration** via kind 12345 events (hot reload)
2. **Administrator Follow List Whitelisting** within the policy system
3. **Web Interface** for policy management with JSON editing

## Architecture Summary

### Current System Analysis

**Policy System** ([pkg/policy/policy.go](pkg/policy/policy.go)):
- Policy loaded from `~/.config/ORLY/policy.json` at startup
- `P` struct with unexported `rules` field (map[int]Rule)
- `PolicyManager` manages script runners for external policy scripts
- `LoadFromFile()` method exists for loading policy from disk
- No hot reload mechanism currently exists

**ACL System** ([pkg/acl/follows.go](pkg/acl/follows.go)):
- Separate from policy system
- Manages admin/owner/follows lists for write access control
- Fetches kind 3 events from relays
- Has callback mechanism for updates

**Event Handling** ([app/handle-event.go](app/handle-event.go)):213-226
- Special handling for NIP-43 events (join/leave requests)
- Pattern: Check kind early, process, return early

**Web UI**:
- Svelte-based component architecture
- Tab-based navigation in [app/web/src/App.svelte](app/web/src/App.svelte)
- API endpoints follow `/api/<feature>/<action>` pattern

## Feature 1: Dynamic Policy Configuration (Kind 12345)

### Design

**Event Kind:** 12345 (Relay Policy Configuration)
**Purpose:** Allow admins/owners to update policy configuration via Nostr event
**Security:** Only admins/owners can publish; only visible to admins/owners
**Process Flow:**
1. Admin/owner creates kind 12345 event with JSON policy in `content` field
2. Relay receives event via WebSocket
3. Validate sender is admin/owner
4. Pause policy manager (stop script runners)
5. Parse and validate JSON configuration
6. Apply new policy configuration
7. Persist to `~/.config/ORLY/policy.json`
8. Resume policy manager (restart script runners)
9. Send OK response

### Implementation Steps

#### Step 1.1: Define Kind Constant
**File:** Create `pkg/protocol/policyconfig/policyconfig.go`
```go
package policyconfig

const (
    // KindPolicyConfig is a relay-internal event for policy configuration updates
    // Only visible to admins and owners
    KindPolicyConfig uint16 = 12345
)
```

#### Step 1.2: Add Policy Hot Reload Methods
**File:** [pkg/policy/policy.go](pkg/policy/policy.go)

Add methods to `P` struct:
```go
// Reload loads policy from JSON bytes and applies it to the existing policy instance
// This pauses the policy manager, updates configuration, and resumes
func (p *P) Reload(policyJSON []byte) error

// Pause pauses the policy manager and stops all script runners
func (p *P) Pause() error

// Resume resumes the policy manager and restarts script runners
func (p *P) Resume() error

// SaveToFile persists the current policy configuration to disk
func (p *P) SaveToFile(configPath string) error
```

**Implementation Details:**
- `Reload()` should:
  - Call `Pause()` to stop all script runners
  - Unmarshal JSON into policy struct (using shadow struct pattern)
  - Validate configuration
  - Populate binary caches
  - Call `SaveToFile()` to persist
  - Call `Resume()` to restart scripts
  - Return error if any step fails

- `Pause()` should:
  - Iterate through `p.manager.runners` map
  - Call `Stop()` on each runner
  - Set a paused flag on the manager

- `Resume()` should:
  - Clear paused flag
  - Call `startPolicyIfExists()` to restart default script
  - Restart any rule-specific scripts that were running

- `SaveToFile()` should:
  - Marshal policy to JSON (using pJSON shadow struct)
  - Write atomically to config path (write to temp file, then rename)

#### Step 1.3: Handle Kind 12345 Events
**File:** [app/handle-event.go](app/handle-event.go)

Add handling after NIP-43 special events (after line 226):
```go
// Handle policy configuration update events (kind 12345)
case policyconfig.KindPolicyConfig:
    // Process policy config update and return early
    if err = l.HandlePolicyConfigUpdate(env.E); chk.E(err) {
        log.E.F("failed to process policy config update: %v", err)
        if err = Ok.Error(l, env, err.Error()); chk.E(err) {
            return
        }
        return
    }
    // Send OK response
    if err = Ok.Ok(l, env, "policy configuration updated"); chk.E(err) {
        return
    }
    return
```

Create new file: `app/handle-policy-config.go`
```go
// HandlePolicyConfigUpdate processes kind 12345 policy configuration events
// Only admins and owners can update policy configuration
func (l *Listener) HandlePolicyConfigUpdate(ev *event.E) error {
    // 1. Verify sender is admin or owner
    // 2. Parse JSON from event content
    // 3. Validate JSON structure
    // 4. Call l.policyManager.Reload(jsonBytes)
    // 5. Log success/failure
    return nil
}
```

**Security Checks:**
- Verify `ev.Pubkey` is in admins or owners list
- Validate JSON syntax before applying
- Catch all errors and return descriptive messages
- Log all policy update attempts (success and failure)

#### Step 1.4: Query Filtering (Optional)
**File:** [app/handle-req.go](app/handle-req.go)

Add filter to hide kind 12345 from non-admins:
```go
// In handleREQ, after ACL checks:
// Filter out policy config events (kind 12345) for non-admin users
if !isAdminOrOwner(l.authedPubkey.Load(), l.Admins, l.Owners) {
    // Remove kind 12345 from filter
    for _, f := range filters {
        f.Kinds.Remove(policyconfig.KindPolicyConfig)
    }
}
```

## Feature 2: Administrator Follow List Whitelisting

### Design

**Purpose:** Enable policy-based follow list whitelisting (separate from ACL follows)
**Use Case:** Policy admins can designate follows who get special policy privileges
**Configuration:**
```json
{
  "policy_admins": ["admin_pubkey_hex_1", "admin_pubkey_hex_2"],
  "policy_follow_whitelist_enabled": true,
  "rules": {
    "1": {
      "write_allow_follows": true  // Allow writes from policy admin follows
    }
  }
}
```

### Implementation Steps

#### Step 2.1: Extend Policy Configuration Structure
**File:** [pkg/policy/policy.go](pkg/policy/policy.go)

Extend `P` struct:
```go
type P struct {
    Kind          Kinds  `json:"kind"`
    rules         map[int]Rule
    Global        Rule   `json:"global"`
    DefaultPolicy string `json:"default_policy"`

    // New fields for follow list whitelisting
    PolicyAdmins                   []string `json:"policy_admins,omitempty"`
    PolicyFollowWhitelistEnabled   bool     `json:"policy_follow_whitelist_enabled,omitempty"`

    // Unexported cached data
    policyAdminsBin  [][]byte     // Binary cache for admin pubkeys
    policyFollows    [][]byte     // Cached follow list from policy admins
    policyFollowsMx  sync.RWMutex // Protect follows list

    manager *PolicyManager
}
```

Extend `Rule` struct:
```go
type Rule struct {
    // ... existing fields ...

    // New field for follow-based whitelisting
    WriteAllowFollows bool `json:"write_allow_follows,omitempty"`
    ReadAllowFollows  bool `json:"read_allow_follows,omitempty"`
}
```

Update `pJSON` shadow struct to include new fields.

#### Step 2.2: Add Follow List Fetching
**File:** [pkg/policy/policy.go](pkg/policy/policy.go)

Add methods:
```go
// FetchPolicyFollows fetches follow lists (kind 3) from database for policy admins
// This is called during policy load and can be called periodically
func (p *P) FetchPolicyFollows(db database.D) error {
    p.policyFollowsMx.Lock()
    defer p.policyFollowsMx.Unlock()

    // Clear existing follows
    p.policyFollows = nil

    // For each policy admin, query kind 3 events
    for _, adminPubkey := range p.policyAdminsBin {
        // Build filter for kind 3 from this admin
        // Query database for latest kind 3 event
        // Extract p-tags from event
        // Add to p.policyFollows list
    }

    return nil
}

// IsPolicyFollow checks if pubkey is in policy admin follows
func (p *P) IsPolicyFollow(pubkey []byte) bool {
    p.policyFollowsMx.RLock()
    defer p.policyFollowsMx.RUnlock()

    for _, follow := range p.policyFollows {
        if utils.FastEqual(pubkey, follow) {
            return true
        }
    }
    return false
}
```

#### Step 2.3: Integrate Follow Checking in Policy Rules
**File:** [pkg/policy/policy.go](pkg/policy/policy.go)

Update `checkRulePolicy()` method (around line 1062):
```go
// In write access checks, after checking write_allow list:
if access == "write" {
    // Check if follow-based whitelisting is enabled for this rule
    if rule.WriteAllowFollows && p.PolicyFollowWhitelistEnabled {
        if p.IsPolicyFollow(loggedInPubkey) {
            return true, nil  // Allow write from policy admin follow
        }
    }

    // Continue with existing write_allow checks...
}

// Similar for read access:
if access == "read" {
    if rule.ReadAllowFollows && p.PolicyFollowWhitelistEnabled {
        if p.IsPolicyFollow(loggedInPubkey) {
            return true, nil  // Allow read from policy admin follow
        }
    }
    // Continue with existing read_allow checks...
}
```

#### Step 2.4: Periodic Follow List Refresh
**File:** [pkg/policy/policy.go](pkg/policy/policy.go)

Add to `NewWithManager()`:
```go
// Start periodic follow list refresh for policy admins
if len(policy.PolicyAdmins) > 0 && policy.PolicyFollowWhitelistEnabled {
    go policy.startPeriodicFollowRefresh(ctx)
}
```

Add method:
```go
// startPeriodicFollowRefresh periodically fetches policy admin follow lists
func (p *P) startPeriodicFollowRefresh(ctx context.Context) {
    ticker := time.NewTicker(15 * time.Minute) // Refresh every 15 minutes
    defer ticker.Stop()

    // Fetch immediately on startup
    if err := p.FetchPolicyFollows(p.db); err != nil {
        log.E.F("failed to fetch policy follows: %v", err)
    }

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if err := p.FetchPolicyFollows(p.db); err != nil {
                log.E.F("failed to fetch policy follows: %v", err)
            } else {
                log.I.F("refreshed policy admin follow lists")
            }
        }
    }
}
```

**Note:** Need to pass database reference to policy manager. Update `NewWithManager()` signature:
```go
func NewWithManager(ctx context.Context, appName string, enabled bool, db *database.D) *P
```

## Feature 3: Web Interface for Policy Management

### Design

**Components:**
1. `PolicyView.svelte` - Main policy management UI
2. API endpoints for policy CRUD operations
3. JSON editor with validation
4. Follow list viewer

**UI Features:**
- View current policy configuration (read-only JSON display)
- Edit policy JSON with syntax highlighting
- Validate JSON before publishing
- Publish kind 12345 event to update policy
- View policy admin pubkeys
- View follow lists for each policy admin
- Add/remove policy admin pubkeys (updates and republishes config)

### Implementation Steps

#### Step 3.1: Create Policy View Component
**File:** `app/web/src/PolicyView.svelte`

Structure:
```svelte
<script>
    export let isLoggedIn = false;
    export let userRole = "";
    export let policyConfig = null;
    export let policyAdmins = [];
    export let policyFollows = [];
    export let isLoadingPolicy = false;
    export let policyMessage = "";
    export let policyMessageType = "info";
    export let policyEditJson = "";

    import { createEventDispatcher } from "svelte";
    const dispatch = createEventDispatcher();

    // Event handlers
    function loadPolicy() { dispatch("loadPolicy"); }
    function savePolicy() { dispatch("savePolicy"); }
    function validatePolicy() { dispatch("validatePolicy"); }
    function addPolicyAdmin() { dispatch("addPolicyAdmin"); }
    function removePolicyAdmin(pubkey) { dispatch("removePolicyAdmin", pubkey); }
    function refreshFollows() { dispatch("refreshFollows"); }
</script>

<div class="policy-view">
    <h2>Policy Configuration Management</h2>

    {#if isLoggedIn && (userRole === "owner" || userRole === "admin")}
        <!-- Policy JSON Editor Section -->
        <div class="policy-section">
            <h3>Policy Configuration</h3>
            <div class="policy-controls">
                <button on:click={loadPolicy}>🔄 Reload</button>
                <button on:click={validatePolicy}>✓ Validate</button>
                <button on:click={savePolicy}>📤 Publish Update</button>
            </div>

            <textarea
                class="policy-editor"
                bind:value={policyEditJson}
                spellcheck="false"
                placeholder="Policy JSON configuration..."
            />
        </div>

        <!-- Policy Admins Section -->
        <div class="policy-admins-section">
            <h3>Policy Administrators</h3>
            <p class="section-description">
                Policy admins can update configuration and their follows get whitelisted
                (if policy_follow_whitelist_enabled is true)
            </p>

            <div class="admin-list">
                {#each policyAdmins as admin}
                    <div class="admin-item">
                        <span class="admin-pubkey">{admin}</span>
                        <button
                            class="remove-btn"
                            on:click={() => removePolicyAdmin(admin)}
                        >
                            Remove
                        </button>
                    </div>
                {/each}
            </div>

            <div class="add-admin">
                <input
                    type="text"
                    placeholder="npub or hex pubkey"
                    id="new-admin-input"
                />
                <button on:click={addPolicyAdmin}>Add Admin</button>
            </div>
        </div>

        <!-- Follow List Section -->
        <div class="policy-follows-section">
            <h3>Policy Follow Whitelist</h3>
            <button on:click={refreshFollows}>🔄 Refresh Follows</button>

            <div class="follows-list">
                {#if policyFollows.length === 0}
                    <p class="no-follows">No follows loaded</p>
                {:else}
                    <p class="follows-count">
                        {policyFollows.length} pubkey(s) in whitelist
                    </p>
                    <div class="follows-grid">
                        {#each policyFollows as follow}
                            <div class="follow-item">{follow}</div>
                        {/each}
                    </div>
                {/if}
            </div>
        </div>

        <!-- Message Display -->
        {#if policyMessage}
            <div class="policy-message {policyMessageType}">
                {policyMessage}
            </div>
        {/if}
    {:else}
        <div class="access-denied">
            <p>Policy management is only available to relay administrators and owners.</p>
            {#if !isLoggedIn}
                <button on:click={() => dispatch("openLoginModal")}>
                    Login
                </button>
            {/if}
        </div>
    {/if}
</div>

<style>
    /* Policy-specific styling */
    .policy-view { /* ... */ }
    .policy-editor {
        width: 100%;
        min-height: 400px;
        font-family: 'Monaco', 'Courier New', monospace;
        font-size: 0.9em;
        padding: 1em;
        border: 1px solid var(--border-color);
        border-radius: 4px;
        background: var(--code-bg);
        color: var(--code-text);
    }
    /* ... more styles ... */
</style>
```

#### Step 3.2: Add Policy Tab to Main App
**File:** [app/web/src/App.svelte](app/web/src/App.svelte)

Add state variables (around line 94):
```javascript
// Policy management state
let policyConfig = null;
let policyAdmins = [];
let policyFollows = [];
let isLoadingPolicy = false;
let policyMessage = "";
let policyMessageType = "info";
let policyEditJson = "";
```

Add tab definition in `tabs` array (look for export/import/sprocket tabs):
```javascript
if (isLoggedIn && (userRole === "owner" || userRole === "admin")) {
    tabs.push({
        id: "policy",
        label: "Policy",
        icon: "🛡️",
        isSearchTab: false
    });
}
```

Add component import:
```javascript
import PolicyView from "./PolicyView.svelte";
```

Add view in main content area (look for {#if selectedTab === "sprocket"}):
```svelte
{:else if selectedTab === "policy"}
    <PolicyView
        {isLoggedIn}
        {userRole}
        {policyConfig}
        {policyAdmins}
        {policyFollows}
        {isLoadingPolicy}
        {policyMessage}
        {policyMessageType}
        bind:policyEditJson
        on:loadPolicy={handleLoadPolicy}
        on:savePolicy={handleSavePolicy}
        on:validatePolicy={handleValidatePolicy}
        on:addPolicyAdmin={handleAddPolicyAdmin}
        on:removePolicyAdmin={handleRemovePolicyAdmin}
        on:refreshFollows={handleRefreshFollows}
        on:openLoginModal={() => (showLoginModal = true)}
    />
```

Add event handlers:
```javascript
async function handleLoadPolicy() {
    isLoadingPolicy = true;
    policyMessage = "";

    try {
        const response = await fetch("/api/policy/config", {
            credentials: "include"
        });

        if (!response.ok) {
            throw new Error(`Failed to load policy: ${response.statusText}`);
        }

        const data = await response.json();
        policyConfig = data.config;
        policyEditJson = JSON.stringify(data.config, null, 2);
        policyAdmins = data.config.policy_admins || [];

        policyMessage = "Policy loaded successfully";
        policyMessageType = "success";
    } catch (error) {
        policyMessage = `Error loading policy: ${error.message}`;
        policyMessageType = "error";
        console.error("Error loading policy:", error);
    } finally {
        isLoadingPolicy = false;
    }
}

async function handleSavePolicy() {
    isLoadingPolicy = true;
    policyMessage = "";

    try {
        // Validate JSON first
        const config = JSON.parse(policyEditJson);

        // Publish kind 12345 event via websocket with auth
        const event = {
            kind: 12345,
            content: policyEditJson,
            tags: [],
            created_at: Math.floor(Date.now() / 1000)
        };

        const result = await publishEventWithAuth(event, userSigner);

        if (result.success) {
            policyMessage = "Policy updated successfully";
            policyMessageType = "success";
            // Reload to get updated config
            await handleLoadPolicy();
        } else {
            throw new Error(result.message || "Failed to publish policy update");
        }
    } catch (error) {
        policyMessage = `Error updating policy: ${error.message}`;
        policyMessageType = "error";
        console.error("Error updating policy:", error);
    } finally {
        isLoadingPolicy = false;
    }
}

function handleValidatePolicy() {
    try {
        JSON.parse(policyEditJson);
        policyMessage = "Policy JSON is valid ✓";
        policyMessageType = "success";
    } catch (error) {
        policyMessage = `Invalid JSON: ${error.message}`;
        policyMessageType = "error";
    }
}

async function handleRefreshFollows() {
    isLoadingPolicy = true;
    policyMessage = "";

    try {
        const response = await fetch("/api/policy/follows", {
            credentials: "include"
        });

        if (!response.ok) {
            throw new Error(`Failed to load follows: ${response.statusText}`);
        }

        const data = await response.json();
        policyFollows = data.follows || [];

        policyMessage = `Loaded ${policyFollows.length} follows`;
        policyMessageType = "success";
    } catch (error) {
        policyMessage = `Error loading follows: ${error.message}`;
        policyMessageType = "error";
        console.error("Error loading follows:", error);
    } finally {
        isLoadingPolicy = false;
    }
}

async function handleAddPolicyAdmin(event) {
    // Get input value
    const input = document.getElementById("new-admin-input");
    const pubkey = input.value.trim();

    if (!pubkey) {
        policyMessage = "Please enter a pubkey";
        policyMessageType = "error";
        return;
    }

    try {
        // Convert npub to hex if needed (implement or use nostr library)
        // Add to policy_admins array in config
        const config = JSON.parse(policyEditJson);
        if (!config.policy_admins) {
            config.policy_admins = [];
        }
        if (!config.policy_admins.includes(pubkey)) {
            config.policy_admins.push(pubkey);
            policyEditJson = JSON.stringify(config, null, 2);
            input.value = "";
            policyMessage = "Admin added (click Publish to save)";
            policyMessageType = "info";
        } else {
            policyMessage = "Admin already in list";
            policyMessageType = "warning";
        }
    } catch (error) {
        policyMessage = `Error adding admin: ${error.message}`;
        policyMessageType = "error";
    }
}

async function handleRemovePolicyAdmin(event) {
    const pubkey = event.detail;

    try {
        const config = JSON.parse(policyEditJson);
        if (config.policy_admins) {
            config.policy_admins = config.policy_admins.filter(p => p !== pubkey);
            policyEditJson = JSON.stringify(config, null, 2);
            policyMessage = "Admin removed (click Publish to save)";
            policyMessageType = "info";
        }
    } catch (error) {
        policyMessage = `Error removing admin: ${error.message}`;
        policyMessageType = "error";
    }
}
```

#### Step 3.3: Add API Endpoints
**File:** [app/server.go](app/server.go)

Add to route registration (around line 245):
```go
// Policy management endpoints (admin/owner only)
s.mux.HandleFunc("/api/policy/config", s.handlePolicyConfig)
s.mux.HandleFunc("/api/policy/follows", s.handlePolicyFollows)
```

Create new file: `app/handle-policy-api.go`
```go
package app

import (
    "encoding/json"
    "net/http"
    "lol.mleku.dev/log"
    "git.smesh.lol/orly/pkg/nostr/encoders/hex"
)

// handlePolicyConfig returns the current policy configuration
// GET /api/policy/config
func (s *Server) handlePolicyConfig(w http.ResponseWriter, r *http.Request) {
    // Verify authentication
    session, err := s.getSession(r)
    if err != nil || session == nil {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    // Verify user is admin or owner
    role := s.getUserRole(session.Pubkey)
    if role != "admin" && role != "owner" {
        http.Error(w, "Forbidden", http.StatusForbidden)
        return
    }

    // Get current policy configuration from policy manager
    // This requires adding a method to get the raw config
    config := s.policyManager.GetConfig() // Need to implement this

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "config": config,
    })
}

// handlePolicyFollows returns the policy admin follow lists
// GET /api/policy/follows
func (s *Server) handlePolicyFollows(w http.ResponseWriter, r *http.Request) {
    // Verify authentication
    session, err := s.getSession(r)
    if err != nil || session == nil {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    // Verify user is admin or owner
    role := s.getUserRole(session.Pubkey)
    if role != "admin" && role != "owner" {
        http.Error(w, "Forbidden", http.StatusForbidden)
        return
    }

    // Get policy follows from policy manager
    follows := s.policyManager.GetPolicyFollows() // Need to implement this

    // Convert to hex strings for JSON response
    followsHex := make([]string, len(follows))
    for i, f := range follows {
        followsHex[i] = hex.Enc(f)
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "follows": followsHex,
    })
}
```

**Note:** Need to add getter methods to policy manager:
```go
// GetConfig returns the current policy configuration as a map
// File: pkg/policy/policy.go
func (p *P) GetConfig() map[string]interface{} {
    // Marshal to JSON and back to get map representation
    jsonBytes, _ := json.Marshal(p)
    var config map[string]interface{}
    json.Unmarshal(jsonBytes, &config)
    return config
}

// GetPolicyFollows returns the current policy follow list
func (p *P) GetPolicyFollows() [][]byte {
    p.policyFollowsMx.RLock()
    defer p.policyFollowsMx.RUnlock()

    follows := make([][]byte, len(p.policyFollows))
    copy(follows, p.policyFollows)
    return follows
}
```

## Testing Strategy

### Unit Tests

1. **Policy Reload Tests** (`pkg/policy/policy_test.go`):
   - Test `Reload()` with valid JSON
   - Test `Reload()` with invalid JSON
   - Test `Pause()` and `Resume()` functionality
   - Test `SaveToFile()` atomic write

2. **Follow List Tests** (`pkg/policy/follows_test.go`):
   - Test `FetchPolicyFollows()` with mock database
   - Test `IsPolicyFollow()` with various inputs
   - Test follow list caching and expiry

3. **Handler Tests** (`app/handle-policy-config_test.go`):
   - Test kind 12345 handling with admin pubkey
   - Test kind 12345 rejection from non-admin
   - Test JSON validation errors

### Integration Tests

1. **End-to-End Policy Update**:
   - Publish kind 12345 event as admin
   - Verify policy reloaded
   - Verify new policy enforced
   - Verify policy persisted to disk

2. **Follow Whitelist E2E**:
   - Configure policy with follow whitelist enabled
   - Add admin pubkey to policy_admins
   - Publish kind 3 follow list for admin
   - Verify follows can write/read per policy rules

3. **Web UI E2E**:
   - Load policy via API
   - Edit and publish via UI
   - Verify changes applied
   - Check follow list display

## Security Considerations

1. **Authorization**:
   - Only admins/owners can publish kind 12345
   - Only admins/owners can access policy API endpoints
   - Policy events only visible to admins/owners in queries

2. **Validation**:
   - Strict JSON schema validation before applying
   - Rollback mechanism if policy fails to load
   - Catch all parsing errors

3. **Audit Trail**:
   - Log all policy update attempts
   - Store kind 12345 events in database for audit
   - Include who changed what and when

4. **Atomic Operations**:
   - Pause-update-resume must be atomic
   - File writes must be atomic (temp file + rename)
   - No partial updates on failure

## Migration Path

### Phase 1: Backend Foundation
1. Implement kind 12345 constant
2. Add policy reload methods
3. Add follow list support to policy
4. Test hot reload mechanism

### Phase 2: Event Handling
1. Add kind 12345 handler
2. Add API endpoints
3. Test event flow end-to-end

### Phase 3: Web UI
1. Create PolicyView component
2. Integrate into App.svelte
3. Add JSON editor
4. Test user workflows

### Phase 4: Testing & Documentation
1. Write comprehensive tests
2. Update CLAUDE.md
3. Create user documentation
4. Add examples to docs/

## Open Questions / Decisions Needed

1. **Policy Admin vs Relay Admin**:
   - Should policy_admins be separate from ORLY_ADMINS?
   - **Recommendation:** Yes, separate. Policy admins manage policy, relay admins manage relay.

2. **Follow List Refresh Frequency**:
   - How often to refresh policy admin follows?
   - **Recommendation:** 15 minutes (configurable via ORLY_POLICY_FOLLOW_REFRESH)

3. **Backward Compatibility**:
   - What happens to relays without policy_admins field?
   - **Recommendation:** Fall back to empty list, disabled by default

4. **Database Reference in Policy**:
   - Policy needs database reference for follow queries
   - **Recommendation:** Pass database to NewWithManager()

5. **Error Handling on Reload Failure**:
   - Should failed reload keep old policy or disable policy?
   - **Recommendation:** Keep old policy, log error, return error to client

## Success Criteria

1. ✅ Admin can publish kind 12345 event with new policy JSON
2. ✅ Relay receives event, validates sender, reloads policy without restart
3. ✅ Policy persisted to `~/.config/ORLY/policy.json`
4. ✅ Script runners paused during reload, resumed after
5. ✅ Policy admins can be configured in policy JSON
6. ✅ Policy admin follow lists fetched from database
7. ✅ Follow-based whitelisting enforced in policy rules
8. ✅ Web UI displays current policy configuration
9. ✅ Web UI allows editing and validation of policy JSON
10. ✅ Web UI shows policy admin follows
11. ✅ Only admins/owners can access policy management
12. ✅ All tests pass
13. ✅ Documentation updated

## Estimated Effort

- **Backend (Policy + Event Handling):** 8-12 hours
  - Policy reload methods: 3-4 hours
  - Follow list support: 3-4 hours
  - Event handling: 2-3 hours
  - Testing: 2-3 hours

- **API Endpoints:** 2-3 hours
  - Route setup: 1 hour
  - Handler implementation: 1-2 hours
  - Testing: 1 hour

- **Web UI:** 6-8 hours
  - PolicyView component: 3-4 hours
  - App integration: 2-3 hours
  - Styling and UX: 2-3 hours
  - Testing: 2 hours

- **Documentation & Testing:** 4-6 hours
  - Unit tests: 2-3 hours
  - Integration tests: 2-3 hours
  - Documentation: 2 hours

**Total:** 20-29 hours

## Dependencies

- No external dependencies required
- Uses existing ORLY infrastructure
- Compatible with current policy system

## Next Steps

1. Review and approve this plan
2. Clarify open questions/decisions
3. Begin implementation in phases
4. Iterative testing and refinement
