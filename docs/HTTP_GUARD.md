# HTTP Guard — Bot Blocking and Rate Limiting

## Overview

The HTTP Guard is application-level middleware that protects ORLY from abusive traffic: automated scrapers, AI crawlers, and high-volume requesters. It runs inside ORLY's HTTP handler, before any routing, so it covers both REST API endpoints and WebSocket upgrade requests.

This is designed for deployments where the reverse proxy cannot be customized — particularly Cloudron, which runs nginx but does not allow user modifications to its configuration. For deployments behind a configurable reverse proxy (Caddy, nginx, HAProxy), you can use the HTTP Guard as defense-in-depth or disable it in favor of proxy-level rules.

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `ORLY_HTTP_GUARD_ENABLED` | `true` | Enable the HTTP guard middleware |
| `ORLY_HTTP_GUARD_BOT_BLOCK` | `true` | Block known scraper/bot User-Agents |
| `ORLY_HTTP_GUARD_RPM` | `120` | Max HTTP requests per minute per IP |
| `ORLY_HTTP_GUARD_WS_PER_MIN` | `10` | Max WebSocket upgrade requests per minute per IP |

The existing `ORLY_IP_BLACKLIST` variable is also respected — IPs matching any blacklist prefix are blocked with 403 before any other checks.

## Bot Blocking

When `ORLY_HTTP_GUARD_BOT_BLOCK=true`, requests with User-Agent strings containing any of the following substrings (case-insensitive) are rejected with HTTP 403:

| Bot | Operator |
|-----|----------|
| SemrushBot | Semrush (SEO crawler) |
| AhrefsBot | Ahrefs (SEO crawler) |
| MJ12bot | Majestic (SEO crawler) |
| DotBot | Moz (SEO crawler) |
| PetalBot | Huawei/Aspiegel (search) |
| BLEXBot | WebMeUp (backlink checker) |
| DataForSeoBot | DataForSEO (SEO data) |
| Amazonbot | Amazon (product indexing) |
| meta-externalagent | Meta/Facebook (content scraper) |
| Bytespider | ByteDance/TikTok (crawler) |
| GPTBot | OpenAI (AI training crawler) |
| ClaudeBot | Anthropic (AI training crawler) |
| CCBot | Common Crawl (dataset crawler) |
| FacebookBot | Meta (social preview crawler) |

This list matches the scraper blocking rules from the relay.orly.dev Caddy configuration. Legitimate search engines (Googlebot, Bingbot) are not blocked.

To disable bot blocking while keeping rate limiting active:

```bash
ORLY_HTTP_GUARD_BOT_BLOCK=false
```

## Rate Limiting

Each client IP gets two independent token buckets:

- **HTTP bucket** — Starts at `ORLY_HTTP_GUARD_RPM` tokens. Each HTTP request consumes one token. Refills to maximum every 60 seconds.
- **WebSocket bucket** — Starts at `ORLY_HTTP_GUARD_WS_PER_MIN` tokens. Each WebSocket upgrade request consumes one token (in addition to the HTTP token). Refills to maximum every 60 seconds.

When a bucket is exhausted, the request is rejected with HTTP 429 (Too Many Requests) and a `Retry-After: 60` header.

### Why Separate WebSocket Limits

A single WebSocket connection is far more expensive than an HTTP request — it holds a goroutine, consumes memory for subscription state, and generates continuous traffic for the lifetime of the connection. Rate limiting WebSocket upgrades separately prevents a single IP from opening hundreds of connections while still allowing normal HTTP API usage.

### IP Extraction

The guard determines the client IP using this priority:

1. `X-Forwarded-For` header (first IP in chain) — covers reverse proxy deployments
2. `X-Real-Ip` header — alternative proxy header
3. `RemoteAddr` from the connection — direct connections

In Cloudron, the `X-Forwarded-For` header is set by Cloudron's nginx. In direct deployments, `RemoteAddr` is used.

### Memory Management

Per-IP state is stored in a concurrent map. A background goroutine runs every 5 minutes and evicts entries for IPs that haven't been seen in the last 10 minutes. This prevents memory growth from drive-by scanners.

## Interaction with Other Protections

The HTTP Guard runs **before** all other request processing in `ServeHTTP`:

```
HTTP request → HTTP Guard (bot + rate) → CORS → Blossom → WebSocket → API routing
```

It complements (does not replace) ORLY's other protection mechanisms:

| Layer | Scope | Mechanism |
|-------|-------|-----------|
| **HTTP Guard** | All HTTP + WS | Bot UA blocking, per-IP rate limiting |
| **IP Blacklist** (`ORLY_IP_BLACKLIST`) | All connections | Prefix-match IP blocking (also checked in Guard) |
| **Per-IP Connection Limit** (`ORLY_MAX_CONN_PER_IP`) | WebSocket only | Max concurrent WS connections per IP |
| **Global Connection Limit** (`ORLY_MAX_GLOBAL_CONNECTIONS`) | WebSocket only | Total WS connection cap |
| **PID Rate Limiter** (`ORLY_RATE_LIMIT_*`) | Database operations | Memory-pressure-adaptive throttling |
| **Query Result Limit** (`ORLY_QUERY_RESULT_LIMIT`) | Nostr REQ queries | Max events per filter response |

## Cloudron Deployment

The HTTP Guard is enabled by default in the cloudron-orly deployment template. The environment variables are set in `/app/data/orly.env`:

```bash
export ORLY_HTTP_GUARD_ENABLED="true"
export ORLY_HTTP_GUARD_BOT_BLOCK="true"
```

No port changes or nginx configuration are needed.

## Disabling

To disable the guard entirely:

```bash
ORLY_HTTP_GUARD_ENABLED=false
```

This is appropriate when running behind a reverse proxy that already handles bot blocking and rate limiting (e.g., Caddy with `respond` rules, Cloudflare, or nginx with `limit_req`).
