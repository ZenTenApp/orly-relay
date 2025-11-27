NIP-XX
======

Free Internet Name Daemon (FIND): Decentralized Name Registry with Trust-Weighted Consensus
----------------------------------------------------------

`draft` `optional`

## Abstract

This NIP defines a decentralized name registry protocol for human-readable resource naming with trustless title transfer. The protocol is implemented as an independent service that uses Nostr relays for communication (via ephemeral events) and persistent storage (for name state records). The service uses trust-weighted attestations from registry service operators to achieve Byzantine fault tolerance without requiring centralized coordination or proof-of-work. Consensus is reached through social mechanisms of association and affinity, enabling the network to scale to thousands of participating registry services while maintaining ~51% Byzantine fault tolerance against censorship.

All name registrations last exactly 1 year by network consensus. During the final 30 days before expiration, only the current owner can renew the name (preferential renewal window), preventing last-minute sniping while requiring active maintenance to prevent squatting.

Names follow standard Internet DNS conventions (lowercase, dot-separated hierarchical structure). **TLDs are registerable by anyone** - there is no ICANN equivalent, meaning valuable TLDs like `com`, `org`, `net` will see intense competition in a first-come-first-served race at network launch.

Name owners can publish DNS-style resource records (A, AAAA, CNAME, MX, TXT, NS, SRV) to define what their names resolve to, enabling use cases from IP address mapping to mail servers to service discovery.

To complete the replacement of traditional DNS and TLS infrastructure, the protocol includes a decentralized certificate system using Let's Encrypt-style challenge-response verification and threshold witnessing, combined with a Noise protocol variant (Nostr Noise) for establishing secure authenticated connections without certificate authorities.

## Motivation

Many peer-to-peer applications require human-readable naming systems for locating network resources (analogous to DNS). Traditional approaches either rely on centralized authorities or blockchain-based systems with slow finality times and high resource requirements.

This proposal (Free Internet Name Daemon - FIND) leverages Nostr's existing social graph and relay infrastructure to create a complete naming and security system that replaces both DNS and TLS:

**Name Registry Properties:**
- **Trustless**: No single authority controls name registration
- **Byzantine fault tolerant**: Resistant to malicious registry services (up to ~51% by trust weight)
- **Scalable**: Supports thousands of participating registry services
- **Censorship resistant**: Diverse trust graphs make network-wide censorship difficult
- **Permissionless**: Anyone can operate a registry service and participate in consensus
- **Relay-agnostic**: Uses standard Nostr relays for communication and storage without requiring relay modifications
- **Anti-squatting**: Mandatory 1-year registration period with preferential 30-day renewal window prevents indefinite name holding

**DNS Replacement:**
- **Functionally complete**: DNS-style records (A, AAAA, CNAME, MX, TXT, NS, SRV) enable practical applications
- **Instant updates**: Record changes propagate in seconds via Nostr relays
- **Built-in auth**: Records cryptographically signed by name owners

**TLS/CA Replacement:**
- **Decentralized witnessing**: Threshold signatures (3+ witnesses) replace certificate authorities
- **Challenge-response**: Let's Encrypt-style verification proves name ownership
- **Noise protocol**: Secure authenticated transport using Nostr's secp256k1 primitives
- **Perfect forward secrecy**: Ephemeral key exchanges protect past sessions

The protocol is implemented as an independent service (separate from relay software) that communicates via ephemeral Nostr events and stores persistent state as parameterized replaceable events.

**Key Innovations:**

1. **No Central Naming Authority:** Unlike traditional DNS with ICANN controlling TLDs, this protocol allows anyone to register TLDs. Names follow standard DNS conventions (lowercase, dot-separated) ensuring compatibility, but TLDs are first-come-first-served. Subdomain authority is enforced (must own parent domain), creating a hierarchical ownership model without centralized gatekeepers.

2. **Preferential Renewal Window:** All names expire after exactly 1 year by network consensus. During the final 30 days before expiration, only the current owner can renew the name, preventing last-minute sniping while ensuring names don't remain claimed indefinitely without active maintenance. After expiration, names become available to anyone on a first-come, first-served basis.

3. **DNS-Compatible Records:** Owners publish standard DNS-style resource records (A, AAAA, CNAME, MX, TXT, NS, SRV) as Nostr events, enabling drop-in replacement for traditional DNS in many applications. Per-type limits prevent spam while supporting real-world use cases.

4. **Decentralized Certificates & TLS Replacement:** Challenge-response verification (similar to Let's Encrypt) proves name ownership, threshold witnessing (3+ trusted parties) replaces certificate authorities, and a Noise protocol variant using Nostr's secp256k1 primitives provides secure authenticated transport with perfect forward secrecy.

The protocol achieves registration finality in 1-2 minutes, record updates are instant (limited only by relay propagation), and certificate issuance takes minutes, making it suitable for replacing DNS and TLS infrastructure while avoiding the complexity and resource requirements of traditional blockchain consensus.

## Specification

### Architecture Overview

The name registry protocol is implemented as an independent service that runs separately from Nostr relay software. The service architecture consists of:

**Registry Service:**
- A standalone daemon/process operated by registry service providers
- Subscribes to registration proposals and attestations from Nostr relays
- Computes consensus independently using trust-weighted voting
- Publishes attestations (ephemeral) and name state (persistent) back to relays
- Maintains local trust graph and consensus state

**Communication Layer (Ephemeral Events):**
- **Attestations (kind 20100)**: Service-to-service communication via ephemeral events
- Posted to Nostr relays for propagation to other registry services
- Short-lived (pruned after attestation window completes)
- Enables decentralized coordination without direct peer connections

**Storage Layer (Persistent Events):**
- **Registration Proposals (kind 30100)**: User requests stored on relays
- **Trust Graphs (kind 30101)**: Service trust relationships stored on relays
- **Name State (kind 30102)**: Consensus results stored on relays
- Parameterized replaceable events ensure only current state is maintained

**Relay Role:**
- Standard Nostr relays (no modifications required)
- Store persistent events (kinds 30100, 30101, 30102)
- Propagate ephemeral attestations (kind 20100) between services
- Serve queries from clients and registry services
- No consensus logic or special name registry support needed

**Advantages of Independent Service Architecture:**
- Relays remain simple event stores and routers
- Service operators can upgrade consensus logic independently
- Multiple competing registry implementations can coexist
- Clear separation of concerns (relay = transport/storage, service = consensus)
- Registry services can use multiple relays for redundancy

### Event Kinds

This NIP defines the following event kinds:

- `30100`: Registration Proposal - Claim or transfer of a name (parameterized replaceable)
- `20100`: Attestation - Registry service vote on a registration proposal (ephemeral)
- `30101`: Trust Graph - Service trust relationships with other services (parameterized replaceable)
- `30102`: Name State - Current ownership state (parameterized replaceable)
- `30103`: Name Records - DNS-style resource records published by name owner (parameterized replaceable)
- `30104`: Certificate - Cryptographic certificate for secure connections (parameterized replaceable)
- `30105`: Witness Service - Certificate witness service information (parameterized replaceable)

**Expiration Tags:**

All event kinds in this protocol support expiration tags (NIP-40) which serve dual purposes:

1. **Protocol Boundary**: Defines when an event is no longer valid for protocol operations. Registry services MUST ignore expired events when computing consensus.
2. **Relay Housekeeping**: Hints to relays when events can be deleted to reduce storage overhead. Relays MAY (but are not required to) delete expired events.

Expiration ensures that:
- Ephemeral coordination data (attestations) is automatically pruned
- Trust relationships remain current through required renewal
- Names don't remain claimed indefinitely (anti-squatting)
- Abandoned services/names automatically exit the active network
- Relay storage requirements remain bounded

**Recommended Expiration Periods:**

| Event Kind | Purpose | Network Consensus Expiration | Rationale |
|------------|---------|-------------------------------|-----------|
| 30100 | Registration Proposal | `created_at + 300` (5 min) | Proposals should be processed quickly; unprocessed proposals are stale |
| 20100 | Attestation | `created_at + 180` (3 min) | Attestations only needed during consensus window |
| 30101 | Trust Graph | `created_at + 2592000` (30 days) | Trust relationships should be actively maintained monthly |
| 30102 | Name State | `registered_at + 31536000` (1 year) | Name ownership lasts exactly 1 year by network consensus |
| 30103 | Name Records | Tied to name expiration | Records are valid while name registration is active; expire with name |
| 30104 | Certificate | `valid_until` timestamp | Certificates expire per their validity period (recommended: 90 days) |
| 30105 | Witness Service | `created_at + 15552000` (180 days) | Witness service info should be updated semi-annually |

### Name Format and Syntax

Names in this protocol MUST follow standard Internet domain name conventions to ensure compatibility with existing tools and mental models.

**Name Format Rules:**

1. **Case Normalization**: All names MUST be lowercase ASCII only
   - `Example.Com` → normalized to `example.com`
   - Names are case-insensitive for registration but stored in lowercase
   - Uppercase letters in proposals are automatically converted to lowercase

2. **Character Set**: Names MUST only contain:
   - Lowercase letters: `a-z`
   - Digits: `0-9`
   - Hyphens: `-` (not at start or end of labels)
   - Dots: `.` (label separator)

3. **Label Structure**: Names are composed of dot-separated labels
   - Each label: 1-63 characters
   - Total name: maximum 253 characters
   - Labels cannot start or end with hyphens
   - Labels cannot be all numeric

4. **Hierarchical Structure**: Standard DNS hierarchy applies
   - `com` - top-level domain (TLD)
   - `example.com` - second-level domain under `com`
   - `www.example.com` - third-level domain (subdomain) under `example.com`
   - `api.services.example.com` - fourth-level domain

**Top-Level Domains (TLDs):**

In this protocol, **TLDs are just names like any other** and can be registered by anyone. There is no reserved set of TLDs (no ICANN equivalent).

```json
// Anyone can register a TLD
{"kind": 30100, "tags": [["d", "com"], ["action", "register"]]}
{"kind": 30100, "tags": [["d", "nostr"], ["action", "register"]]}
{"kind": 30100, "tags": [["d", "bitcoin"], ["action", "register"]]}
```

**TLD Registration Race:**

Since TLDs are registerable by anyone, valuable/generic TLDs will likely see intense competition:
- Popular extensions: `com`, `org`, `net`, `io`, `dev`, etc.
- Geographic: `us`, `uk`, `jp`, `eu`, etc.
- Generic terms: `shop`, `mail`, `web`, `app`, etc.
- Cryptocurrency: `bitcoin`, `nostr`, `lightning`, etc.

The first proposal to achieve consensus wins the TLD. Expect significant competition for short, memorable TLDs in the early network.

**Subdomain Authority:**

Only the owner of a domain can register its direct subdomains:
- Owner of `com` can register `example.com`, `google.com`, etc.
- Owner of `example.com` can register `www.example.com`, `api.example.com`, etc.
- Owner of `example.com` CANNOT register `other.com` (different parent)

This is enforced by the protocol: Registry services MUST verify that the registrant of `sub.parent.tld` also owns `parent.tld`.

**Examples:**

Valid names:
```
com
example.com
www.example.com
api-v2.services.example.com
my-site.nostr
user123.bitcoin
```

Invalid names:
```
Example.Com          # Not lowercase (will be normalized)
exam ple.com         # Contains space
-example.com         # Label starts with hyphen
example-.com         # Label ends with hyphen
exam_ple.com         # Contains underscore
123.456              # All-numeric labels
a.b.c.d.e.f.g.h.i.j.k.l.m.n.o.p.q.r.s.t.u.v.w.x.y.z.a.b.c.d.e.f  # > 253 chars
```

**Name Validation:**

Registry services and clients MUST validate names against these rules:
1. Convert to lowercase (normalization)
2. Check character set (a-z, 0-9, hyphen, dot only)
3. Validate label lengths (1-63 chars per label)
4. Validate total length (≤253 chars)
5. Check hyphen placement (not at label start/end)
6. Verify label is not all-numeric
7. For subdomains: Verify parent domain ownership

Invalid names MUST be rejected before attestation.

### Registration Proposal (Kind 30100)

A parameterized replaceable event where users propose to register or transfer a name:

```json
{
  "kind": 30100,
  "pubkey": "<claimant_pubkey>",
  "created_at": <unix_timestamp>,
  "tags": [
    ["d", "<name>"],              // name being claimed (e.g., "foo.n")
    ["action", "register"],        // "register" or "transfer"
    ["prev_owner", "<pubkey>"],    // previous owner pubkey (for transfers only)
    ["prev_sig", "<signature>"],   // signature from prev_owner authorizing transfer
    ["expiration", "<unix_timestamp>"]  // proposal expires if not processed
  ],
  "content": "",
  "sig": "<signature>"
}
```

**Field Specifications:**

- `d` tag: The name being registered. MUST be lowercase, follow DNS naming rules (see Name Format section), and be unique within the namespace.
- `action` tag: Either `register` (initial claim) or `transfer` (ownership change)
- `prev_owner` tag: Required for `transfer` actions. The pubkey of the current owner.
- `prev_sig` tag: Required for `transfer` actions. Signature proving authorization from previous owner.
- `expiration` tag: Optional Unix timestamp when this proposal expires. Registry services MUST ignore proposals after expiration. Relays MAY delete expired proposals for housekeeping. Recommended: `created_at + 300` (5 minutes).

**Subdomain Registration:**

To register a subdomain (e.g., `www.example.com`), the proposer MUST own the parent domain (`example.com`). Registry services validate this during proposal processing:

1. Proposer owns `com` TLD
2. Proposer registers `example.com` (requires owning `com`)
3. Proposer registers `www.example.com` (requires owning `example.com`)

This hierarchical ownership ensures that only domain owners can create subdomains, preventing unauthorized subdomain squatting.

**Transfer Authorization:**

For transfers, the `prev_sig` MUST be a signature over the following message:
```
transfer:<name>:<new_owner_pubkey>:<timestamp>
```

This prevents unauthorized transfers and ensures the current owner explicitly approves the transfer.

### Attestation (Kind 20100)

An ephemeral event where registry service operators attest to their acceptance or rejection of a registration proposal. This event is published to Nostr relays for propagation to other registry services:

```json
{
  "kind": 20100,
  "pubkey": "<registry_service_pubkey>",
  "created_at": <unix_timestamp>,
  "tags": [
    ["e", "<proposal_event_id>"],           // the registration proposal being attested
    ["decision", "approve"],                 // "approve", "reject", or "abstain"
    ["weight", "100"],                       // optional stake/confidence weight
    ["reason", "first_valid"],               // optional audit trail
    ["service", "wss://registry.example.com"], // optional: the registry service endpoint
    ["expiration", "<unix_timestamp>"]       // attestation expires after consensus window
  ],
  "content": "",
  "sig": "<signature>"
}
```

**Field Specifications:**

- `e` tag: References the registration proposal event ID
- `decision` tag: One of:
  - `approve`: Registry service accepts this registration as valid
  - `reject`: Registry service rejects this registration (e.g., conflict detected)
  - `abstain`: Registry service acknowledges but doesn't vote
- `weight` tag: Optional numeric weight (default: 100). Higher weights indicate stronger confidence or stake.
- `reason` tag: Optional human-readable justification for audit trails
- `service` tag: Optional identifier for the registry service (URL or other identifier)
- `expiration` tag: Unix timestamp when this attestation expires. Registry services MUST ignore attestations after expiration. Relays SHOULD delete expired attestations for housekeeping. Recommended: `created_at + 180` (3 minutes after attestation, allowing for consensus window completion).

**Attestation Window:**

Registry services SHOULD publish attestations within 60-120 seconds of receiving a registration proposal. Attestations are posted to Nostr relays as ephemeral events, allowing them to propagate to other registry services via relay gossip. This allows sufficient time for event propagation while maintaining reasonable finality times.

**Expiration and Cleanup:**

Attestations include an expiration tag that serves two purposes:
1. **Protocol validity**: Registry services MUST NOT count expired attestations in consensus calculations
2. **Relay housekeeping**: Relays SHOULD delete expired attestations to reduce storage overhead

Once consensus is reached and name state (kind 30102) is published, attestations are no longer needed and relays can safely prune them.

### Trust Graph (Kind 30101)

A parameterized replaceable event where registry service operators declare their trust relationships. This event is stored on Nostr relays and used by other registry services to build trust graphs:

```json
{
  "kind": 30101,
  "pubkey": "<registry_service_pubkey>",
  "created_at": <unix_timestamp>,
  "tags": [
    ["d", "trust-graph"],  // identifier for replacement
    ["p", "<trusted_service1_pubkey>", "wss://service1.example.com", "0.9"],
    ["p", "<trusted_service2_pubkey>", "wss://service2.example.com", "0.7"],
    ["p", "<trusted_service3_pubkey>", "wss://service3.example.com", "0.5"],
    ["expiration", "<unix_timestamp>"]  // trust graph expires and requires renewal
  ],
  "content": "",
  "sig": "<signature>"
}
```

**Field Specifications:**

- `p` tag: Defines trust in another registry service operator
  - First parameter: Trusted registry service's pubkey
  - Second parameter: Optional service identifier (URL or other identifier)
  - Third parameter: Trust score (0.0 to 1.0, where 1.0 = complete trust)
- `expiration` tag: Unix timestamp when this trust graph expires. Registry services MUST NOT use expired trust graphs in consensus calculations. Requires periodic renewal to maintain active trust relationships. Recommended: `created_at + 2592000` (30 days).

**Trust Score Guidelines:**

- `1.0`: Complete trust (e.g., operator's own other registry services)
- `0.7-0.9`: High trust (well-known, reputable service operators)
- `0.4-0.6`: Moderate trust (established but less familiar operators)
- `0.1-0.3`: Low trust (new or unknown operators)
- `0.0`: No trust (excluded from consensus)

**Trust Graph Updates and Renewal:**

Registry service operators SHOULD update their trust graphs gradually. Rapid changes to trust relationships MAY be penalized in weighted consensus calculations to prevent manipulation. Trust graphs are stored as parameterized replaceable events on Nostr relays, making them publicly auditable.

Services MUST publish renewed trust graphs before expiration to maintain their trust relationships. An expired trust graph is treated as if the service has no trust relationships, effectively removing it from the consensus network until a new trust graph is published. This ensures that:
1. Only active, maintained services participate in consensus
2. Abandoned services automatically drop out of the network
3. Trust relationships remain current and intentional
4. Relays can optionally prune very old expired trust graphs

### Name State (Kind 30102)

A parameterized replaceable event representing the current ownership state of a name. This event is published by registry services after consensus is reached and stored on Nostr relays:

```json
{
  "kind": 30102,
  "pubkey": "<registry_service_pubkey>",
  "created_at": <unix_timestamp>,
  "tags": [
    ["d", "<name>"],                    // the name
    ["owner", "<current_owner_pubkey>"],
    ["registered_at", "<timestamp>"],
    ["proposal", "<proposal_event_id>"],
    ["attestations", "<count>"],
    ["confidence", "0.87"],             // consensus confidence score
    ["expiration", "<unix_timestamp>"]  // name registration expires and requires renewal
  ],
  "content": "<optional_metadata_json>",
  "sig": "<signature>"
}
```

**Field Specifications:**

- `d` tag: The name being described
- `owner` tag: Current owner's pubkey
- `registered_at` tag: Unix timestamp when originally registered (or last renewed)
- `proposal` tag: Event ID of the registration proposal that established current ownership
- `attestations` tag: Number of services that attested to this registration
- `confidence` tag: Consensus confidence score (0.0 to 1.0)
- `expiration` tag: Unix timestamp when this name registration expires. By network consensus, MUST be exactly `registered_at + 31536000` (1 year). Owners can renew during the preferential renewal window (final 30 days). After expiration, anyone can register.

This event is published by registry services after consensus is reached and stored on Nostr relays as a parameterized replaceable event. This allows clients to quickly query current ownership state from relays without recomputing consensus or running their own registry service.

**Name Expiration and Renewal:**

All name registrations last exactly **1 year** by network consensus. This is not configurable - registry services MUST enforce this standard registration period.

**Registration Lifecycle:**

```
T=0:         Name registered (registered_at timestamp)
T=0-335d:    Active registration, owner has exclusive control
T=335-365d:  Preferential renewal window (final 30 days)
T=365d:      Expiration - name becomes available to anyone
```

**Preferential Renewal Window (Days 335-365):**

During the final 30 days before expiration (starting at day 335, approximately 11 months), ONLY the current owner can successfully register the name. Registry services MUST reject registration proposals from other pubkeys during this window. This gives owners time to renew while preventing last-minute sniping.

**After Expiration:**

After the expiration timestamp passes, the name becomes available for registration by anyone. The original owner has no special claim and must compete with other claimants through the standard consensus process.

The expiration mechanism serves multiple purposes:
1. **Anti-squatting**: Prevents indefinite holding of unused names
2. **Fair renewal**: Owners get 30 days exclusive renewal window
3. **Protocol boundary**: Clear definition of when ownership lapses
4. **Relay housekeeping**: Relays MAY delete expired name state events

### Name Records (Kind 30103)

Once a name is successfully registered, the owner can publish DNS-style resource records to define what the name resolves to. These records are parameterized replaceable events published by the name owner and stored on Nostr relays.

```json
{
  "kind": 30103,
  "pubkey": "<name_owner_pubkey>",
  "created_at": <unix_timestamp>,
  "tags": [
    ["d", "<name>:<record_type>"],     // e.g., "foo.n:A" or "foo.n:AAAA"
    ["name", "<name>"],                 // the registered name
    ["type", "<record_type>"],          // A, AAAA, CNAME, MX, TXT, NS, SRV
    ["value", "<record_value>"],        // IP address, hostname, text, etc.
    ["ttl", "3600"],                    // cache TTL in seconds
    ["priority", "10"],                 // for MX and SRV records (optional)
    ["weight", "50"],                   // for SRV records (optional)
    ["port", "443"]                     // for SRV records (optional)
  ],
  "content": "",
  "sig": "<signature>"
}
```

**Field Specifications:**

- `d` tag: Unique identifier combining name and record type (e.g., "example.n:A", "example.n:MX:1")
- `name` tag: The registered name these records belong to
- `type` tag: DNS record type (see below)
- `value` tag: The record value (format depends on type)
- `ttl` tag: Time-to-live in seconds for client caching (recommended: 3600 = 1 hour)
- `priority` tag: For MX and SRV records, lower numbers = higher priority (0-65535)
- `weight` tag: For SRV records, relative weight for load balancing (0-65535)
- `port` tag: For SRV records, the service port number (0-65535)

**Authorization:**

Only the current owner of a name (as determined by kind 30102 name state events) can publish valid records for that name. Clients and services MUST verify that:
1. The name is currently registered (valid, non-expired kind 30102 exists)
2. `record.pubkey == name_state.owner`
3. If either check fails, ignore the record

**Supported Record Types:**

| Type | Purpose | Value Format | Limit per Name |
|------|---------|--------------|----------------|
| **A** | IPv4 address | Dotted decimal (e.g., "192.0.2.1") | Max 5 records |
| **AAAA** | IPv6 address | Colon-separated hex (e.g., "2001:db8::1") | Max 5 records |
| **CNAME** | Canonical name (alias) | Fully qualified domain/name (e.g., "target.n") | Max 1 record |
| **MX** | Mail exchange server | Hostname (e.g., "mail.example.n") + priority tag | Max 5 records |
| **TXT** | Text record | Any text string (max 1024 chars) | Max 10 records |
| **NS** | Name server (delegation) | Hostname of authoritative name server | Max 5 records |
| **SRV** | Service location | Hostname + priority, weight, port tags | Max 10 records |
| **GIT** | Git repository location | Repository URL with protocol + optional metadata | Max 10 records |

**Record Type Details:**

**A Record (IPv4 Address):**
```json
{
  "tags": [
    ["d", "example.n:A:1"],
    ["name", "example.n"],
    ["type", "A"],
    ["value", "192.0.2.1"],
    ["ttl", "3600"]
  ]
}
```
Maps name to IPv4 address. Multiple A records enable round-robin load balancing.

**AAAA Record (IPv6 Address):**
```json
{
  "tags": [
    ["d", "example.n:AAAA:1"],
    ["name", "example.n"],
    ["type", "AAAA"],
    ["value", "2001:db8::1"],
    ["ttl", "3600"]
  ]
}
```
Maps name to IPv6 address.

**CNAME Record (Canonical Name):**
```json
{
  "tags": [
    ["d", "example.n:CNAME"],
    ["name", "example.n"],
    ["type", "CNAME"],
    ["value", "target.n"],
    ["ttl", "3600"]
  ]
}
```
Creates an alias pointing to another name. If CNAME exists, A/AAAA records SHOULD NOT exist for the same name (CNAME takes precedence).

**MX Record (Mail Exchange):**
```json
{
  "tags": [
    ["d", "example.n:MX:1"],
    ["name", "example.n"],
    ["type", "MX"],
    ["value", "mail.example.n"],
    ["priority", "10"],
    ["ttl", "3600"]
  ]
}
```
Specifies mail servers for the domain. Lower priority numbers = higher preference.

**TXT Record (Text):**
```json
{
  "tags": [
    ["d", "example.n:TXT:1"],
    ["name", "example.n"],
    ["type", "TXT"],
    ["value", "v=spf1 include:example.n ~all"],
    ["ttl", "3600"]
  ]
}
```
Arbitrary text data. Commonly used for verification (SPF, DKIM, domain ownership), configuration, or metadata. Max 1024 characters per record.

**NS Record (Name Server):**
```json
{
  "tags": [
    ["d", "example.n:NS:1"],
    ["name", "example.n"],
    ["type", "NS"],
    ["value", "ns1.example.n"],
    ["ttl", "3600"]
  ]
}
```
Delegates subdomain authority to specific name servers. Used for distributed name management.

**SRV Record (Service):**
```json
{
  "tags": [
    ["d", "example.n:SRV:1"],
    ["name", "_http._tcp.example.n"],
    ["type", "SRV"],
    ["value", "server.example.n"],
    ["priority", "10"],
    ["weight", "50"],
    ["port", "443"],
    ["ttl", "3600"]
  ]
}
```
Specifies service location (host and port). Name format: `_service._proto.name`. Priority determines preference order, weight enables load distribution.

**Record Limits and Validation:**

Registry services and clients SHOULD enforce the following limits:

1. **Per-type limits**: Maximum records per type as specified in table above
2. **Total records**: Maximum 50 records per name across all types
3. **Value validation**:
   - A: Valid IPv4 address format
   - AAAA: Valid IPv6 address format
   - CNAME: Valid name format, no circular references
   - MX/NS/SRV: Valid hostname format
   - TXT: Max 1024 characters
   - GIT: Valid git-compatible URL (git://, https://, ssh://, file://)
   - Priority/weight/port: Valid integer ranges (0-65535)
4. **CNAME exclusivity**: If CNAME record exists, A/AAAA records MUST NOT exist for the same name
5. **Owner authorization**: Record pubkey MUST match current name owner
6. **GIT record validation**:
   - Protocol tag matches URL scheme in value tag
   - Description max 256 characters
   - Access level is "public", "private", or "restricted" (if specified)
   - Mirror flag is "true" or absent
   - SSH/HTTPS URLs are valid if provided

**Record Expiration:**

Name records do not have separate expiration. They are implicitly valid while the name registration is active. When a name expires (after 1 year without renewal):
- All associated records become invalid
- Clients MUST stop resolving records for expired names
- Relays MAY prune expired name records for housekeeping
- New owner must publish fresh records after re-registering

**GIT Record (Git Repository Location):**
```json
{
  "tags": [
    ["d", "example.n:GIT:1"],
    ["name", "example.n"],
    ["type", "GIT"],
    ["value", "git://git.example.n/repo.git"],
    ["ttl", "3600"],
    ["protocol", "git"],
    ["clone_url", "https://git.example.n/repo.git"],
    ["ssh_url", "ssh://git@git.example.n:repo.git"],
    ["description", "My awesome project"],
    ["default_branch", "main"]
  ]
}
```
Maps name to git repository location with access metadata. Supports multiple protocols (git://, https://, ssh://). Multiple GIT records can provide different repository locations (mirrors, forks).

**GIT Record Field Specifications:**

- `value` tag: Primary repository URL (any git-compatible protocol)
- `protocol` tag: Primary protocol (git, https, ssh, file)
- `clone_url` tag: Optional HTTPS clone URL for web-based access
- `ssh_url` tag: Optional SSH URL for authenticated access
- `description` tag: Optional repository description (max 256 chars)
- `default_branch` tag: Optional default branch name (default: "main")
- `access` tag: Optional access level (public, private, restricted)
- `mirror` tag: Optional flag indicating this is a mirror ("true")

**GIT Record Limit:** Max 10 records per name (supporting multiple mirrors and access methods)

**GIT Record Usage Examples:**

**Example 1: Public Repository with Multiple Access Methods**
```json
{
  "kind": 30103,
  "pubkey": "<name_owner_pubkey>",
  "tags": [
    ["d", "myproject.n:GIT:1"],
    ["name", "myproject.n"],
    ["type", "GIT"],
    ["value", "https://git.myproject.n/myproject.git"],
    ["ttl", "3600"],
    ["protocol", "https"],
    ["clone_url", "https://git.myproject.n/myproject.git"],
    ["ssh_url", "git@git.myproject.n:myproject.git"],
    ["description", "Decentralized social network built on Nostr"],
    ["default_branch", "main"],
    ["access", "public"]
  ]
}
```

**Example 2: Repository with Multiple Mirrors**
```json
{
  "kind": 30103,
  "pubkey": "<name_owner_pubkey>",
  "tags": [
    ["d", "bitcoin.n:GIT:1"],
    ["name", "bitcoin.n"],
    ["type", "GIT"],
    ["value", "https://git.bitcoin.n/bitcoin.git"],
    ["protocol", "https"],
    ["description", "Bitcoin Core reference implementation"],
    ["default_branch", "master"]
  ]
}
```

```json
{
  "kind": 30103,
  "pubkey": "<name_owner_pubkey>",
  "tags": [
    ["d", "bitcoin.n:GIT:2"],
    ["name", "bitcoin.n"],
    ["type", "GIT"],
    ["value", "https://mirror1.bitcoin.n/bitcoin.git"],
    ["protocol", "https"],
    ["mirror", "true"],
    ["description", "Bitcoin Core reference implementation (Mirror 1)"]
  ]
}
```

```json
{
  "kind": 30103,
  "pubkey": "<name_owner_pubkey>",
  "tags": [
    ["d", "bitcoin.n:GIT:3"],
    ["name", "bitcoin.n"],
    ["type", "GIT"],
    ["value", "git://mirror2.bitcoin.n/bitcoin.git"],
    ["protocol", "git"],
    ["mirror", "true"],
    ["description", "Bitcoin Core reference implementation (Mirror 2)"]
  ]
}
```

**Example 3: Subdomain for Repository Organization**
```json
{
  "kind": 30103,
  "tags": [
    ["d", "orly.mleku.dev:GIT:1"],
    ["name", "orly.mleku.dev"],
    ["type", "GIT"],
    ["value", "https://git.mleku.dev/mleku/orly.git"],
    ["protocol", "https"],
    ["clone_url", "https://git.mleku.dev/mleku/orly.git"],
    ["ssh_url", "git@git.mleku.dev:mleku/orly.git"],
    ["description", "ORLY - High-performance Nostr relay in Go"],
    ["default_branch", "main"],
    ["access", "public"]
  ]
}
```

**Example 4: Private Repository with SSH-Only Access**
```json
{
  "kind": 30103,
  "tags": [
    ["d", "private.company.n:GIT:1"],
    ["name", "private.company.n"],
    ["type", "GIT"],
    ["value", "ssh://git@git.company.n:internal/project.git"],
    ["protocol", "ssh"],
    ["ssh_url", "git@git.company.n:internal/project.git"],
    ["description", "Internal company project (authorized access only)"],
    ["default_branch", "develop"],
    ["access", "private"]
  ]
}
```

**GIT Record Client Resolution:**

When resolving a git repository name, clients should:

1. **Verify name ownership:**
   - Query kind 30102 for name state
   - Check name is not expired
   - Note the owner pubkey

2. **Query GIT records:**
   - Subscribe to kind 30103 events with `name` and `type=GIT` tags
   - Filter to only records where `record.pubkey == name_state.owner`

3. **Select repository URL:**
   - Primary: Use `value` tag as main repository URL
   - HTTPS fallback: Use `clone_url` if available
   - SSH access: Use `ssh_url` for authenticated operations
   - Mirror selection: If multiple records exist with `mirror=true`, randomly select or prefer by latency

4. **Display metadata:**
   - Show `description` to users
   - Indicate `access` level (public/private/restricted)
   - Note if repository is a mirror

5. **Clone operation:**
   ```bash
   # Automatic resolution via client tool
   git clone myproject.n

   # Explicit protocol selection
   git clone https://myproject.n
   git clone git@myproject.n
   ```

**Integration with Existing Git Tools:**

Git clients can be extended to resolve names via a custom URL handler or Git credential helper:

```bash
# Example: Custom git-remote-n helper
# Resolves myproject.n by querying Nostr relays
git clone n://myproject.n

# Or via DNS-style integration (if local resolver configured)
git clone https://myproject.n/repo.git
```

**GIT Record Validation:**

Registry services and clients SHOULD validate GIT records:

1. **URL format**: Valid git-compatible URL (git://, https://, ssh://, file://)
2. **Protocol consistency**: `protocol` tag matches URL scheme in `value`
3. **Description length**: Max 256 characters
4. **Access level**: One of "public", "private", "restricted" (if specified)
5. **Mirror flag**: Must be "true" or absent
6. **SSH URL format**: Valid SSH URL if `ssh_url` provided
7. **HTTPS URL format**: Valid HTTPS URL if `clone_url` provided
8. **Per-name limit**: Maximum 10 GIT records per name
9. **Owner authorization**: Record pubkey matches current name owner

**Subdomain Delegation:**

Subdomains can be delegated using NS records:

```json
{
  "tags": [
    ["d", "example.n:NS:1"],
    ["name", "sub.example.n"],
    ["type", "NS"],
    ["value", "ns1.subdomain-operator.n"]
  ]
}
```

The owner of `example.n` publishes NS records for `sub.example.n`, delegating authority to another operator's name server. The subdomain operator can then publish records for `*.sub.example.n` (if their system supports it).

**Client Resolution Process:**

1. Query relays for kind 30102 name state to verify ownership and expiration
2. If expired, return NXDOMAIN (name does not exist)
3. Query relays for kind 30103 records matching the name
4. Filter records by pubkey (MUST match name owner)
5. Apply record limits (reject excess records)
6. Check for CNAME - if exists, follow to target name (with loop detection)
7. Return A/AAAA records (or other requested type)
8. Cache result according to TTL

### Name Renewal Process

Owners can renew their name registration during the preferential renewal window (final 30 days before expiration) by submitting a new registration proposal (kind 30100) with `action: "register"` for the same name.

**Preferential Renewal Window Enforcement:**

Registry services MUST enforce the following rules:

1. **Before Renewal Window** (Days 0-335):
   - Name is owned and cannot be registered by anyone
   - Registry services MUST reject all registration proposals for this name

2. **During Renewal Window** (Days 335-365):
   - ONLY the current owner's pubkey can successfully register the name
   - Registry services MUST reject registration proposals from other pubkeys
   - Owner's renewal proposals follow normal consensus process

3. **After Expiration** (Day 365+):
   - Name becomes available to anyone
   - First valid registration to achieve consensus wins
   - Former owner has no special privileges

**Renewal Process:**

1. **Renewal Proposal**: Owner publishes a kind 30100 event with:
   - `action` tag: `"register"` (renewals use the register action)
   - `d` tag: The name being renewed
   - `expiration` tag: Proposal expiration (5 minutes)
   - Must be published during the renewal window (days 335-365)

2. **Validation**: Registry services validate:
   - Current time is within renewal window (`expiration - 2592000` to `expiration`)
   - Proposal pubkey matches current owner pubkey from name state
   - If validation fails, reject the proposal

3. **Consensus**: Registry services attest to the renewal following normal consensus rules.

4. **New Name State**: After consensus, services publish updated kind 30102 events with:
   - Updated `registered_at` timestamp (time of renewal consensus)
   - New `expiration` tag: `registered_at + 31536000` (exactly 1 year)
   - Same `owner` pubkey
   - New `proposal` event ID pointing to renewal proposal

**Renewal Timing Recommendations:**
- Owners SHOULD renew as soon as the renewal window opens (day 335)
- Clients MUST warn owners when renewal window opens (30 days until expiration)
- Early renewal reduces risk of missing the deadline
- If renewal consensus completes before expiration, ownership continues uninterrupted
- If owner misses the renewal window, they lose preferential status after expiration

### Consensus Algorithm

Each registry service independently computes consensus using the following algorithm. Registry services subscribe to events from Nostr relays to receive proposals and attestations:

#### 1. Proposal Validation

When a registry service receives a registration proposal for name `N` (via subscription to kind 30100 events on relays):

1. **Check proposal expiration** - if expired, ignore the proposal

2. **Validate name format:**
   - Convert name to lowercase (normalization)
   - Verify character set (a-z, 0-9, hyphen, dot only)
   - Validate label lengths (1-63 chars per label)
   - Validate total length (≤253 chars)
   - Check hyphen placement (not at label start/end)
   - Verify labels are not all-numeric
   - REJECT if validation fails

3. **Verify subdomain authority** (if name contains dots):
   - Extract parent domain (e.g., `www.example.com` → parent is `example.com`)
   - Query kind 30102 for parent domain name state
   - Verify `proposal.pubkey == parent_name_state.owner`
   - REJECT if proposer doesn't own parent domain
   - Note: TLDs (single labels like `com`) have no parent, always allowed

4. **Query current name state** - fetch kind 30102 events for name `N` from relays

5. **Validate against name state:**
   - If name is NOT registered (no valid name state):
     - Accept proposal from any pubkey (or from parent owner for subdomains)
   - If name IS registered and NOT expired:
     - Calculate: `current_time` vs `name_state.expiration`
     - If `current_time < (expiration - 2592000)` (before renewal window):
       - REJECT: Name is owned, not available
     - If `current_time >= (expiration - 2592000)` AND `current_time < expiration` (renewal window):
       - ACCEPT only if `proposal.pubkey == name_state.owner`
       - REJECT if `proposal.pubkey != name_state.owner`
     - If `current_time >= expiration` (after expiration):
       - ACCEPT proposal from any pubkey (or from parent owner for subdomains)

6. **Start attestation timer** for `T` seconds (recommended: 60-120s)

7. **Collect attestations** (kind 20100 events from relays), filtering out expired ones

8. **Collect competing proposals** for the same name `N`, validating each against the rules above

#### 2. Trust-Weighted Scoring

For each competing proposal `P`, compute a weighted score:

```
score(P) = Σ (attestation_weight × trust_decay)
```

Where:
- `attestation_weight`: The weight from the attestation's `weight` tag (default: 100)
- `trust_decay`: Distance-based trust decay from this relay's trust graph:
  - **Direct trust** (0-hop): `trust_score × 1.0`
  - **1-hop trust**: `trust_score × 0.8`
  - **2-hop trust**: `trust_score × 0.6`
  - **3-hop trust**: `trust_score × 0.4`
  - **4+ hops**: `0.0` (not counted)

#### 3. Trust Distance Calculation

Trust distance is computed using the registry service's trust graph (assembled from kind 30101 events retrieved from relays):

1. Subscribe to kind 30101 events from relays to build a trust graph
2. Filter out expired trust graphs based on expiration tag - expired trust graphs are not used
3. Build a directed graph from all valid (non-expired) registry service trust declarations
4. Use Dijkstra's algorithm or breadth-first search to find shortest trust path
5. Multiply trust scores along the path for effective trust weight

**Note:** Services with expired trust graphs are excluded from consensus calculations as if they published no trust relationships. This ensures only actively maintained services participate in the network.

**Example:**
```
Service A → Service B (0.9) → Service C (0.8)
Effective trust from A to C: 0.9 × 0.8 × 0.8 (1-hop decay) = 0.576
```

#### 4. Consensus Decision

After the attestation window expires:

1. **Compute total trust weight**: Sum of all attestation scores across all proposals
2. **Compute proposal score**: Each proposal's weighted attestation sum
3. **Accept proposal `P` if:**
   - `score(P) / total_trust_weight > threshold` (recommended: 0.51)
   - No competing proposal has higher score
   - Valid transfer authorization (if action is "transfer")

4. **Defer decision if:**
   - No proposal reaches threshold
   - Multiple proposals exceed threshold with similar scores (within 5%)
   - Insufficient attestations received (coverage < 30% of trust graph)

5. **Publish name state** (kind 30102) to Nostr relays once consensus is reached

### Sparse Attestation Optimization

To reduce network load, registry services MAY use sparse attestation where they only publish attestations when:

1. **Local interest**: One of the service's clients queries this name
2. **Duty rotation**: `hash(proposal_event_id || service_pubkey) % K == 0` (for sampling rate 1/K)
3. **Conflict detected**: Multiple competing proposals received for the same name
4. **Direct request**: Client explicitly requests attestation

**Recommended sampling rates:**
- Networks with <100 services: K=1 (100% attestation)
- Networks with 100-1000 services: K=10 (10% attestation)
- Networks with >1000 services: K=20 (5% attestation)

This optimization reduces attestation volume by 80-95% while maintaining consensus reliability through randomized duty rotation.

### Handling Conflicts

#### Simultaneous Registration

When multiple proposals for the same name arrive at similar timestamps:

1. **Primary resolution**: Weighted consensus (highest score wins)
2. **Tiebreaker**: If scores are within 5%, use lexicographic ordering of proposal event IDs
3. **Client notification**: Registry services SHOULD publish notices about conflicts for transparency

#### Competing Transfers

When multiple transfer proposals claim to originate from the same owner:

1. Verify `prev_sig` authenticity for each proposal
2. Accept the earliest valid transfer (by `created_at`)
3. If timestamps are identical (within 1 second), use event ID tiebreaker

#### Trust Graph Divergence

Different registry services may reach different consensus due to divergent trust graphs. This is acceptable and provides censorship resistance:

- Clients SHOULD query multiple registry services (recommended: 3-5) by fetching kind 30102 events from relays
- Use majority consensus among responses
- Display uncertainty warnings if services disagree (similar to SSL certificate warnings)

### Finality Timeline

Expected timeline for name registration:

```
T=0s:      Registration proposal published to relays (kind 30100)
T=0-30s:   Proposal propagates via relay gossip to registry services
T=30-90s:  Registry services validate proposal (ownership, renewal window) and publish attestations (kind 20100)
T=90s:     Most registry services compute weighted consensus
T=120s:    High confidence threshold (2-minute finality)
T=120s+:   Registry services publish name state (kind 30102) with expiration = registered_at + 31536000
```

**Registration Lifecycle Timeline:**

```
T=0:                Initial registration achieves consensus
T=0-28944000s:     Name is owned and active (days 0-335, ~11 months)
T=28944000s:       Preferential renewal window opens (day 335)
T=28944000-31536000s: Renewal window active - only owner can register (days 335-365)
T=31536000s:       Name expires - available to anyone (day 365, exactly 1 year)
```

**Early finality**: If >70% of expected attestations arrive within 30s and reach consensus threshold, registry services MAY finalize earlier.

## Implementation Guidelines

### Client Implementation

**Name Ownership Queries:**

Clients querying name ownership should:

1. Subscribe to kind 30102 events for the name from multiple relays

2. Check expiration tag on each name state event - treat expired names as unregistered

3. Use majority consensus among responses from different registry services

4. Warn users if registry services disagree on ownership

5. **Display renewal window status** for owned names:
   - Calculate days until expiration: `(expiration - current_time) / 86400`
   - If days remaining > 35: Show "Active until [date]"
   - If days remaining 30-35: Show "Renewal window opens soon on [date]"
   - If days remaining 0-30: Show **"Renewal window active - renew now!"** (prominent warning)
   - If days remaining < 7: Show **"URGENT: Name expires in X days!"** (critical warning)
   - If expired: Show "Expired - available for registration"

6. For names owned by user's pubkey:
   - Track expiration dates in wallet/profile
   - Send notifications when renewal window opens (day 335)
   - Send urgent notifications if approaching expiration (days 364-365)
   - Provide easy renewal button/workflow in UI

7. Cache results with TTL of 5-10 minutes

8. Re-query on cache expiry or when name is used in critical operations

Note: Clients do not need to run registry service software; they simply query kind 30102 events from standard Nostr relays. Clients MUST respect expiration tags and treat expired names as available for registration.

**Name Record Resolution:**

Clients resolving names to addresses/resources should:

1. **Verify name ownership first:**
   - Query kind 30102 for name state from multiple relays
   - Check name is not expired
   - Note the owner pubkey

2. **Query records:**
   - Subscribe to kind 30103 events with `name` tag matching the target name
   - Filter to only records where `record.pubkey == name_state.owner`
   - Apply record type filters based on query type (A, AAAA, MX, etc.)

3. **Validate records:**
   - Check record count limits per type
   - Validate value formats (IP address syntax, hostname format, etc.)
   - For CNAME: Check for circular references (max 10 hops)
   - Reject records exceeding limits or with invalid formats

4. **Handle CNAME:**
   - If CNAME record exists, recursively resolve the target name
   - Track visited names to detect loops
   - Stop after 10 CNAME hops and return error

5. **Sort and prioritize:**
   - MX/SRV records: Sort by priority (ascending), then weight (descending)
   - A/AAAA records: Randomize order for round-robin load balancing
   - Return all valid records of the requested type

6. **Cache results:**
   - Use TTL from record tags (default: 3600 seconds)
   - Cache negative results (NXDOMAIN) for short period (300 seconds)
   - Invalidate cache if name ownership changes

7. **Handle errors:**
   - Name not registered: Return NXDOMAIN
   - Name expired: Return NXDOMAIN
   - No records of requested type: Return NODATA
   - Record owner mismatch: Ignore invalid records

**Example Resolution Flow:**

```
User queries: example.n → A record

1. Query kind 30102 for "example.n"
   → Found, owner=npub1abc..., expires=2026-01-15, not expired ✓

2. Query kind 30103 with name="example.n", type="A"
   → Found 2 records from npub1abc... (matches owner ✓)
   → Record 1: 192.0.2.1
   → Record 2: 192.0.2.2

3. Return both A records to application
4. Cache for 3600 seconds
```

### Registry Service Implementation

Registry services participating in consensus MUST:

1. Subscribe to kind 30100, 20100, 30101, and 30102 events from configured Nostr relays

2. Maintain local trust graph (derived from kind 30101 events fetched from relays)

3. **Enforce renewal window rules** (critical for network consensus):
   - Query kind 30102 name state for each registration proposal
   - Calculate current time vs name expiration
   - Before renewal window (days 0-335): REJECT all registration proposals
   - During renewal window (days 335-365): ACCEPT only if `proposal.pubkey == name_state.owner`
   - After expiration (day 365+): ACCEPT proposals from any pubkey

4. Filter out expired events when processing:
   - Ignore proposals with expiration timestamps in the past
   - Ignore attestations with expiration timestamps in the past
   - Ignore trust graphs with expiration timestamps in the past
   - Treat expired name states (>365 days old) as available for registration

5. **Set mandatory registration period**:
   - All name state events MUST have `expiration = registered_at + 31536000` (exactly 1 year)
   - Services MUST NOT accept custom expiration periods

6. Implement the consensus algorithm described above with proposal validation

7. Publish attestations (kind 20100) as ephemeral events to relays for valid proposals, with appropriate expiration tags

8. Publish name state (kind 30102) as parameterized replaceable events to relays after reaching consensus

9. Maintain local cache of active proposals, attestations, and name states (ephemeral data can be pruned after consensus)

10. Periodically renew own trust graph (kind 30101) before expiration to remain in the network

**Storage requirements (per registry service):**
- Trust graph cache: ~100 KB per 1000 registry services
- Active proposals cache: ~1 MB per 1000 pending registrations
- Attestations cache (during attestation window): ~1-10 MB depending on network size
- Name state cache: ~1 KB per name

Note: Registry services can prune ephemeral attestations after consensus is reached. Persistent data (proposals, trust graphs, name state) is stored on Nostr relays, not in the registry service.

### Bootstrap Service Discovery

New registry services should bootstrap their trust graph by:

1. Connecting to well-known Nostr relays to fetch kind 30101 events
2. Importing trust graphs from 3-5 reputable registry services
3. Using weighted average of imported trust scores as initial state
4. Gradually adjusting trust based on observed service behavior over 30 days
5. Publishing their own trust graph (kind 30101) to relays once established

## Security Considerations

### Attack Vectors and Mitigations

#### 1. Sybil Attack

**Attack**: Attacker creates thousands of registry service identities to dominate attestations.

**Mitigation**:
- Trust-weighted consensus prevents new services from having immediate influence
- Established services must trust new services before they gain weight
- Age-weighted trust (new services have reduced influence for 30 days)
- Trust graphs stored on relays are publicly auditable

#### 2. Trust Graph Manipulation

**Attack**: Attacker gradually builds trust relationships then exploits them.

**Mitigation**:
- Audit trails (kind 30101 events stored on relays are public and verifiable)
- Sudden trust changes are penalized in consensus calculations
- Diverse trust graphs mean attacker must control different sets of services for different victims

#### 3. Censorship

**Attack**: Powerful registry services refuse to attest certain names (e.g., dissidents, competitors).

**Mitigation**:
- Subjective consensus means each registry service has its own trust graph
- Censoring a name requires controlling >51% of *each service's* trust graph
- More difficult than controlling 51% of network-wide consensus
- Users can query different registry services via different relays aligned with their values

#### 4. Name Squatting

**Attack**: Registering valuable names without use.

**Mitigation**:
- **Name expiration** (mandatory): All names expire after exactly 1 year by network consensus. Registry services MUST enforce this standard registration period.
- **Preferential renewal window** (mandatory): During the final 30 days before expiration, only the current owner can renew. This prevents sniping while requiring active maintenance.
- **No indefinite holding**: Owners must actively renew during their renewal window or lose the name.
- **Optional economic cost**: Registration fees (via Lightning) can be added to further discourage speculative squatting

The mandatory expiration and renewal window mechanism ensures that:
- Abandoned names automatically return to availability after 1 year
- Owners demonstrate continued interest through annual renewal
- Owners get fair notice (30 days) to renew before losing the name
- Snipers cannot steal names during the renewal window
- The protocol automatically handles inactive name holders
- Network consensus on registration duration prevents gaming the system

**TLD Squatting:**

Since TLDs are registerable by anyone (no reserved set), expect immediate competition for valuable TLDs at network launch:
- Generic TLDs (`com`, `org`, `net`, `io`, `app`, etc.) will see intense competition
- Geographic TLDs (`us`, `uk`, `jp`, `de`, etc.) may be contested
- Cryptocurrency TLDs (`bitcoin`, `nostr`, `lightning`) will be highly sought after
- Short TLDs (single letter: `x`, `a`, `z`) will be extremely competitive

The first proposal to achieve consensus wins the TLD. Early adopters who successfully claim popular TLDs control all second-level registrations under those TLDs (e.g., owner of `com` controls `*.com` registrations).

**Subdomain Squatting Prevention:**

Subdomain authority (parent domain ownership requirement) prevents unauthorized subdomain squatting, but TLD owners could potentially:
- Squat on valuable second-level domains (e.g., `google.com`, `bitcoin.com`)
- Charge fees for second-level registrations
- Refuse to register certain names (censorship)

This is mitigated by:
- Alternative TLDs (if `com` owner censors, use `org`, `net`, `nostr`, etc.)
- Community pressure and reputation for fair TLD operation
- Ability to create unlimited new TLDs as alternatives

#### 5. Renewal Window Denial of Service

**Attack**: Attacker floods network with spam proposals during owner's renewal window to prevent owner's renewal from being processed, causing name expiration.

**Mitigation**:
- Owner has 30 full days (720 hours) to submit renewal
- Proposals expire after 5 minutes, limiting spam effectiveness
- Owner can submit multiple renewal proposals if first ones fail
- Registry services filter proposals by pubkey during renewal window (non-owner proposals automatically rejected)
- Owner should renew early in renewal window (day 335, not day 364)

**Attack**: Attacker observes owner's renewal proposal and attempts to register name immediately after expiration in case renewal fails.

**Mitigation**:
- Renewal window gives owners 30 days exclusive opportunity
- After expiration, first proposal to achieve consensus wins (fair race)
- Owner can re-register immediately after expiration with no penalty
- Network propagation delays (~30s) give no particular advantage to any participant

#### 6. Transfer Fraud

**Attack**: Forged transfer without owner's consent.

**Mitigation**:
- Transfer requires cryptographic signature from previous owner (`prev_sig`)
- Signature includes timestamp to prevent replay attacks
- Invalid signatures cause immediate rejection by honest registry services

### Privacy Considerations

- Registration proposals are public and stored on relays (necessary for consensus)
- Ownership history is permanently visible on relays storing kind 30100/30102 events
- Trust relationships are public on relays (required for verifiable consensus)
- Clients leaking name queries to relays (similar to DNS privacy issues)
  - Mitigation: Use private relays or Tor for sensitive queries
- Registry service attestations are ephemeral but visible during attestation window
  - Attestations reveal which services are active and participating

## Cryptographic Transport Security

To complete the infrastructure for replacing traditional DNS and TLS certificate authorities, this protocol includes a decentralized certificate system using Nostr cryptography and a Noise protocol variant for secure communications.

### Certificate Events (Kind 30104)

Name owners can generate certificates for their names and have them witnessed (signed) by trusted third parties. Certificates are published as parameterized replaceable events:

```json
{
  "kind": 30104,
  "pubkey": "<name_owner_pubkey>",
  "created_at": <unix_timestamp>,
  "tags": [
    ["d", "<name>"],
    ["name", "<name>"],
    ["cert_pubkey", "<service_public_key>"],   // public key for this name's service
    ["valid_from", "<unix_timestamp>"],
    ["valid_until", "<unix_timestamp>"],
    ["challenge", "<challenge_token>"],         // proof of ownership challenge
    ["challenge_proof", "<signature>"],         // signature over challenge by name owner
    ["witness", "<witness1_pubkey>", "<witness1_sig>"],  // witness signatures
    ["witness", "<witness2_pubkey>", "<witness2_sig>"],
    ["witness", "<witness3_pubkey>", "<witness3_sig>"]
  ],
  "content": "{\"algorithm\":\"secp256k1-schnorr\",\"usage\":\"tls-replacement\"}",
  "sig": "<signature>"
}
```

**Field Specifications:**

- `d` tag: The name this certificate is for
- `name` tag: The registered name being certified
- `cert_pubkey` tag: The public key used for the service (may differ from owner's Nostr pubkey)
- `valid_from`/`valid_until` tags: Certificate validity period (recommended: 90 days)
- `challenge` tag: Random challenge token for proving name ownership
- `challenge_proof` tag: Signature over `challenge||name||cert_pubkey||valid_until` by name owner
- `witness` tags: Each witness (trusted third party) signs the entire certificate content
- `content`: JSON metadata about algorithm and usage

**Certificate Validity:**

A certificate is valid if:
1. Current time is between `valid_from` and `valid_until`
2. Certificate pubkey matches name owner pubkey (from kind 30102)
3. Challenge proof signature is valid
4. At least 3 witness signatures are valid (from trusted witnesses)
5. The associated name is not expired

### Challenge-Response Protocol

To prove ownership of a name when requesting certificate witnessing, name owners must complete a challenge-response similar to Let's Encrypt:

**Step 1: Generate Challenge**

Witness generates random challenge token:
```
challenge = random_bytes(32) | hex
```

**Step 2: Publish Challenge Record**

Name owner publishes a TXT record proving control of the name:
```json
{
  "kind": 30103,
  "pubkey": "<name_owner_pubkey>",
  "tags": [
    ["d", "<name>:TXT:_challenge"],
    ["name", "<name>"],
    ["type", "TXT"],
    ["value", "_nostr-challenge=<challenge>"],
    ["ttl", "300"]
  ]
}
```

**Step 3: Verify Challenge**

Witness queries the name's TXT records:
1. Query kind 30102 to verify name ownership
2. Query kind 30103 for TXT records with `_nostr-challenge` prefix
3. Verify TXT record pubkey matches name owner
4. Confirm challenge token matches expected value
5. Validate within time window (5 minutes)

**Step 4: Issue Witness Signature**

If challenge succeeds, witness signs the certificate:
```
witness_message = cert_pubkey || name || valid_from || valid_until || challenge
witness_sig = schnorr_sign(witness_privkey, sha256(witness_message))
```

**Alternative Challenge Methods:**

- **HTTP Challenge**: Place challenge token at `https://name/.well-known/nostr-challenge`
- **DNS Challenge**: TXT record at `_nostr-challenge.name`
- **Nostr Event Challenge**: Publish kind 1 event with challenge token and name tag

### Certificate Witnesses

Witnesses are trusted entities that verify name ownership and sign certificates. Anyone can become a witness, but clients must trust witness pubkeys.

**Witness Trust Models:**

1. **Web of Trust**: Users maintain list of trusted witness pubkeys
2. **Registry Service Witnesses**: Registry services that achieve consensus can act as witnesses
3. **Community Witnesses**: Well-known Nostr identities with established reputation
4. **Threshold Witnesses**: Require N-of-M witness signatures (e.g., 3-of-5)

**Witness Responsibilities:**

- Verify name ownership via challenge-response
- Check name is registered and not expired
- Verify certificate parameters are reasonable (validity period, pubkey format)
- Sign certificate if all checks pass
- Publish witness signature to relays
- Maintain witness service availability

**Witness Discovery:**

Witnesses can publish their service information:
```json
{
  "kind": 30105,
  "pubkey": "<witness_pubkey>",
  "tags": [
    ["d", "witness-service"],
    ["endpoint", "wss://witness.example.n"],
    ["challenges", "txt", "http", "event"],
    ["max_validity", "7776000"],  // 90 days max
    ["fee", "1000"],  // sats per certificate
    ["reputation", "<reputation_event_id>"]
  ],
  "content": "{\"description\":\"Community certificate witness\",\"contact\":\"witness@example.n\"}"
}
```

### Noise Protocol Variant: Nostr Noise (NN)

For establishing secure connections, we define a Noise protocol handshake using Nostr primitives (secp256k1 ECDH and Schnorr signatures).

**Handshake Pattern: NNpsk0**

Based on Noise protocol's `NNpsk0` pattern, adapted for Nostr:

```
-> e
<- e, ee, s, es, ss
-> s, se
```

Where:
- `e` = ephemeral key
- `s` = static key
- `ee` = ephemeral-ephemeral DH
- `es` = ephemeral-static DH
- `se` = static-ephemeral DH
- `ss` = static-static DH

**Message 1: Client → Server (Initiator)**

```
message1:
  ephemeral_pubkey (32 bytes)
  encrypted_payload:
    name_requested (variable)
    supported_versions (variable)
```

Client generates ephemeral keypair and sends public key.

**Message 2: Server → Client (Responder)**

```
message2:
  ephemeral_pubkey (32 bytes)
  encrypted_payload:
    server_static_pubkey (32 bytes)     // Server's certificate pubkey
    certificate_event_id (32 bytes)      // Reference to kind 30104 certificate
    schnorr_signature (64 bytes)         // Server signs (client_ephemeral || server_ephemeral || name)
```

Server:
1. Generates ephemeral keypair
2. Computes `ee` = ECDH(server_ephemeral_priv, client_ephemeral_pub)
3. Derives encryption key from `ee`
4. Encrypts payload containing static pubkey and certificate reference
5. Signs handshake with certificate private key

**Message 3: Client → Server (Authentication)**

```
message3:
  encrypted_payload:
    client_static_pubkey (32 bytes)      // Optional: client auth
    schnorr_signature (64 bytes)         // Client signature (optional)
    application_data (variable)
```

Client:
1. Verifies server's certificate (query kind 30104, validate witnesses)
2. Verifies server's signature
3. Computes `es` = ECDH(client_ephemeral_priv, server_static_pub)
4. Computes `se` = ECDH(client_static_priv, server_ephemeral_pub)
5. Computes `ss` = ECDH(client_static_priv, server_static_pub) (if client auth)
6. Derives final encryption keys from `ee || es || se || ss`
7. Sends authenticated payload

**Key Derivation:**

After handshake completes:
```
handshake_hash = SHA256(message1 || message2 || message3)
encryption_key = HKDF(chaining_key, handshake_hash, "nostr-noise-1.0")
```

Derives two keys:
- `initiator_key`: Client → Server encryption
- `responder_key`: Server → Client encryption

**Encryption:**

Uses ChaCha20-Poly1305 AEAD with:
- 96-bit nonce (counter-based)
- 256-bit key from key derivation
- Associated data: sequence number

**Certificate Verification in Handshake:**

Client must verify server certificate:
1. Fetch kind 30104 event by ID provided in message 2
2. Verify certificate is not expired (`valid_from` to `valid_until`)
3. Verify `cert_pubkey` matches server's static pubkey
4. Verify certificate owner matches name being requested (query kind 30102)
5. Verify challenge proof signature
6. Verify at least 3 witness signatures from trusted witnesses
7. Verify name registration is not expired
8. Accept connection only if all checks pass

**Perfect Forward Secrecy:**

Ephemeral keys provide forward secrecy. Even if static keys are compromised later, past sessions remain secure due to ephemeral ECDH exchanges.

### Integration with Name Records

The complete flow for establishing a secure connection to a name:

**Step 1: Name Resolution**

```
1. Client queries "example.n" for A/AAAA records
2. Client receives IP address(es) from kind 30103 records
3. Client verifies records are signed by current name owner
```

**Step 2: Certificate Discovery**

```
4. Client queries kind 30104 for "example.n" certificate
5. Client validates certificate:
   - Check validity period
   - Verify owner pubkey matches name owner (kind 30102)
   - Verify challenge proof
   - Verify witness signatures (at least 3 trusted)
   - Confirm name not expired
```

**Step 3: Secure Connection**

```
6. Client initiates TCP connection to resolved IP
7. Client starts Nostr Noise handshake
8. Client sends ephemeral key (message 1)
9. Server responds with ephemeral key + certificate (message 2)
10. Client verifies certificate matches queried certificate
11. Client completes handshake (message 3)
12. Encrypted communication begins
```

**Step 4: Certificate Pinning**

```
13. Client caches certificate pubkey for this name
14. Future connections verify same pubkey (TOFU - Trust On First Use)
15. Client warns if certificate pubkey changes
```

### Certificate Renewal

Certificates should be renewed before expiration (recommended: every 90 days):

1. **Generate new challenge**: Request fresh challenge from witnesses
2. **Prove ownership**: Complete challenge-response (TXT record or HTTP)
3. **Request signatures**: Submit to witnesses for signature
4. **Publish certificate**: Publish new kind 30104 event with updated validity period
5. **Overlap period**: New certificate should overlap old by 7 days for smooth transition

**Automated Renewal:**

Services can automate certificate renewal:
```
while true:
  if certificate_expires_in < 30_days:
    new_cert = request_certificate_renewal()
    publish_certificate(new_cert)
    sleep(7_days)
  else:
    sleep(1_day)
```

### Security Properties

**Advantages over Traditional TLS/CA:**

1. **No Certificate Authorities**: No centralized trust anchors to compromise
2. **Transparent Witnessing**: All certificate issuance is public and auditable
3. **Key Continuity**: Same Nostr keys used for name ownership and certificates
4. **Instant Revocation**: Delete kind 30104 event to revoke certificate
5. **Integrated with Names**: Certificate validity tied to name registration
6. **Threshold Trust**: Require multiple witnesses (3-of-5, 5-of-7, etc.)
7. **User-Selected Trust**: Clients choose which witnesses they trust

**Threat Model:**

- **Compromised Witness**: Requires 3+ witnesses, attacker must compromise majority
- **Name Hijacking**: Name ownership verified via registry consensus
- **MitM Attack**: Client verifies certificate ownership chain and witness signatures
- **Certificate Forgery**: Impossible without both name owner key and witness keys
- **Replay Attacks**: Prevented by ephemeral keys and sequence numbers in Noise protocol

**Limitations:**

- **Bootstrap Trust**: Users must initially trust some witnesses (similar to CA root trust)
- **Witness Availability**: Need responsive witnesses for certificate issuance
- **Certificate Size**: Larger than X.509 due to multiple witness signatures
- **Compatibility**: Not compatible with traditional TLS/X.509 infrastructure

### Event Kind Summary

| Kind | Purpose | Persistence |
|------|---------|-------------|
| 30104 | Certificates | Parameterized replaceable (by name) |
| 30105 | Witness Service Info | Parameterized replaceable (by witness) |

## Discussion

### Trust Graph Bootstrapping

The effectiveness of trust-weighted consensus depends on establishing a healthy trust graph. This presents a chicken-and-egg problem: new registry services need trust to participate, but must participate to earn trust.

**Proposed bootstrapping strategies:**

#### 1. Seed Service Trust Inheritance

New registry services can inherit trust graphs from established "seed" services by fetching their kind 30101 events from relays:

```json
{
  "kind": 30101,
  "tags": [
    ["inherit", "<seed_service_pubkey>", "0.8"],
    ["inherit", "<seed_service_pubkey2>", "0.6"]
  ]
}
```

The new service computes its initial trust graph as a weighted average of the seed services' graphs. Over time (30-90 days), the service adjusts trust based on observed behavior and publishes updated trust graphs to relays.

**Trade-offs:**
- ✅ Enables immediate participation
- ✅ Leverages existing reputation
- ❌ Creates trust concentrations around seed services
- ❌ Seed service compromise affects many descendants

#### 2. Web of Trust Endorsements

Established registry service operators can endorse new services through signed endorsements (stored on relays):

```json
{
  "kind": 1100,
  "pubkey": "<established_service_pubkey>",
  "tags": [
    ["p", "<new_service_pubkey>"],
    ["endorsement", "verified"],
    ["duration", "30d"],
    ["initial_trust", "0.3"]
  ]
}
```

Other services consuming this endorsement (from relays) automatically grant the new service temporary trust (e.g., 0.3 for 30 days), after which it decays to 0 unless organically reinforced.

**Trade-offs:**
- ✅ Decentralized trust building
- ✅ Time-limited risk exposure
- ✅ Organic network growth
- ❌ Slower bootstrap for new services
- ❌ Endorsement spam risk

#### 3. Proof-of-Stake Bootstrap

New registry services can stake economic value (e.g., lock tokens in a Bitcoin Lightning channel) to gain initial trust. Stake proof events are published to relays:

```json
{
  "kind": 30103,
  "pubkey": "<new_service_pubkey>",
  "tags": [
    ["stake", "<lightning_invoice>", "1000000"],  // 1M sats
    ["stake_duration", "180d"],
    ["stake_proof", "<proof_of_lock>"]
  ]
}
```

Other services fetching these events from relays treat staked services as having trust proportional to stake amount. Dishonest behavior results in slashing (stake destruction).

**Trade-offs:**
- ✅ Sybil resistant (economic cost)
- ✅ Clear incentive alignment
- ✅ Immediate trust proportional to stake
- ❌ Requires economic layer integration
- ❌ Excludes low-resource operators
- ❌ Introduces plutocracy risk

#### 4. Proof-of-Work Bootstrap

New registry services solve computational puzzles to earn temporary trust. PoW proof events are published to relays:

```json
{
  "kind": 30104,
  "pubkey": "<new_service_pubkey>",
  "tags": [
    ["pow", "<nonce>", "24"],  // 24-bit difficulty
    ["pow_created", "<timestamp>"]
  ]
}
```

Higher difficulty equals higher initial trust. PoW expires after 90 days unless organic trust replaces it.

**Trade-offs:**
- ✅ Sybil resistant (computational cost)
- ✅ No economic barrier
- ✅ Proven model (Hashcash, Bitcoin)
- ❌ Energy intensive
- ❌ Favors well-resourced operators
- ❌ Difficulty calibration challenges

**Recommendation**: Implement **hybrid approach**:
- Start with seed service inheritance (#1) for immediate participation
- Layer WoT endorsements (#2) for organic trust growth
- Optional PoW (#4) for permissionless entry without social connections
- Reserve PoS (#3) for future upgrade if economic attacks become problematic

### Economic Incentives

Why would registry service operators honestly attest to name registrations? Without proper incentives, rational operators might:
- Refuse to attest (freeloading)
- Attest dishonestly (e.g., favoring paying customers)
- Collude to manipulate consensus

**Proposed incentive mechanisms:**

#### 1. Registration Fees

Name registration proposals include optional tips to registry services that attest. Proposals are published to relays with fee information:

```json
{
  "kind": 30100,
  "tags": [
    ["d", "foo.n"],
    ["fee", "<lightning_invoice>", "10000"],  // 10k sats
    ["fee_distribution", "attestors"]  // or "weighted" or "threshold"
  ]
}
```

**Distribution strategies:**
- **Attestors**: Split among all services that attested (encourages broad participation)
- **Weighted**: Proportional to trust weight (rewards influential services)
- **Threshold**: All-or-nothing (only if consensus reached)

**Trade-offs:**
- ✅ Direct incentive for honest attestation
- ✅ Market-driven fee discovery
- ✅ Revenue for registry service operators
- ❌ Plutocracy risk (wealthy users get priority)
- ❌ Creates spam incentive (register garbage for fees)

#### 2. Reputation Markets

Registry service operators earn reputation scores based on attestation quality. Reputation events are published to relays:

```json
{
  "kind": 30105,
  "tags": [
    ["p", "<service_pubkey>"],
    ["metric", "accuracy", "0.97"],      // % of attestations matching consensus
    ["metric", "uptime", "0.99"],        // % of proposals attested
    ["metric", "latency", "15"],         // avg attestation time (seconds)
    ["period", "<start>", "<end>"]
  ]
}
```

High-reputation services gain:
- Greater trust from other services
- Priority in client queries (clients prefer high-reputation services)
- Higher economic value (e.g., subscription fees, tips)

**Trade-offs:**
- ✅ Long-term incentive alignment
- ✅ Punishes dishonest behavior
- ✅ No direct economic barrier
- ❌ Reputation calculation is complex
- ❌ Slow feedback loop (months to build reputation)

#### 3. Mutual Benefit Networks

Registry services form explicit cooperation agreements (published to relays):

```json
{
  "kind": 30106,
  "tags": [
    ["p", "<partner_service1>", "wss://service1.example.com"],
    ["p", "<partner_service2>", "wss://service2.example.com"],
    ["agreement", "reciprocal_attestation"],
    ["sla", "95_percent_uptime"]
  ]
}
```

Services attest to partners' proposals in exchange for reciprocal service. Violating agreements results in removal from the network.

**Trade-offs:**
- ✅ No economic transactions needed
- ✅ Builds social cohesion
- ✅ Flexible SLA definitions
- ❌ Creates service cartels
- ❌ Excludes new entrants
- ❌ May lead to consensus fragmentation

#### 4. Altruism + Low Operational Cost

If attestation cost is sufficiently low, registry service operators may participate altruistically:

- Storage: ~1-10 MB for active proposals and attestations (ephemeral data, pruned after consensus)
- Bandwidth: ~1 KB per attestation published to relays
- Computation: Simple signature verification and weighted sum
- Network: WebSocket connections to a few Nostr relays

At 1000 registrations/day with 10% sparse attestation:
- **Cost per service**: <$1/month in infrastructure (plus relay costs if self-hosting)
- **Comparable to**: Running a lightweight daemon alongside a Nostr relay

Many operators already run services without direct compensation, motivated by:
- Ideological alignment (decentralization, censorship resistance)
- Supporting applications they use
- Technical interest and experimentation
- Using existing Nostr relays means no additional infrastructure

**Trade-offs:**
- ✅ No economic complexity
- ✅ Aligns with Nostr's ethos
- ✅ Proven model (Nostr relay operators, Bitcoin nodes)
- ❌ May not scale to high-value registrations
- ❌ Vulnerable to tragedy of the commons

**Recommendation**: Start with **altruism (#4)** supplemented by **reputation (#2)**:
- Most service operators will participate without direct payment (proven by existing Nostr network)
- Implement transparent reputation metrics to reward high-quality attestors
- Enable optional tipping (#1) for users who want priority or to support services
- Reserve mutual benefit networks (#3) for enterprise deployments

### Client Conflict Resolution

Clients querying name ownership may receive conflicting responses from different registry services due to:
1. Trust graph divergence (different services trust different attestors)
2. Network partitions (some services missed attestations from relays)
3. Byzantine services (malicious responses)

**Conflict resolution strategies:**

#### 1. Majority Consensus

Client queries kind 30102 events from N relays (recommended N=5) to get name state from different registry services, then accepts the majority response:

```python
def resolve_name(name, relays):
    responses = []
    for relay in relays:
        # Query kind 30102 events with d tag = name
        # Each event is from a different registry service
        name_states = query_relay(relay, kind=30102, filter={"#d": [name]})
        for state in name_states:
            owner = state.tags["owner"]
            responses.append(owner)

    # Count occurrences
    counts = {}
    for owner in responses:
        counts[owner] = counts.get(owner, 0) + 1

    # Return majority (>50%)
    for owner, count in counts.items():
        if count > len(responses) / 2:
            return owner

    return None  # No majority
```

**Trade-offs:**
- ✅ Simple and intuitive
- ✅ Byzantine fault tolerant (up to N/2 malicious services)
- ✅ Works with any relay set
- ❌ Requires querying multiple relays (latency)
- ❌ No nuance for different service qualities

#### 2. Trust-Weighted Voting

Client maintains its own trust graph and weights responses by registry service pubkey:

```python
def resolve_name_weighted(name, relays, trust_scores):
    responses = {}
    for relay in relays:
        # Query kind 30102 events
        name_states = query_relay(relay, kind=30102, filter={"#d": [name]})
        for state in name_states:
            service_pubkey = state.pubkey
            owner = state.tags["owner"]
            weight = trust_scores.get(service_pubkey, 0.5)  # default 0.5
            responses[owner] = responses.get(owner, 0) + weight

    # Return highest weighted response
    return max(responses, key=responses.get)
```

**Trade-offs:**
- ✅ Rewards high-quality services
- ✅ Resilient to Sybil attacks
- ✅ Customizable trust model
- ❌ Client must maintain trust graph
- ❌ More complex UX

#### 3. Consensus Confidence Scoring

Registry services include confidence scores in name state events (kind 30102):

```json
{
  "kind": 30102,
  "pubkey": "<registry_service_pubkey>",
  "tags": [
    ["d", "foo.n"],
    ["owner", "<pubkey>"],
    ["confidence", "0.87"],  // 87% of trust weight agreed
    ["attestations", "42"]   // 42 services attested
  ]
}
```

Client queries kind 30102 events from multiple relays and uses the response with highest confidence:

```python
def resolve_name_confidence(name, relays):
    best_response = None
    best_confidence = 0

    for relay in relays:
        # Query kind 30102 events
        name_states = query_relay(relay, kind=30102, filter={"#d": [name]})
        for state in name_states:
            confidence = float(state.tags["confidence"])

            if confidence > best_confidence:
                best_confidence = confidence
                best_response = state.tags["owner"]

    # Warn if low confidence
    if best_confidence < 0.6:
        warn_user("Low consensus confidence")

    return best_response
```

**Trade-offs:**
- ✅ Transparent consensus strength
- ✅ User warnings for disputed names
- ✅ Minimal client complexity
- ❌ Trusts registry service's confidence calculation
- ❌ Services could lie about confidence

#### 4. Recursive Attestation Verification

Client fetches attestations (kind 20100) from relays and recomputes consensus locally:

```python
def resolve_name_verify(name, relays):
    # 1. Get all proposals for this name
    proposals = []
    for relay in relays:
        props = query_relay(relay, kind=30100, filter={"d": name})
        proposals.extend(props)

    # 2. Get all attestations for these proposals
    attestations = []
    for relay in relays:
        atts = query_relay(relay, kind=20100, filter={"e": [p.id for p in proposals]})
        attestations.extend(atts)

    # 3. Get trust graphs
    trust_graphs = []
    for relay in relays:
        graphs = query_relay(relay, kind=30101)
        trust_graphs.extend(graphs)

    # 4. Recompute consensus locally
    return compute_consensus(proposals, attestations, trust_graphs)
```

**Trade-offs:**
- ✅ Maximum trustlessness
- ✅ Client verifies everything
- ✅ Can audit registry service dishonesty
- ❌ Significant bandwidth and computation
- ❌ Complex client implementation
- ❌ May timeout on large registries
- ❌ Ephemeral attestations may be pruned from relays

#### 5. Hybrid: Quick Query with Dispute Resolution

Normal case: Query 3 relays, accept majority (fast path)
Dispute case: If no majority, fetch attestations and recompute (slow path)

```python
def resolve_name_hybrid(name, relays):
    # Fast path: majority consensus
    responses = [query_relay(r, name) for r in relays]
    majority = get_majority(responses)

    if majority:
        return majority

    # Slow path: fetch attestations and verify
    return resolve_name_verify(name, relays)
```

**Trade-offs:**
- ✅ Fast common case (1 round trip)
- ✅ Trustless dispute resolution
- ✅ Best of both worlds
- ❌ Unpredictable latency (95% fast, 5% slow)

**Recommendation**: Implement **hybrid approach (#5)**:
- Default to majority consensus for speed
- Fall back to attestation verification on conflicts
- Display confidence scores (#3) to users
- Allow power users to enable trust-weighted voting (#2)
- Provide CLI tool for full recursive verification (#4) for auditing

This provides good UX for most users while maintaining trustless properties for disputed or high-value names.

## References

- [NIP-01: Basic protocol flow](https://github.com/nostr-protocol/nips/blob/master/01.md)
- [NIP-33: Parameterized replaceable events](https://github.com/nostr-protocol/nips/blob/master/33.md)
- [NIP-40: Expiration timestamp](https://github.com/nostr-protocol/nips/blob/master/40.md)
- [NIP-44: Encrypted Direct Messages (ECDH)](https://github.com/nostr-protocol/nips/blob/master/44.md)
- [NIP-65: Relay list metadata](https://github.com/nostr-protocol/nips/blob/master/65.md)
- [Noise Protocol Framework](http://www.noiseprotocol.org/noise.html) - Perrin, T. (2018)
- [Let's Encrypt ACME Protocol](https://datatracker.ietf.org/doc/html/rfc8555) - RFC 8555
- PageRank algorithm: Brin, S. and Page, L. (1998). "The anatomy of a large-scale hypertextual Web search engine"
- Byzantine Generals Problem: Lamport, L., Shostak, R., and Pease, M. (1982)

## Changelog

- 2025-01-XX: Initial draft
  - Independent service architecture (services separate from relays)
  - Trust-weighted consensus with Byzantine fault tolerance
  - Standard DNS naming conventions (lowercase, dot-separated, hierarchical)
  - TLDs registerable by anyone (no ICANN) - expect TLD registration race
  - Subdomain authority enforced (must own parent domain)
  - Expiration tags for protocol boundaries and relay housekeeping
  - Mandatory 1-year registration period by network consensus
  - Preferential 30-day renewal window (days 335-365) for current owners
  - Name state, trust graphs, proposals, and attestations as Nostr events
  - DNS-style name records (A, AAAA, CNAME, MX, TXT, NS, SRV) with per-type limits
  - Subdomain delegation via NS records
  - Decentralized certificate system with challenge-response verification
  - Threshold witnessing (3+ signatures) replacing certificate authorities
  - Nostr Noise protocol for secure authenticated transport (TLS replacement)
  - Perfect forward secrecy via ephemeral key exchanges
