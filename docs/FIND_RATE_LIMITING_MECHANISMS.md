# FIND Rate Limiting Mechanisms (Non-Monetary, Non-PoW)

## Overview

This document explores mechanisms to rate limit name registrations in the FIND protocol without requiring:
- Security deposits or payments
- Monetary mechanisms (Lightning, ecash, etc.)
- Proof of work (computational puzzles)

The goal is to prevent spam and name squatting while maintaining decentralization and accessibility.

---

## 1. Time-Based Mechanisms

### 1.1 Proposal-to-Ratification Delay

**Concept:** Mandatory waiting period between submitting a registration proposal and consensus ratification.

**Implementation:**
```go
type ProposalDelay struct {
    MinDelay    time.Duration // e.g., 1 hour
    MaxDelay    time.Duration // e.g., 24 hours
    GracePeriod time.Duration // Random jitter to prevent timing attacks
}

func (r *RegistryService) validateProposalTiming(proposal *Proposal) error {
    elapsed := time.Since(proposal.CreatedAt)
    minRequired := r.config.ProposalDelay.MinDelay

    if elapsed < minRequired {
        return fmt.Errorf("proposal must age %v before ratification (current: %v)",
            minRequired, elapsed)
    }

    return nil
}
```

**Advantages:**
- Simple to implement
- Gives community time to review and object
- Prevents rapid-fire squatting
- Allows for manual intervention in disputes

**Disadvantages:**
- Poor UX (users wait hours/days)
- Doesn't prevent determined attackers with patience
- Vulnerable to timing attacks (frontrunning)

**Variations:**
- **Progressive Delays:** First name = 1 hour, second = 6 hours, third = 24 hours, etc.
- **Random Delays:** Each proposal gets random delay within range to prevent prediction
- **Peak-Time Penalties:** Longer delays during high registration volume

---

### 1.2 Per-Account Cooldown Periods

**Concept:** Limit how frequently a single npub can register names.

**Implementation:**
```go
type RateLimiter struct {
    registrations map[string][]time.Time // npub -> registration timestamps
    cooldown      time.Duration          // e.g., 7 days
    maxPerPeriod  int                    // e.g., 3 names per week
}

func (r *RateLimiter) canRegister(npub string, now time.Time) (bool, time.Duration) {
    timestamps := r.registrations[npub]

    // Remove expired timestamps
    cutoff := now.Add(-r.cooldown)
    active := filterAfter(timestamps, cutoff)

    if len(active) >= r.maxPerPeriod {
        oldestExpiry := active[0].Add(r.cooldown)
        waitTime := oldestExpiry.Sub(now)
        return false, waitTime
    }

    return true, 0
}
```

**Advantages:**
- Directly limits per-user registration rate
- Configurable (relays can set own limits)
- Persistent across sessions

**Disadvantages:**
- Easy to bypass with multiple npubs
- Requires state tracking across registry services
- May be too restrictive for legitimate bulk registrations

**Variations:**
- **Sliding Window:** Count registrations in last N days
- **Token Bucket:** Allow bursts but enforce long-term average
- **Decay Model:** Cooldown decreases over time (1 day → 6 hours → 1 hour)

---

### 1.3 Account Age Requirements

**Concept:** Npubs must be a certain age before they can register names.

**Implementation:**
```go
func (r *RegistryService) validateAccountAge(npub string, minAge time.Duration) error {
    // Query oldest event from this npub across known relays
    oldestEvent, err := r.getOldestEventByAuthor(npub)
    if err != nil {
        return fmt.Errorf("cannot determine account age: %w", err)
    }

    accountAge := time.Since(oldestEvent.CreatedAt)
    if accountAge < minAge {
        return fmt.Errorf("account must be %v old (current: %v)", minAge, accountAge)
    }

    return nil
}
```

**Advantages:**
- Prevents throwaway account spam
- Encourages long-term participation
- No ongoing cost to users

**Disadvantages:**
- Barrier for new users
- Can be gamed with pre-aged accounts
- Requires historical event data

**Variations:**
- **Tiered Ages:** Basic names require 30 days, premium require 90 days
- **Activity Threshold:** Not just age, but "active" age (X events published)

---

## 2. Web of Trust (WoT) Mechanisms

### 2.1 Follow Count Requirements

**Concept:** Require minimum follow count from trusted accounts to register names.

**Implementation:**
```go
type WoTValidator struct {
    minFollowers    int      // e.g., 5 followers
    trustedAccounts []string // Bootstrap trusted npubs
}

func (v *WoTValidator) validateFollowCount(npub string) error {
    // Query kind 3 events that include this npub in follow list
    followers, err := v.queryFollowers(npub)
    if err != nil {
        return err
    }

    // Count only followers who are themselves trusted
    trustedFollowers := 0
    for _, follower := range followers {
        if v.isTrusted(follower) {
            trustedFollowers++
        }
    }

    if trustedFollowers < v.minFollowers {
        return fmt.Errorf("need %d trusted followers, have %d",
            v.minFollowers, trustedFollowers)
    }

    return nil
}
```

**Advantages:**
- Leverages existing Nostr social graph
- Self-regulating (community decides who's trusted)
- Sybil-resistant if trust graph is diverse

**Disadvantages:**
- Chicken-and-egg for new users
- Can create gatekeeping
- Vulnerable to follow-for-follow schemes

**Variations:**
- **Weighted Followers:** High-reputation followers count more
- **Mutual Follows:** Require bidirectional relationships
- **Follow Depth:** Count 2-hop or 3-hop follows

---

### 2.2 Endorsement/Vouching System

**Concept:** Existing name holders can vouch for new registrants.

**Implementation:**
```go
// Kind 30110: Name Registration Endorsement
type Endorsement struct {
    Voucher   string // npub of existing name holder
    Vouchee   string // npub seeking registration
    NamesSeen int    // How many names voucher has endorsed (spam detection)
}

func (r *RegistryService) validateEndorsements(proposal *Proposal) error {
    // Query endorsements for this npub
    endorsements, err := r.queryEndorsements(proposal.Author)
    if err != nil {
        return err
    }

    // Require at least 2 endorsements from different name holders
    uniqueVouchers := make(map[string]bool)
    for _, e := range endorsements {
        // Check voucher holds a name
        if r.holdsActiveName(e.Voucher) {
            uniqueVouchers[e.Voucher] = true
        }
    }

    if len(uniqueVouchers) < 2 {
        return fmt.Errorf("need 2 endorsements from name holders, have %d",
            len(uniqueVouchers))
    }

    return nil
}
```

**Advantages:**
- Creates social accountability
- Name holders have "skin in the game"
- Can revoke endorsements if abused

**Disadvantages:**
- Requires active participation from name holders
- Can create favoritism/cliques
- Vouchers may sell endorsements

**Variations:**
- **Limited Vouches:** Each name holder can vouch for max N users per period
- **Reputation Cost:** Vouching for spammer reduces voucher's reputation
- **Delegation Chains:** Vouched users can vouch others (with decay)

---

### 2.3 Activity History Requirements

**Concept:** Require meaningful Nostr activity before allowing registration.

**Implementation:**
```go
type ActivityRequirements struct {
    MinEvents      int           // e.g., 50 events
    MinTimespan    time.Duration // e.g., 30 days
    RequiredKinds  []int         // Must have posted notes, not just kind 0
    MinUniqueRelays int          // Must use multiple relays
}

func (r *RegistryService) validateActivity(npub string, reqs ActivityRequirements) error {
    events, err := r.queryUserEvents(npub)
    if err != nil {
        return err
    }

    // Check event count
    if len(events) < reqs.MinEvents {
        return fmt.Errorf("need %d events, have %d", reqs.MinEvents, len(events))
    }

    // Check timespan
    oldest := events[0].CreatedAt
    newest := events[len(events)-1].CreatedAt
    timespan := newest.Sub(oldest)
    if timespan < reqs.MinTimespan {
        return fmt.Errorf("activity must span %v, current span: %v",
            reqs.MinTimespan, timespan)
    }

    // Check event diversity
    kinds := make(map[int]bool)
    for _, e := range events {
        kinds[e.Kind] = true
    }

    hasRequiredKinds := true
    for _, kind := range reqs.RequiredKinds {
        if !kinds[kind] {
            hasRequiredKinds = false
            break
        }
    }

    if !hasRequiredKinds {
        return fmt.Errorf("missing required event kinds")
    }

    return nil
}
```

**Advantages:**
- Rewards active community members
- Hard to fake authentic activity
- Aligns with Nostr values (participation)

**Disadvantages:**
- High barrier for new users
- Can be gamed with bot activity
- Definition of "meaningful" is subjective

**Variations:**
- **Engagement Metrics:** Require replies, reactions, zaps received
- **Content Quality:** Use NIP-32 labels to filter quality content
- **Relay Diversity:** Must have published to N different relays

---

## 3. Multi-Phase Verification

### 3.1 Two-Phase Commit with Challenge

**Concept:** Proposal → Challenge → Response → Ratification

**Implementation:**
```go
// Phase 1: Submit proposal (kind 30100)
type RegistrationProposal struct {
    Name   string
    Action string // "register"
}

// Phase 2: Registry issues challenge (kind 20110)
type RegistrationChallenge struct {
    ProposalID string
    Challenge  string // Random challenge string
    IssuedAt   time.Time
    ExpiresAt  time.Time
}

// Phase 3: User responds (kind 20111)
type ChallengeResponse struct {
    ChallengeID string
    Response    string // Signed challenge
    ProposalID  string
}

func (r *RegistryService) processProposal(proposal *Proposal) {
    // Generate random challenge
    challenge := generateRandomChallenge()

    // Publish challenge event
    challengeEvent := &ChallengeEvent{
        ProposalID: proposal.ID,
        Challenge:  challenge,
        ExpiresAt:  time.Now().Add(5 * time.Minute),
    }
    r.publishChallenge(challengeEvent)

    // Wait for response
    // If valid response received within window, proceed with attestation
}
```

**Advantages:**
- Proves user is actively monitoring
- Prevents pre-signed bulk registrations
- Adds friction without monetary cost

**Disadvantages:**
- Requires active participation (can't be automated)
- Poor UX (multiple steps)
- Vulnerable to automated response systems

**Variations:**
- **Time-Delayed Challenge:** Challenge issued X hours after proposal
- **Multi-Registry Challenges:** Must respond to challenges from multiple services
- **Progressive Challenges:** Later names require harder challenges

---

### 3.2 Multi-Signature Requirements

**Concept:** Require signatures from multiple devices/keys to prove human operator.

**Implementation:**
```go
type MultiSigProposal struct {
    Name           string
    PrimaryKey     string   // Main npub
    SecondaryKeys  []string // Additional npubs that must co-sign
    Signatures     []Signature
}

func (r *RegistryService) validateMultiSig(proposal *MultiSigProposal) error {
    // Require at least 2 signatures from different keys
    if len(proposal.Signatures) < 2 {
        return fmt.Errorf("need at least 2 signatures")
    }

    // Verify each signature
    for _, sig := range proposal.Signatures {
        if !verifySignature(proposal.Name, sig) {
            return fmt.Errorf("invalid signature from %s", sig.Pubkey)
        }
    }

    // Ensure signatures are from different keys
    uniqueKeys := make(map[string]bool)
    for _, sig := range proposal.Signatures {
        uniqueKeys[sig.Pubkey] = true
    }

    if len(uniqueKeys) < 2 {
        return fmt.Errorf("signatures must be from distinct keys")
    }

    return nil
}
```

**Advantages:**
- Harder to automate at scale
- Proves access to multiple devices
- No external dependencies

**Disadvantages:**
- Complex UX (managing multiple keys)
- Still bypassable with multiple hardware keys
- May lose access if secondary key lost

---

## 4. Lottery and Randomization

### 4.1 Random Selection Among Competing Proposals

**Concept:** When multiple proposals for same name arrive, randomly select winner.

**Implementation:**
```go
func (r *RegistryService) selectWinner(proposals []*Proposal) *Proposal {
    if len(proposals) == 1 {
        return proposals[0]
    }

    // Use deterministic randomness based on block hash or similar
    seed := r.getConsensusSeed() // From latest Bitcoin block hash, etc.

    // Create weighted lottery based on account age, reputation, etc.
    weights := make([]int, len(proposals))
    for i, p := range proposals {
        weights[i] = r.calculateWeight(p.Author)
    }

    // Select winner
    rng := rand.New(rand.NewSource(seed))
    winner := weightedRandomSelect(proposals, weights, rng)

    return winner
}

func (r *RegistryService) calculateWeight(npub string) int {
    // Base weight: 1
    weight := 1

    // +1 for each month of account age (max 12)
    accountAge := r.getAccountAge(npub)
    weight += min(int(accountAge.Hours()/730), 12)

    // +1 for each 100 events (max 10)
    eventCount := r.getEventCount(npub)
    weight += min(eventCount/100, 10)

    // +1 for each trusted follower (max 20)
    followerCount := r.getTrustedFollowerCount(npub)
    weight += min(followerCount, 20)

    return weight
}
```

**Advantages:**
- Fair chance for all participants
- Can weight by reputation without hard gatekeeping
- Discourages squatting (no guarantee of winning)

**Disadvantages:**
- Winners may feel arbitrary
- Still requires sybil resistance (or attackers spam proposals)
- Requires consensus on randomness source

**Variations:**
- **Time-Weighted Lottery:** Earlier proposals have slightly higher odds
- **Reputation-Only Lottery:** Only weight by WoT score
- **Periodic Lotteries:** Batch proposals weekly, run lottery for all conflicts

---

### 4.2 Queue System with Priority Ranking

**Concept:** Proposals enter queue, priority determined by non-transferable metrics.

**Implementation:**
```go
type ProposalQueue struct {
    proposals []*ScoredProposal
}

type ScoredProposal struct {
    Proposal *Proposal
    Score    int
}

func (r *RegistryService) scoreProposal(p *Proposal) int {
    score := 0

    // Account age contribution (0-30 points)
    accountAge := r.getAccountAge(p.Author)
    score += min(int(accountAge.Hours()/24), 30) // 1 point per day, max 30

    // Event count contribution (0-20 points)
    eventCount := r.getEventCount(p.Author)
    score += min(eventCount/10, 20) // 1 point per 10 events, max 20

    // WoT contribution (0-30 points)
    wotScore := r.getWoTScore(p.Author)
    score += min(wotScore, 30)

    // Endorsements (0-20 points)
    endorsements := r.getEndorsementCount(p.Author)
    score += min(endorsements*5, 20) // 5 points per endorsement, max 20

    return score
}

func (q *ProposalQueue) process() *Proposal {
    if len(q.proposals) == 0 {
        return nil
    }

    // Sort by score (descending)
    sort.Slice(q.proposals, func(i, j int) bool {
        return q.proposals[i].Score > q.proposals[j].Score
    })

    // Process highest score
    winner := q.proposals[0]
    q.proposals = q.proposals[1:]

    return winner.Proposal
}
```

**Advantages:**
- Transparent, merit-based selection
- Rewards long-term participation
- Predictable for users (can see their score)

**Disadvantages:**
- Complex scoring function
- May favor old accounts over new legitimate users
- Gaming possible if score calculation public

---

## 5. Behavioral Analysis

### 5.1 Pattern Detection

**Concept:** Detect and flag suspicious registration patterns.

**Implementation:**
```go
type BehaviorAnalyzer struct {
    recentProposals map[string][]*Proposal // IP/relay -> proposals
    suspiciousScore map[string]int         // npub -> suspicion score
}

func (b *BehaviorAnalyzer) analyzeProposal(p *Proposal) (suspicious bool, reason string) {
    score := 0

    // Check registration frequency
    if b.recentProposalCount(p.Author, 1*time.Hour) > 5 {
        score += 20
    }

    // Check name similarity (registering foo1, foo2, foo3, ...)
    if b.hasSequentialNames(p.Author) {
        score += 30
    }

    // Check relay diversity (all from same relay = suspicious)
    if b.relayDiversity(p.Author) < 2 {
        score += 15
    }

    // Check timestamp patterns (all proposals at exact intervals)
    if b.hasRegularIntervals(p.Author) {
        score += 25
    }

    // Check for dictionary attack patterns
    if b.isDictionaryAttack(p.Author) {
        score += 40
    }

    if score > 50 {
        return true, b.generateReason(score)
    }

    return false, ""
}
```

**Advantages:**
- Catches automated attacks
- No burden on legitimate users
- Adaptive (can tune detection rules)

**Disadvantages:**
- False positives possible
- Requires heuristic development
- Attackers can adapt

**Variations:**
- **Machine Learning:** Train model on spam vs. legitimate patterns
- **Collaborative Filtering:** Share suspicious patterns across registry services
- **Progressive Restrictions:** Suspicious users face longer delays

---

### 5.2 Diversity Requirements

**Concept:** Require proposals to exhibit "natural" diversity patterns.

**Implementation:**
```go
type DiversityRequirements struct {
    MinRelays      int           // Must use >= N relays
    MinTimeJitter  time.Duration // Registrations can't be exactly spaced
    MaxSimilarity  float64       // Names can't be too similar (Levenshtein distance)
}

func (r *RegistryService) validateDiversity(npub string, reqs DiversityRequirements) error {
    proposals := r.getProposalsByAuthor(npub)

    // Check relay diversity
    relays := make(map[string]bool)
    for _, p := range proposals {
        relays[p.SeenOnRelay] = true
    }
    if len(relays) < reqs.MinRelays {
        return fmt.Errorf("must use %d different relays", reqs.MinRelays)
    }

    // Check timestamp jitter
    if len(proposals) > 1 {
        intervals := make([]time.Duration, len(proposals)-1)
        for i := 1; i < len(proposals); i++ {
            intervals[i-1] = proposals[i].CreatedAt.Sub(proposals[i-1].CreatedAt)
        }

        // If all intervals are suspiciously similar (< 10% variance), reject
        variance := calculateVariance(intervals)
        avgInterval := calculateAverage(intervals)
        if variance/avgInterval < 0.1 {
            return fmt.Errorf("timestamps too regular, appears automated")
        }
    }

    // Check name similarity
    for i := 0; i < len(proposals); i++ {
        for j := i + 1; j < len(proposals); j++ {
            similarity := levenshteinSimilarity(proposals[i].Name, proposals[j].Name)
            if similarity > reqs.MaxSimilarity {
                return fmt.Errorf("names too similar: %s and %s",
                    proposals[i].Name, proposals[j].Name)
            }
        }
    }

    return nil
}
```

**Advantages:**
- Natural requirement for humans
- Hard for bots to fake convincingly
- Doesn't require state or external data

**Disadvantages:**
- May flag legitimate bulk registrations
- Requires careful threshold tuning
- Can be bypassed with sufficient effort

---

## 6. Hybrid Approaches

### 6.1 Graduated Trust Model

**Concept:** Combine multiple mechanisms with progressive unlock.

```
Level 0 (New User):
- Account must be 7 days old
- Must have 10 events published
- Can register 1 name every 30 days
- 24-hour proposal delay
- Requires 2 endorsements

Level 1 (Established User):
- Account must be 90 days old
- Must have 100 events, 10 followers
- Can register 3 names every 30 days
- 6-hour proposal delay
- Requires 1 endorsement

Level 2 (Trusted User):
- Account must be 365 days old
- Must have 1000 events, 50 followers
- Can register 10 names every 30 days
- 1-hour proposal delay
- No endorsement required

Level 3 (Name Holder):
- Already holds an active name
- Can register unlimited subdomains under owned names
- Can register 5 TLDs every 30 days
- Instant proposal for subdomains
- Can vouch for others
```

**Implementation:**
```go
type UserLevel struct {
    Level          int
    Requirements   Requirements
    Privileges     Privileges
}

type Requirements struct {
    MinAccountAge   time.Duration
    MinEvents       int
    MinFollowers    int
    MinActiveNames  int
}

type Privileges struct {
    MaxNamesPerPeriod int
    ProposalDelay     time.Duration
    EndorsementsReq   int
    CanVouch          bool
}

func (r *RegistryService) getUserLevel(npub string) UserLevel {
    age := r.getAccountAge(npub)
    events := r.getEventCount(npub)
    followers := r.getFollowerCount(npub)
    names := r.getActiveNameCount(npub)

    // Check Level 3
    if names > 0 {
        return UserLevel{
            Level: 3,
            Privileges: Privileges{
                MaxNamesPerPeriod: 5,
                ProposalDelay:     0,
                EndorsementsReq:   0,
                CanVouch:          true,
            },
        }
    }

    // Check Level 2
    if age >= 365*24*time.Hour && events >= 1000 && followers >= 50 {
        return UserLevel{
            Level: 2,
            Privileges: Privileges{
                MaxNamesPerPeriod: 10,
                ProposalDelay:     1 * time.Hour,
                EndorsementsReq:   0,
                CanVouch:          false,
            },
        }
    }

    // Check Level 1
    if age >= 90*24*time.Hour && events >= 100 && followers >= 10 {
        return UserLevel{
            Level: 1,
            Privileges: Privileges{
                MaxNamesPerPeriod: 3,
                ProposalDelay:     6 * time.Hour,
                EndorsementsReq:   1,
                CanVouch:          false,
            },
        }
    }

    // Default: Level 0
    return UserLevel{
        Level: 0,
        Privileges: Privileges{
            MaxNamesPerPeriod: 1,
            ProposalDelay:     24 * time.Hour,
            EndorsementsReq:   2,
            CanVouch:          false,
        },
    }
}
```

**Advantages:**
- Flexible and granular
- Rewards participation without hard barriers
- Self-regulating (community grows trust over time)
- Discourages throwaway accounts

**Disadvantages:**
- Complex to implement and explain
- May still be gamed by determined attackers
- Requires careful balance of thresholds

---

## 7. Recommended Hybrid Implementation

For FIND, I recommend combining these mechanisms:

### Base Layer: Time + WoT
```go
type BaseRequirements struct {
    // Minimum account requirements
    MinAccountAge       time.Duration  // 30 days
    MinPublishedEvents  int            // 20 events
    MinEventKinds       []int          // Must have kind 1 (notes)

    // WoT requirements
    MinWoTScore         float64        // 0.01 (very low threshold)
    MinTrustedFollowers int            // 2 followers from trusted accounts

    // Proposal timing
    ProposalDelay       time.Duration  // 6 hours
}
```

### Rate Limiting Layer: Progressive Cooldowns
```go
type RateLimits struct {
    // First name: 7 day cooldown after
    // Second name: 14 day cooldown
    // Third name: 30 day cooldown
    // Fourth+: 60 day cooldown

    GetCooldown func(registrationCount int) time.Duration
}
```

### Reputation Layer: Graduated Trust
```go
// Users with existing names get faster registration
// Users with high WoT scores get reduced delays
// Users with endorsements bypass some checks
```

### Detection Layer: Behavioral Analysis
```go
// Flag suspicious patterns
// Require manual review for flagged accounts
// Share blocklists between registry services
```

This hybrid approach:
- ✅ Low barrier for new legitimate users (30 days + minimal activity)
- ✅ Strong sybil resistance (WoT + account age)
- ✅ Prevents rapid squatting (progressive cooldowns)
- ✅ Rewards participation (graduated trust)
- ✅ Catches automation (behavioral analysis)
- ✅ No monetary cost
- ✅ No proof of work
- ✅ Decentralized (no central authority)

---

## 8. Comparison Matrix

| Mechanism | Sybil Resistance | UX Impact | Implementation Complexity | Bypass Difficulty |
|-----------|------------------|-----------|---------------------------|-------------------|
| Proposal Delay | Low | High | Low | Low |
| Per-Account Cooldown | Medium | Medium | Low | Low (multiple keys) |
| Account Age | Medium | Low | Low | Medium (pre-age accounts) |
| Follow Count | High | Medium | Medium | High (requires real follows) |
| Endorsement System | High | High | High | High (requires cooperation) |
| Activity History | High | Low | Medium | High (must fake real activity) |
| Multi-Phase Commit | Medium | High | Medium | Medium (can automate) |
| Lottery System | Medium | Medium | High | Medium (sybil can spam proposals) |
| Queue/Priority | High | Low | High | High (merit-based) |
| Behavioral Analysis | High | Low | Very High | Very High (adaptive) |
| **Hybrid Graduated** | **Very High** | **Medium** | **High** | **Very High** |

---

## 9. Attack Scenarios and Mitigations

### Scenario 1: Sybil Attack (1000 throwaway npubs)
**Mitigation:** Account age + activity requirements filter out new accounts. WoT requirements prevent isolated accounts from registering.

### Scenario 2: Pre-Aged Accounts
**Attacker creates accounts months in advance**
**Mitigation:** Activity history requirements force ongoing engagement. Behavioral analysis detects coordinated registration waves.

### Scenario 3: Follow-for-Follow Rings
**Attackers create mutual follow networks**
**Mitigation:** WoT decay for insular networks. Only follows from trusted/bootstrapped accounts count.

### Scenario 4: Bulk Registration by Legitimate User
**Company wants 100 names for project**
**Mitigation:** Manual exception process for verified organizations. Higher-level users get higher quotas.

### Scenario 5: Frontrunning
**Attacker monitors proposals and submits competing proposal**
**Mitigation:** Proposal delay + lottery system makes frontrunning less effective. Random selection among competing proposals.

---

## 10. Configuration Recommendations

```go
// Conservative (strict anti-spam)
conservative := RateLimitConfig{
    MinAccountAge:       90 * 24 * time.Hour,  // 90 days
    MinEvents:           100,
    MinFollowers:        10,
    ProposalDelay:       24 * time.Hour,
    CooldownPeriod:      30 * 24 * time.Hour,
    MaxNamesPerAccount:  5,
}

// Balanced (recommended for most relays)
balanced := RateLimitConfig{
    MinAccountAge:       30 * 24 * time.Hour,  // 30 days
    MinEvents:           20,
    MinFollowers:        2,
    ProposalDelay:       6 * time.Hour,
    CooldownPeriod:      7 * 24 * time.Hour,
    MaxNamesPerAccount:  10,
}

// Permissive (community trust-based)
permissive := RateLimitConfig{
    MinAccountAge:       7 * 24 * time.Hour,   // 7 days
    MinEvents:           5,
    MinFollowers:        0,  // No WoT requirement
    ProposalDelay:       1 * time.Hour,
    CooldownPeriod:      24 * time.Hour,
    MaxNamesPerAccount:  20,
}
```

Each relay can choose their own configuration based on their community values and spam tolerance.

---

## Conclusion

Non-monetary, non-PoW rate limiting is achievable through careful combination of:
1. **Time-based friction** (delays, cooldowns)
2. **Social proof** (WoT, endorsements)
3. **Behavioral signals** (activity history, pattern detection)
4. **Graduated trust** (reward long-term participation)

The key insight is that **time + social capital** can be as effective as monetary deposits for spam prevention, while being more aligned with Nostr's values of openness and decentralization.

The recommended hybrid approach provides strong sybil resistance while maintaining accessibility for legitimate new users, creating a natural barrier that's low for humans but high for bots.
