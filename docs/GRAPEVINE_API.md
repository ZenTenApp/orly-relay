# GrapeVine WoT Influence Scoring API

The GrapeVine API computes Web of Trust influence scores using an iterative convergence algorithm over the follow graph. It uses Badger's materialized graph indexes (ppg/gpp) for fast traversal, avoiding the need for Neo4j or SQL databases.

## Enabling

The API is disabled by default. Enable it with:

```bash
ORLY_GRAPEVINE_ENABLED=true
```

This requires the Badger database backend (`ORLY_DB_TYPE=badger`, which is the default).

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `ORLY_GRAPEVINE_ENABLED` | `false` | Enable the GrapeVine scoring API |
| `ORLY_GRAPEVINE_MAX_DEPTH` | `6` | Maximum BFS depth for follow graph traversal |
| `ORLY_GRAPEVINE_CYCLES` | `5` | Number of convergence iterations |
| `ORLY_GRAPEVINE_ATTENUATION` | `0.8` | Weight decay factor per hop (0-1). Lower = faster decay. |
| `ORLY_GRAPEVINE_RIGOR` | `0.25` | Certainty curve steepness (0-1). Lower = more certainty from fewer inputs. |
| `ORLY_GRAPEVINE_FOLLOW_CONFIDENCE` | `0.05` | Base confidence weight for a follow edge |
| `ORLY_GRAPEVINE_OBSERVERS` | | Comma-separated hex pubkeys to auto-calculate scores for |
| `ORLY_GRAPEVINE_REFRESH` | `6h` | How often to recalculate scores for configured observers |

### Example

```bash
ORLY_GRAPEVINE_ENABLED=true
ORLY_GRAPEVINE_OBSERVERS=abc123...hex64,def456...hex64
ORLY_GRAPEVINE_REFRESH=4h
ORLY_GRAPEVINE_MAX_DEPTH=4
```

## Algorithm

The scoring algorithm has two components:

### Influence Scores (GrapeVine convergence)

1. Starting from the observer's pubkey, BFS traverses the follow graph to `MAX_DEPTH` hops
2. All pubkeys in this set get an initial influence of 0.0; the observer starts at 1.0
3. For `CYCLES` iterations:
   - For each pubkey (ratee) in the set:
     - Find all followers of that ratee who are also in the set
     - Each follower's "vote" is weighted by: `ATTENUATION * follower_influence * FOLLOW_CONFIDENCE`
     - The observer's own follows get no attenuation (direct trust)
     - `average = sum_of_products / sum_of_weights`
     - `certainty = 1 - exp(-sum_of_weights * -ln(RIGOR))`
     - `influence = average * certainty`
4. Scores stabilize after a few iterations (typically 3-5)

### WoT Intersection Scores

For each pubkey in the hop set, count how many of the observer's direct follows also follow that pubkey. This gives a simple "social proof" metric independent of the influence calculation.

### Placeholders

Mute lists (kind 10000) and report events (kind 1984) are not yet factored into the calculation. The algorithm has TODO hooks for integrating these signals.

## API Endpoints

All endpoints require NIP-98 HTTP authentication. Use the `nurl` tool for testing:

```bash
# Build nurl
go build -o nurl ./cmd/nurl
```

### GET /api/grapevine/scores

Returns all scores for an observer.

**Parameters:**
- `observer` (query, optional): Hex pubkey of the observer. Defaults to the authenticated pubkey.

**Authorization:** Any authenticated user can query their own scores. Owner/admin can query any observer.

**Example:**

```bash
NOSTR_SECRET_KEY=nsec1... ./nurl "https://relay.example.com/api/grapevine/scores?observer=abc123..."
```

**Response (200):**

```json
{
  "observer": "abc123...",
  "scores": [
    {
      "pubkey": "def456...",
      "influence": 0.85,
      "average": 0.90,
      "certainty": 0.94,
      "input": 0.15,
      "wot_score": 42,
      "depth": 1
    }
  ],
  "computed_at": "2026-03-12T10:30:00Z",
  "compute_ms": 4500,
  "total_pubkeys": 1234
}
```

**Response (404):** No scores computed yet for this observer.

### GET /api/grapevine/score

Returns a single score for an observer+target pair.

**Parameters:**
- `observer` (query, optional): Hex pubkey of the observer. Defaults to authenticated pubkey.
- `target` (query, required): Hex pubkey of the target.

**Example:**

```bash
NOSTR_SECRET_KEY=nsec1... ./nurl "https://relay.example.com/api/grapevine/score?observer=abc123...&target=def456..."
```

**Response (200):**

```json
{
  "observer": "abc123...",
  "target": "def456...",
  "influence": 0.85,
  "average": 0.90,
  "certainty": 0.94,
  "input": 0.15,
  "wot_score": 42,
  "depth": 1
}
```

### POST /api/grapevine/recalculate

Triggers an async score recalculation for an observer. Returns immediately.

**Body (JSON):**

```json
{"observer": "abc123..."}
```

If `observer` is omitted, uses the authenticated pubkey. Owner/admin can trigger for any observer; others can only trigger their own.

**Example:**

```bash
NOSTR_SECRET_KEY=nsec1... ./nurl "https://relay.example.com/api/grapevine/recalculate" '{"observer":"abc123..."}'
```

**Response (202):**

```json
{"status": "started", "observer": "abc123..."}
```

Or if already computing:

```json
{"status": "already_computing", "observer": "abc123..."}
```

## Score Fields

| Field | Type | Description |
|-------|------|-------------|
| `pubkey` | string | Hex pubkey of the scored user |
| `influence` | float64 | Combined score: `average * certainty` (0.0 to 1.0) |
| `average` | float64 | Weighted average of rater scores (0.0 to 1.0) |
| `certainty` | float64 | Confidence based on total input weight (0.0 to 1.0) |
| `input` | float64 | Raw sum of input weights from raters |
| `wot_score` | int | Number of observer's follows who also follow this pubkey |
| `depth` | int | BFS hop distance from observer (1 = direct follow) |

## Storage

Scores are persisted in Badger under the key prefixes `GV_SET_` (full score sets) and `GV_ENT_` (individual entries). They survive relay restarts. A recalculation replaces the previous scores for that observer.

## Performance

The algorithm uses Badger's materialized ppg/gpp indexes, which encode follow graph edges as 21-byte binary keys. Each follower lookup is a single sorted prefix scan — no event decoding, no SQL parsing.

For a typical relay with ~50k pubkeys in the 2-hop set and 5 convergence cycles, computation completes in seconds. Follower lookups are cached within a single computation run.
