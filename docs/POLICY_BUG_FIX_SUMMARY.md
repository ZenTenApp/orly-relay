# Policy System Bug Fix Summary

## Bug Report
**Issue:** Kind 1 events were being accepted even though the policy whitelist only contained kind 4678.

## Root Cause Analysis

The relay had **TWO critical bugs** in the policy system that worked together to create a security vulnerability:

### Bug #1: Hardcoded `return true` in `checkKindsPolicy()`
**Location:** [`pkg/policy/policy.go:1010`](pkg/policy/policy.go#L1010)

```go
// BEFORE (BUG):
// No specific rules (maybe global rule exists) - allow all kinds
return true

// AFTER (FIXED):
// No specific rules (maybe global rule exists) - fall back to default policy
return p.getDefaultPolicyAction()
```

**Problem:** When no whitelist, blacklist, or rules were present, the function returned `true` unconditionally, ignoring the `default_policy` configuration.

**Impact:** Empty policy configurations would allow ALL event kinds.

---

### Bug #2: Silent Failure on Config Load Error
**Location:** [`pkg/policy/policy.go:363-378`](pkg/policy/policy.go#L363-L378)

```go
// BEFORE (BUG):
if err := policy.LoadFromFile(configPath); err != nil {
    log.W.F("failed to load policy configuration from %s: %v", configPath, err)
    log.I.F("using default policy configuration")
}

// AFTER (FIXED):
if err := policy.LoadFromFile(configPath); err != nil {
    log.E.F("FATAL: Policy system is ENABLED (ORLY_POLICY_ENABLED=true) but configuration failed to load from %s: %v", configPath, err)
    log.E.F("The relay cannot start with an invalid policy configuration.")
    log.E.F("Fix: Either disable the policy system (ORLY_POLICY_ENABLED=false) or ensure %s exists and contains valid JSON", configPath)
    panic(fmt.Sprintf("fatal policy configuration error: %v", err))
}
```

**Problem:** When policy was enabled but `policy.json` failed to load:
- Only logged a WARNING (not fatal)
- Continued with empty policy object (no whitelist, no rules)
- Empty policy + Bug #1 = allowed ALL events
- Relay appeared to be "protected" but was actually wide open

**Impact:** **Critical security vulnerability** - misconfigured policy files would silently allow all events.

---

## Combined Effect

When a relay operator:
1. Enabled policy system (`ORLY_POLICY_ENABLED=true`)
2. Had a missing, malformed, or inaccessible `policy.json` file

The relay would:
- ❌ Log "policy allowed event" (appearing to work)
- ❌ Have empty whitelist/rules (silent failure)
- ❌ Fall through to hardcoded `return true` (Bug #1)
- ✅ **Allow ALL event kinds** (complete bypass)

---

## Fixes Applied

### Fix #1: Respect `default_policy` Setting
Changed `checkKindsPolicy()` to return `p.getDefaultPolicyAction()` instead of hardcoded `true`.

**Result:** When no whitelist/rules exist, the policy respects the `default_policy` configuration (either "allow" or "deny").

### Fix #2: Fail-Fast on Config Error
Changed `NewWithManager()` to **panic immediately** if policy is enabled but config fails to load.

**Result:** Relay refuses to start with invalid configuration, forcing operator to fix it.

---

## Test Coverage

### New Tests Added

1. **`TestBugFix_FailSafeWhenConfigMissing`** - Verifies panic on missing config
2. **`TestBugFix_EmptyWhitelistRespectsDefaultPolicy`** - Tests both deny and allow defaults
3. **`TestBugReproduction_*`** - Reproduces the exact scenario from the bug report

### Existing Tests Updated

- **`TestNewWithManager`** - Now handles both enabled and disabled policy scenarios
- All existing whitelist tests continue to pass ✅

---

## Behavior Changes

### Before Fix
```
Policy System: ENABLED ✅
Config File: MISSING ❌
Logs: "failed to load policy configuration" (warning)
Result: Allow ALL events 🚨

Policy System: ENABLED ✅
Config File: { "whitelist": [4678] } ✅
Logs: "policy allowed event" for kind 1
Result: Allow kind 1 event 🚨
```

### After Fix
```
Policy System: ENABLED ✅
Config File: MISSING ❌
Result: PANIC - relay refuses to start 🛑

Policy System: ENABLED ✅
Config File: { "whitelist": [4678] } ✅
Logs: "policy rejected event" for kind 1
Result: Reject kind 1 event ✅
```

---

## Migration Guide for Operators

### If Your Relay Panics After Upgrade

**Error Message:**
```
FATAL: Policy system is ENABLED (ORLY_POLICY_ENABLED=true) but configuration failed to load
panic: fatal policy configuration error: policy configuration file does not exist
```

**Resolution Options:**

1. **Create valid `policy.json`:**
   ```bash
   mkdir -p ~/.config/ORLY
   cat > ~/.config/ORLY/policy.json << 'EOF'
   {
     "default_policy": "allow",
     "kind": {
       "whitelist": [1, 3, 4, 5, 6, 7]
     },
     "rules": {}
   }
   EOF
   ```

2. **Disable policy system (temporary):**
   ```bash
   # In your systemd service file:
   Environment="ORLY_POLICY_ENABLED=false"

   sudo systemctl daemon-reload
   sudo systemctl restart orly
   ```

---

## Security Impact

**Severity:** 🔴 **CRITICAL**

**CVE-Like Description:**
> When `ORLY_POLICY_ENABLED=true` is set but the policy configuration file fails to load (missing file, permission error, or malformed JSON), the relay silently bypasses all policy checks and allows events of any kind, defeating the intended access control mechanism.

**Affected Versions:** All versions prior to this fix

**Fixed Versions:** Current HEAD after commit [TBD]

**CVSS-like:** Configuration-dependent vulnerability requiring operator misconfiguration

---

## Verification

To verify the fix is working:

1. **Test with valid config:**
   ```bash
   # Should start normally
   ORLY_POLICY_ENABLED=true ./orly
   # Logs: "loaded policy configuration from ~/.config/ORLY/policy.json"
   ```

2. **Test with missing config:**
   ```bash
   # Should panic immediately
   mv ~/.config/ORLY/policy.json ~/.config/ORLY/policy.json.bak
   ORLY_POLICY_ENABLED=true ./orly
   # Expected: FATAL error and panic
   ```

3. **Test whitelist enforcement:**
   ```bash
   # Create whitelist with only kind 4678
   echo '{"kind":{"whitelist":[4678]},"rules":{}}' > ~/.config/ORLY/policy.json

   # Try to send kind 1 event
   # Expected: "policy rejected event" or "event blocked by policy"
   ```

---

## Files Modified

- [`pkg/policy/policy.go`](pkg/policy/policy.go) - Core fixes
- [`pkg/policy/bug_reproduction_test.go`](pkg/policy/bug_reproduction_test.go) - New test file
- [`pkg/policy/policy_test.go`](pkg/policy/policy_test.go) - Updated existing tests

---

## Related Documentation

- [Policy Usage Guide](docs/POLICY_USAGE_GUIDE.md)
- [Policy Troubleshooting](docs/POLICY_TROUBLESHOOTING.md)
- [CLAUDE.md](CLAUDE.md) - Build and configuration instructions

---

## Credits

**Bug Reported By:** User via client relay (relay1.zenotp.app)

**Root Cause Analysis:** Deep investigation of policy evaluation flow

**Fix Verified:** All tests passing, including reproduction of original bug scenario
