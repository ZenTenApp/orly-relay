package app

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v4"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/acl"
	"git.mleku.dev/mleku/nostr/encoders/bech32encoding"
	"git.mleku.dev/mleku/nostr/encoders/envelopes/authenvelope"
	"git.mleku.dev/mleku/nostr/encoders/envelopes/closedenvelope"
	"git.mleku.dev/mleku/nostr/encoders/envelopes/eoseenvelope"
	"git.mleku.dev/mleku/nostr/encoders/envelopes/eventenvelope"
	"git.mleku.dev/mleku/nostr/encoders/envelopes/reqenvelope"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	hexenc "git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/reason"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"next.orly.dev/pkg/policy"
	"next.orly.dev/pkg/protocol/graph"
	"next.orly.dev/pkg/protocol/nip43"
	"next.orly.dev/pkg/protocol/publish"
	"next.orly.dev/pkg/ratelimit"
	"git.mleku.dev/mleku/nostr/utils/normalize"
	"git.mleku.dev/mleku/nostr/utils/pointers"
)

func (l *Listener) HandleReq(msg []byte) (err error) {
	log.D.F("handling REQ: %s", msg)
	// var rem []byte
	env := reqenvelope.New()
	if _, err = env.Unmarshal(msg); chk.E(err) {
		// Provide more specific error context for JSON parsing failures
		if strings.Contains(err.Error(), "invalid character") {
			log.E.F("REQ JSON parsing failed from %s: %v", l.remote, err)
			log.T.F("REQ malformed message from %s: %q", l.remote, string(msg))
			return normalize.Error.Errorf("malformed REQ message: %s", err.Error())
		}
		return normalize.Error.Errorf(err.Error())
	}
	log.T.C(
		func() string {
			return fmt.Sprintf(
				"REQ sub=%s filters=%d", env.Subscription, len(*env.Filters),
			)
		},
	)

	// Classify query cost for adaptive rate limiting
	var totalAuthors, totalKinds, totalIds int
	var hasLimit bool
	var limitVal int
	for _, f := range *env.Filters {
		if f == nil {
			continue
		}
		if f.Authors != nil {
			totalAuthors += f.Authors.Len()
		}
		if f.Kinds != nil {
			totalKinds += f.Kinds.Len()
		}
		if f.Ids != nil {
			totalIds += f.Ids.Len()
		}
		if f.Limit != nil {
			hasLimit = true
			limitVal = int(*f.Limit)
		}
	}
	qCost := ratelimit.ClassifyQuery(totalAuthors, totalKinds, totalIds, hasLimit, limitVal)
	log.D.F("REQ %s: query cost=%s (authors=%d, kinds=%d, ids=%d, limit=%v)",
		env.Subscription, qCost.Level, totalAuthors, totalKinds, totalIds, limitVal)

	// Track accumulated cost per connection (units: multiplier * 100)
	l.queryCostAccumulator.Add(int64(qCost.Multiplier * 100))

	// Adaptive query deferral: apply cost-weighted delay under load
	if l.rateLimiter != nil && l.rateLimiter.IsEnabled() {
		baseDelay := l.rateLimiter.ComputeDelay(ratelimit.Read)
		if baseDelay > 0 {
			costDelay := time.Duration(float64(baseDelay) * qCost.Multiplier)
			if costDelay > 0 {
				log.D.F("REQ %s: cost-weighted delay %v (cost=%s, base=%v)",
					env.Subscription, costDelay, qCost.Level, baseDelay)
				select {
				case <-l.ctx.Done():
					return nil
				case <-time.After(costDelay):
				}
			}
		}

		// In emergency mode, reject expensive queries outright
		if l.rateLimiter.InEmergencyMode() && qCost.Level >= ratelimit.CostHeavy {
			log.W.F("REQ %s: rejecting expensive query (cost=%s) during emergency mode",
				env.Subscription, qCost.Level)
			if err = closedenvelope.NewFrom(
				env.Subscription,
				reason.Error.F("server overloaded, please retry later"),
			).Write(l); chk.E(err) {
				return
			}
			return nil
		}
	}

	// NIP-46 signer-based authentication:
	// If client is not authenticated and requests kind 24133 with exactly one #p tag,
	// check if there's an active signer subscription for that pubkey.
	// If so, authenticate the client as that pubkey.
	const kindNIP46 = 24133
	if len(l.authedPubkey.Load()) == 0 && len(*env.Filters) == 1 {
		f := (*env.Filters)[0]
		if f != nil && f.Kinds != nil && f.Kinds.Len() == 1 {
			isNIP46Kind := false
			for _, k := range f.Kinds.K {
				if k.K == kindNIP46 {
					isNIP46Kind = true
					break
				}
			}
			if isNIP46Kind && f.Tags != nil {
				pTag := f.Tags.GetFirst([]byte("p"))
				// Must have exactly one pubkey in the #p tag
				if pTag != nil && pTag.Len() == 2 {
					signerPubkey := pTag.Value()
					// Convert to binary if hex
					var signerPubkeyBin []byte
					if len(signerPubkey) == 64 {
						signerPubkeyBin, _ = hexenc.Dec(string(signerPubkey))
					} else if len(signerPubkey) == 32 {
						signerPubkeyBin = signerPubkey
					}
					if len(signerPubkeyBin) == 32 {
						// Check if there's an active signer for this pubkey
						if socketPub := l.publishers.GetSocketPublisher(); socketPub != nil {
							if checker, ok := socketPub.(publish.NIP46SignerChecker); ok {
								if checker.HasActiveNIP46Signer(signerPubkeyBin) {
									log.I.F("NIP-46 auth: client %s authenticated via active signer %s",
										l.remote, hexenc.Enc(signerPubkeyBin))
									l.authedPubkey.Store(signerPubkeyBin)
								}
							}
						}
					}
				}
			}
		}
	}

	// send a challenge to the client to auth if an ACL is active, auth is required, or AuthToWrite is enabled
	if len(l.authedPubkey.Load()) == 0 && (acl.Registry.GetMode() != "none" || l.Config.AuthRequired || l.Config.AuthToWrite) {
		if err = authenvelope.NewChallengeWith(l.challenge.Load()).
			Write(l); chk.E(err) {
			return
		}
	}
	// check permissions of user
	accessLevel := acl.Registry.GetAccessLevel(l.authedPubkey.Load(), l.remote)

	// If auth is required but user is not authenticated, deny access
	if l.Config.AuthRequired && len(l.authedPubkey.Load()) == 0 {
		if err = closedenvelope.NewFrom(
			env.Subscription,
			reason.AuthRequired.F("authentication required"),
		).Write(l); chk.E(err) {
			return
		}
		return
	}

	// If AuthToWrite is enabled, allow REQ without auth (but still check ACL)
	// Skip the auth requirement check for REQ when AuthToWrite is true
	if l.Config.AuthToWrite && len(l.authedPubkey.Load()) == 0 {
		// Allow unauthenticated REQ when AuthToWrite is enabled
		// but still respect ACL access levels if ACL is active
		if acl.Registry.GetMode() != "none" {
			switch accessLevel {
			case "none", "blocked", "banned":
				if err = closedenvelope.NewFrom(
					env.Subscription,
					reason.AuthRequired.F("user not authed or has no read access"),
				).Write(l); chk.E(err) {
					return
				}
				return
			}
		}
		// Allow the request to proceed without authentication
	}

	// Only check ACL access level if not already handled by AuthToWrite
	if !l.Config.AuthToWrite || len(l.authedPubkey.Load()) > 0 {
		switch accessLevel {
		case "none":
			// For REQ denial, send a CLOSED with auth-required reason (NIP-01)
			if err = closedenvelope.NewFrom(
				env.Subscription,
				reason.AuthRequired.F("user not authed or has no read access"),
			).Write(l); chk.E(err) {
				return
			}
			return
		default:
			// user has read access or better, continue
		}
	}

	// Handle NIP-43 invite request (kind 28935) - ephemeral event
	// Check if any filter requests kind 28935
	for _, f := range *env.Filters {
		if f != nil && f.Kinds != nil {
			if f.Kinds.Contains(nip43.KindInviteReq) {
				// Generate and send invite event
				inviteEvent, err := l.Server.HandleNIP43InviteRequest(l.authedPubkey.Load())
				if err != nil {
					log.W.F("failed to generate NIP-43 invite: %v", err)
					// Send EOSE and return
					if err = eoseenvelope.NewFrom(env.Subscription).Write(l); chk.E(err) {
						return err
					}
					return nil
				}

				// Send the invite event
				evEnv, _ := eventenvelope.NewResultWith(env.Subscription, inviteEvent)
				if err = evEnv.Write(l); chk.E(err) {
					return err
				}

				// Send EOSE
				if err = eoseenvelope.NewFrom(env.Subscription).Write(l); chk.E(err) {
					return err
				}

				log.I.F("sent NIP-43 invite event to %s", l.remote)
				return nil
			}
		}
	}

	// Check for NIP-XX graph queries in filters
	// Graph queries use the _graph filter extension to traverse the social graph
	for _, f := range *env.Filters {
		if f != nil && graph.IsGraphQuery(f) {
			graphQuery, graphErr := graph.ExtractFromFilter(f)
			if graphErr != nil {
				log.W.F("invalid _graph query from %s: %v", l.remote, graphErr)
				if err = closedenvelope.NewFrom(
					env.Subscription,
					reason.Error.F("invalid _graph query: %s", graphErr.Error()),
				).Write(l); chk.E(err) {
					return
				}
				return
			}
			if graphQuery != nil {
				log.I.F("graph query from %s: edge=%s dir=%s seed=%s depth=%d",
					l.remote, graphQuery.Edge, graphQuery.Direction, graphQuery.Pubkey, graphQuery.Depth)

				// Check if graph executor is available
				if l.graphExecutor == nil {
					log.W.F("graph query received but executor not initialized")
					if err = closedenvelope.NewFrom(
						env.Subscription,
						reason.Error.F("graph queries not supported on this relay"),
					).Write(l); chk.E(err) {
						return
					}
					return
				}

				// Execute the graph query
				resultEvent, execErr := l.graphExecutor.Execute(graphQuery)
				if execErr != nil {
					log.W.F("graph query execution failed from %s: %v", l.remote, execErr)
					if err = closedenvelope.NewFrom(
						env.Subscription,
						reason.Error.F("graph query failed: %s", execErr.Error()),
					).Write(l); chk.E(err) {
						return
					}
					return
				}

				// Send the result event
				var res *eventenvelope.Result
				if res, err = eventenvelope.NewResultWith(env.Subscription, resultEvent); chk.E(err) {
					return
				}
				if err = res.Write(l); chk.E(err) {
					return
				}

				// Send EOSE to signal completion
				if err = eoseenvelope.NewFrom(env.Subscription).Write(l); chk.E(err) {
					return
				}

				log.I.F("graph query completed for %s: edge=%s dir=%s, returned event kind %d",
					l.remote, graphQuery.Edge, graphQuery.Direction, resultEvent.Kind)
				return
			}
		}
	}

	// Filter out policy config events (kind 12345) for non-policy-admin users
	// Policy config events should only be visible to policy administrators
	if l.policyManager != nil && l.policyManager.IsEnabled() {
		isPolicyAdmin := l.policyManager.IsPolicyAdmin(l.authedPubkey.Load())
		if !isPolicyAdmin {
			// Remove kind 12345 from all filters
			for _, f := range *env.Filters {
				if f != nil && f.Kinds != nil && f.Kinds.Len() > 0 {
					// Create a new kinds list without PolicyConfig
					var filteredKinds []*kind.K
					for _, k := range f.Kinds.K {
						if k.K != kind.PolicyConfig.K {
							filteredKinds = append(filteredKinds, k)
						}
					}
					f.Kinds.K = filteredKinds
				}
			}
		}
	}

	var events event.S
	// Create a single context for all filter queries, isolated from the connection context
	// to prevent query timeouts from affecting the long-lived websocket connection
	queryCtx, queryCancel := context.WithTimeout(
		context.Background(), 30*time.Second,
	)
	defer queryCancel()

	// Check cache first for single-filter queries (most common case)
	// Multi-filter queries are not cached as they're more complex
	if len(*env.Filters) == 1 && env.Filters != nil {
		f := (*env.Filters)[0]
		if cachedEvents, found := l.DB.GetCachedEvents(f); found {
			log.D.F("REQ %s: cache HIT, sending %d cached events", env.Subscription, len(cachedEvents))
			// Wrap cached events with current subscription ID
			for _, ev := range cachedEvents {
				var res *eventenvelope.Result
				if res, err = eventenvelope.NewResultWith(env.Subscription, ev); chk.E(err) {
					return
				}
				if err = res.Write(l); err != nil {
					if !strings.Contains(err.Error(), "context canceled") {
						chk.E(err)
					}
					return
				}
			}
			// Send EOSE
			if err = eoseenvelope.NewFrom(env.Subscription).Write(l); chk.E(err) {
				return
			}
			// Don't create subscription for cached results with satisfied limits
			if f.Limit != nil && len(cachedEvents) >= int(*f.Limit) {
				log.D.F("REQ %s: limit satisfied by cache, not creating subscription", env.Subscription)
				return
			}
			// Fall through to create subscription for ongoing updates
		}
	}

	// Collect all events from all filters
	var allEvents event.S

	// Server-side query result limit to prevent memory exhaustion
	serverLimit := l.Config.QueryResultLimit
	if serverLimit <= 0 {
		serverLimit = 256 // Default if not configured
	}

	for _, f := range *env.Filters {
		if f != nil {
			// Enforce server-side limit on each filter
			if serverLimit > 0 {
				if f.Limit == nil {
					// No client limit - apply server limit
					limitVal := uint(serverLimit)
					f.Limit = &limitVal
				} else if int(*f.Limit) > serverLimit {
					// Client limit exceeds server limit - cap it
					limitVal := uint(serverLimit)
					f.Limit = &limitVal
				}
			}
			// Summarize filter details for diagnostics (avoid internal fields)
			var kindsLen int
			if f.Kinds != nil {
				kindsLen = f.Kinds.Len()
			}
			var authorsLen int
			if f.Authors != nil {
				authorsLen = f.Authors.Len()
			}
			var idsLen int
			if f.Ids != nil {
				idsLen = f.Ids.Len()
			}
			var dtag string
			if f.Tags != nil {
				if d := f.Tags.GetFirst([]byte("d")); d != nil {
					dtag = string(d.Value())
				}
			}
			var lim any
			if f.Limit != nil {
				lim = *f.Limit
			}
			var since any
			if f.Since != nil {
				since = f.Since.Int()
			}
			var until any
			if f.Until != nil {
				until = f.Until.Int()
			}
			log.T.C(
				func() string {
					return fmt.Sprintf(
						"REQ %s filter: kinds.len=%d authors.len=%d ids.len=%d d=%q limit=%v since=%v until=%v",
						env.Subscription, kindsLen, authorsLen, idsLen, dtag,
						lim, since, until,
					)
				},
			)

			// Process large author lists by breaking them into chunks
			if f.Authors != nil && f.Authors.Len() > 1000 {
				log.W.F("REQ %s: breaking down large author list (%d authors) into chunks", env.Subscription, f.Authors.Len())

				// Calculate chunk size to stay under message size limits
				// Each pubkey is 64 hex chars, plus JSON overhead, so ~100 bytes per author
				// Target ~50MB per chunk to stay well under 100MB limit
				chunkSize := ClientMessageSizeLimit / 200 // ~500KB per chunk
				if f.Kinds != nil && f.Kinds.Len() > 0 {
					// Reduce chunk size if there are multiple kinds to prevent too many index ranges
					chunkSize = chunkSize / f.Kinds.Len()
					if chunkSize < 100 {
						chunkSize = 100 // Minimum chunk size
					}
				}

				// Process authors in chunks
				for i := 0; i < f.Authors.Len(); i += chunkSize {
					end := i + chunkSize
					if end > f.Authors.Len() {
						end = f.Authors.Len()
					}

					// Create a chunk filter
					chunkAuthors := tag.NewFromBytesSlice(f.Authors.T[i:end]...)
					chunkFilter := &filter.F{
						Kinds:   f.Kinds,
						Authors: chunkAuthors,
						Ids:     f.Ids,
						Tags:    f.Tags,
						Since:   f.Since,
						Until:   f.Until,
						Limit:   f.Limit,
						Search:  f.Search,
					}

					log.T.F("REQ %s: processing chunk %d-%d of %d authors", env.Subscription, i+1, end, f.Authors.Len())

					// Process this chunk
					var chunkEvents event.S
					if chunkEvents, err = l.QueryEvents(queryCtx, chunkFilter); chk.E(err) {
						if errors.Is(err, badger.ErrDBClosed) {
							return
						}
						log.E.F("QueryEvents failed for chunk filter: %v", err)
						err = nil
						continue
					}

					// Add chunk results to overall results
					allEvents = append(allEvents, chunkEvents...)

					// Check if we've hit the limit
					if f.Limit != nil && len(allEvents) >= int(*f.Limit) {
						log.T.F("REQ %s: reached limit of %d events, stopping chunk processing", env.Subscription, *f.Limit)
						break
					}
				}

				// Skip the normal processing since we handled it in chunks
				continue
			}
		}
		if f != nil && pointers.Present(f.Limit) {
			if *f.Limit == 0 {
				continue
			}
		}
		var filterEvents event.S
		if filterEvents, err = l.QueryEvents(queryCtx, f); chk.E(err) {
			if errors.Is(err, badger.ErrDBClosed) {
				return
			}
			log.E.F("QueryEvents failed for filter: %v", err)
			err = nil
			continue
		}
		// Append events from this filter to the overall collection
		allEvents = append(allEvents, filterEvents...)
	}
	events = allEvents
	defer func() {
		for _, ev := range events {
			ev.Free()
		}
	}()
	var tmp event.S
	for _, ev := range events {
		// Check for private tag first
		privateTags := ev.Tags.GetAll([]byte("private"))
		if len(privateTags) > 0 && accessLevel != "admin" {
			pk := l.authedPubkey.Load()
			if pk == nil {
				continue // no auth, can't access private events
			}

			// Convert authenticated pubkey to npub for comparison
			authedNpub, err := bech32encoding.BinToNpub(pk)
			if err != nil {
				continue // couldn't convert pubkey, skip
			}

			// Check if authenticated npub is in any private tag
			authorized := false
			for _, privateTag := range privateTags {
				authorizedNpubs := strings.Split(
					string(privateTag.Value()), ",",
				)
				for _, npub := range authorizedNpubs {
					if strings.TrimSpace(npub) == string(authedNpub) {
						authorized = true
						break
					}
				}
				if authorized {
					break
				}
			}

			if !authorized {
				continue // not authorized to see this private event
			}
			// Event has private tag and user is authorized - continue to privileged check
		}

		// Filter privileged events based on kind when ACL is active
		// When ACL is "none", skip privileged filtering to allow open access
		// Privileged events should only be sent to users who are authenticated and
		// are either the event author or listed in p tags
		aclActive := acl.Registry.GetMode() != "none"
		if kind.IsPrivileged(ev.Kind) && aclActive && accessLevel != "admin" { // admins can see all events
			log.T.C(
				func() string {
					return fmt.Sprintf(
						"checking privileged event %0x", ev.ID,
					)
				},
			)
			pk := l.authedPubkey.Load()

			// Use centralized IsPartyInvolved function for consistent privilege checking
			if policy.IsPartyInvolved(ev, pk) {
				log.T.C(
					func() string {
						return fmt.Sprintf(
							"privileged event %s allowed for logged in pubkey %0x",
							ev.ID, pk,
						)
					},
				)
				tmp = append(tmp, ev)
			} else {
				log.T.C(
					func() string {
						return fmt.Sprintf(
							"privileged event %s denied for pubkey %0x (not authenticated or not a party involved)",
							ev.ID, pk,
						)
					},
				)
			}
		} else {
			// Policy-defined privileged events are handled by the policy engine
			// at line 455+. No early filtering needed here - delegate entirely to
			// the policy engine to avoid duplicate logic.
			tmp = append(tmp, ev)
		}
	}
	events = tmp

	// Apply policy filtering for read access if policy is enabled
	if l.policyManager.IsEnabled() {
		var policyFilteredEvents event.S
		for _, ev := range events {
			allowed, policyErr := l.policyManager.CheckPolicy("read", ev, l.authedPubkey.Load(), l.remote)
			if chk.E(policyErr) {
				log.E.F("policy check failed for read: %v", policyErr)
				// Default to allow on policy error
				policyFilteredEvents = append(policyFilteredEvents, ev)
				continue
			}

			if allowed {
				policyFilteredEvents = append(policyFilteredEvents, ev)
			} else {
				log.D.F("policy filtered out event %0x for read access", ev.ID)
			}
		}
		events = policyFilteredEvents
	}

	// Deduplicate events (in case chunk processing returned duplicates)
	// Use events (already filtered for privileged/policy) instead of allEvents
	if len(events) > 0 {
		seen := make(map[string]struct{})
		var deduplicatedEvents event.S
		originalCount := len(events)
		for _, ev := range events {
			eventID := hexenc.Enc(ev.ID)
			if _, exists := seen[eventID]; !exists {
				seen[eventID] = struct{}{}
				deduplicatedEvents = append(deduplicatedEvents, ev)
			}
		}
		events = deduplicatedEvents
		if originalCount != len(events) {
			log.T.F("REQ %s: deduplicated %d events to %d unique events", env.Subscription, originalCount, len(events))
		}
	}

	// Apply managed ACL filtering for read access if managed ACL is active
	if acl.Registry.GetMode() == "managed" {
		var aclFilteredEvents event.S
		for _, ev := range events {
			// Check if event is banned
			eventID := hex.EncodeToString(ev.ID)
			if banned, err := l.getManagedACL().IsEventBanned(eventID); err == nil && banned {
				log.D.F("managed ACL filtered out banned event %s", hexenc.Enc(ev.ID))
				continue
			}

			// Check if event author is banned
			authorHex := hex.EncodeToString(ev.Pubkey)
			if banned, err := l.getManagedACL().IsPubkeyBanned(authorHex); err == nil && banned {
				log.D.F("managed ACL filtered out event %s from banned pubkey %s", hexenc.Enc(ev.ID), authorHex)
				continue
			}

			// Check if event kind is allowed (only if allowed kinds are configured)
			if allowed, err := l.getManagedACL().IsKindAllowed(int(ev.Kind)); err == nil && !allowed {
				allowedKinds, err := l.getManagedACL().ListAllowedKinds()
				if err == nil && len(allowedKinds) > 0 {
					log.D.F("managed ACL filtered out event %s with disallowed kind %d", hexenc.Enc(ev.ID), ev.Kind)
					continue
				}
			}

			aclFilteredEvents = append(aclFilteredEvents, ev)
		}
		events = aclFilteredEvents
	}

	// Apply curating ACL filtering for read access if curating ACL is active
	if acl.Registry.GetMode() == "curating" {
		// Find the curating ACL instance
		for _, aclInstance := range acl.Registry.ACLs() {
			if aclInstance.Type() == "curating" {
				if curatingACL, ok := aclInstance.(*acl.Curating); ok {
					var curatingFilteredEvents event.S
					for _, ev := range events {
						if curatingACL.IsEventVisible(ev, accessLevel) {
							curatingFilteredEvents = append(curatingFilteredEvents, ev)
						} else {
							log.D.F("curating ACL filtered out event %s from blacklisted pubkey", hexenc.Enc(ev.ID))
						}
					}
					events = curatingFilteredEvents
				}
				break
			}
		}
	}

	// Apply private tag filtering - only show events with "private" tags to authorized users
	var privateFilteredEvents event.S
	authedPubkey := l.authedPubkey.Load()
	for _, ev := range events {
		// Check if event has private tags
		hasPrivateTag := false
		var privatePubkey []byte

		if ev.Tags != nil && ev.Tags.Len() > 0 {
			for _, t := range *ev.Tags {
				if t.Len() >= 2 {
					keyBytes := t.Key()
					if len(keyBytes) == 7 && string(keyBytes) == "private" {
						hasPrivateTag = true
						privatePubkey = t.Value()
						break
					}
				}
			}
		}

		// If no private tag, include the event
		if !hasPrivateTag {
			privateFilteredEvents = append(privateFilteredEvents, ev)
			continue
		}

		// Event has private tag - check if user is authorized to see it
		canSeePrivate := l.canSeePrivateEvent(authedPubkey, privatePubkey)
		if canSeePrivate {
			privateFilteredEvents = append(privateFilteredEvents, ev)
			log.D.F("private tag: allowing event %s for authorized user", hexenc.Enc(ev.ID))
		} else {
			log.D.F("private tag: filtering out event %s from unauthorized user", hexenc.Enc(ev.ID))
		}
	}
	events = privateFilteredEvents

	seen := make(map[string]struct{})
	// Cache events for single-filter queries (without subscription ID)
	shouldCache := len(*env.Filters) == 1 && len(events) > 0

	for _, ev := range events {
		log.T.C(
			func() string {
				return fmt.Sprintf(
					"REQ %s: sending EVENT id=%s kind=%d", env.Subscription,
					hexenc.Enc(ev.ID), ev.Kind,
				)
			},
		)
		log.T.C(
			func() string {
				return fmt.Sprintf("event:\n%s\n", ev.Serialize())
			},
		)
		var res *eventenvelope.Result
		if res, err = eventenvelope.NewResultWith(
			env.Subscription, ev,
		); chk.E(err) {
			return
		}

		if err = res.Write(l); err != nil {
			// Don't log context canceled errors as they're expected during shutdown
			if !strings.Contains(err.Error(), "context canceled") {
				chk.E(err)
			}
			return
		}
		// track the IDs we've sent (use hex encoding for stable key)
		seen[hexenc.Enc(ev.ID)] = struct{}{}
	}

	// Populate cache after successfully sending all events
	// Cache the events themselves (not marshaled JSON with subscription ID)
	if shouldCache && len(events) > 0 {
		f := (*env.Filters)[0]
		l.DB.CacheEvents(f, events)
		log.D.F("REQ %s: cached %d events", env.Subscription, len(events))
	}
	// write the EOSE to signal to the client that all events found have been
	// sent.
	log.T.F("sending EOSE to %s", l.remote)
	if err = eoseenvelope.NewFrom(env.Subscription).
		Write(l); chk.E(err) {
		return
	}

	// Record access for returned events (for GC access-based ranking)
	if l.accessTracker != nil && len(events) > 0 {
		go func(evts event.S, connID string) {
			defer func() {
				if r := recover(); r != nil {
					log.W.F("access tracker panic (recovered): %v", r)
				}
			}()
			for _, ev := range evts {
				// Validate event ID before calling GetSerialById
				// Check both length and capacity to catch corrupted slice headers
				if len(ev.ID) != 32 || cap(ev.ID) < 32 {
					continue
				}
				if ser, err := l.DB.GetSerialById(ev.ID); err == nil && ser != nil {
					l.accessTracker.RecordAccess(ser.Get(), connID)
				}
			}
		}(events, l.connectionID)
	}

	// Trigger archive relay query if enabled (background fetch + stream results)
	if l.archiveManager != nil && l.archiveManager.IsEnabled() && len(*env.Filters) > 0 {
		// Use first filter for archive query
		f := (*env.Filters)[0]
		go l.archiveManager.QueryArchive(
			string(env.Subscription),
			l.connectionID,
			f,
			seen,
			l, // implements EventDeliveryChannel
		)
	}

	// if the query was for just Ids, we know there can't be any more results,
	// so cancel the subscription.
	cancel := true
	log.T.F(
		"REQ %s: computing cancel/subscription; events_sent=%d",
		env.Subscription, len(events),
	)
	var subbedFilters filter.S
	for _, f := range *env.Filters {
		// Check if this filter's limit was satisfied
		limitSatisfied := false
		if pointers.Present(f.Limit) {
			if len(events) >= int(*f.Limit) {
				limitSatisfied = true
			}
		}

		if f.Ids.Len() < 1 {
			// Filter has no IDs - keep subscription open unless limit was satisfied
			if !limitSatisfied {
				cancel = false
				subbedFilters = append(subbedFilters, f)
			}
		} else {
			// remove the IDs that we already sent, as it's one less
			// comparison we have to make.
			var notFounds [][]byte
			for _, id := range f.Ids.T {
				if _, ok := seen[hexenc.Enc(id)]; ok {
					continue
				}
				notFounds = append(notFounds, id)
			}
			log.T.F(
				"REQ %s: ids outstanding=%d of %d", env.Subscription,
				len(notFounds), f.Ids.Len(),
			)
			// if all were found, don't add to subbedFilters
			if len(notFounds) == 0 {
				continue
			}
			// Check if limit was satisfied
			if limitSatisfied {
				continue
			}
			// rewrite the filter Ids to remove the ones we already sent
			f.Ids = tag.NewFromBytesSlice(notFounds...)
			// add the filter to the list of filters we're subscribing to
			cancel = false
			subbedFilters = append(subbedFilters, f)
		}
	}
	receiver := make(event.C, 32)
	// if the subscription should be cancelled, do so
	if !cancel {
		// Check global subscription limit (reduced in emergency mode)
		maxSubs := int64(l.Config.MaxSubscriptions)
		if maxSubs <= 0 {
			maxSubs = 10000
		}
		if l.rateLimiter != nil && l.rateLimiter.InEmergencyMode() {
			maxSubs = maxSubs / 10 // Restrict to 10% during emergency
			if maxSubs < 100 {
				maxSubs = 100
			}
		}
		if l.activeSubscriptionCount.Load() >= maxSubs {
			log.W.F("REQ %s: rejecting subscription (active=%d, max=%d)",
				env.Subscription, l.activeSubscriptionCount.Load(), maxSubs)
			// Send EOSE without creating subscription
			if err = eoseenvelope.NewFrom(env.Subscription).Write(l); chk.E(err) {
				return
			}
			return nil
		}
		l.activeSubscriptionCount.Add(1)

		// Create a dedicated context for this subscription that's independent of query context
		// but is child of the listener context so it gets cancelled when connection closes
		subCtx, subCancel := context.WithCancel(l.ctx)

		// Track this subscription so we can cancel it on CLOSE or connection close
		subID := string(env.Subscription)
		l.subscriptionsMu.Lock()
		if l.subscriptions == nil {
			l.subscriptions = make(map[string]context.CancelFunc)
		}
		l.subscriptions[subID] = subCancel
		l.subscriptionsMu.Unlock()

		// Register subscription with publisher
		// Set AuthRequired based on ACL mode - when ACL is "none", don't require auth for privileged events
		authRequired := acl.Registry.GetMode() != "none"
		l.publishers.Receive(
			&W{
				Conn:         l.conn,
				remote:       l.remote,
				Id:           subID,
				Receiver:     receiver,
				Filters:      &subbedFilters,
				AuthedPubkey: l.authedPubkey.Load(),
				AuthRequired: authRequired,
			},
		)

		// Launch goroutine to consume from receiver channel and forward to client
		// This is the critical missing piece - without this, the receiver channel fills up
		// and the publisher times out trying to send, causing subscription to be removed
		go func() {
			defer func() {
				// Clean up when subscription ends
				l.activeSubscriptionCount.Add(-1)
				l.subscriptionsMu.Lock()
				delete(l.subscriptions, subID)
				l.subscriptionsMu.Unlock()
				log.D.F("subscription goroutine exiting for %s @ %s", subID, l.remote)
			}()

			for {
				select {
				case <-subCtx.Done():
					// Subscription cancelled (CLOSE message or connection closing)
					log.D.F("subscription %s cancelled for %s", subID, l.remote)
					return
				case ev, ok := <-receiver:
					if !ok {
						// Channel closed - subscription ended
						log.D.F("subscription %s receiver channel closed for %s", subID, l.remote)
						return
					}

					// Forward event to client via write channel
					var res *eventenvelope.Result
					var err error
					if res, err = eventenvelope.NewResultWith(subID, ev); chk.E(err) {
						log.E.F("failed to create event envelope for subscription %s: %v", subID, err)
						continue
					}

					// Write to client - this goes through the write worker
					if err = res.Write(l); err != nil {
						if !strings.Contains(err.Error(), "context canceled") {
							log.E.F("failed to write event to subscription %s @ %s: %v", subID, l.remote, err)
						}
						// Don't return here - write errors shouldn't kill the subscription
						// The connection cleanup will handle removing the subscription
						continue
					}

					log.D.F("delivered real-time event %s to subscription %s @ %s",
						hexenc.Enc(ev.ID), subID, l.remote)
				}
			}
		}()

		log.D.F("subscription %s created and goroutine launched for %s", subID, l.remote)
	} else {
		// suppress server-sent CLOSED; client will close subscription if desired
		log.D.F("subscription request cancelled immediately (all IDs found or limit satisfied)")
	}
	log.T.F("HandleReq: COMPLETED processing from %s", l.remote)
	return
}
