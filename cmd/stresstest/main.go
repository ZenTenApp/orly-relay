package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer/p8k"
	"git.smesh.lol/orly/pkg/nostr/encoders/envelopes/eventenvelope"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/event/examples"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/kind"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/nostr/encoders/timestamp"
	"git.smesh.lol/orly/pkg/nostr/ws"
)

// randomHex returns a hex-encoded string of n random bytes (2n hex chars)
func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.Enc(b)
}

func makeEvent(rng *rand.Rand, signer *p8k.Signer) (*event.E, error) {
	ev := &event.E{
		CreatedAt: time.Now().Unix(),
		Kind:      kind.TextNote.K,
		Tags:      tag.NewS(),
		Content:   []byte(fmt.Sprintf("stresstest %d", rng.Int63())),
	}

	// Random number of p-tags up to 100
	nPTags := rng.Intn(101) // 0..100 inclusive
	for i := 0; i < nPTags; i++ {
		// random 32-byte pubkey in hex (64 chars)
		phex := randomHex(32)
		ev.Tags.Append(tag.NewFromAny("p", phex))
	}

	// Sign and verify to ensure pubkey, id and signature are coherent
	if err := ev.Sign(signer); err != nil {
		return nil, err
	}
	if ok, err := ev.Verify(); err != nil || !ok {
		return nil, fmt.Errorf("event signature verification failed: %v", err)
	}
	return ev, nil
}

type RelayConn struct {
	mu     sync.RWMutex
	client *ws.Client
	url    string
}

type CacheIndex struct {
	events  []*event.E
	ids     [][]byte
	authors [][]byte
	times   []int64
	tags    map[byte][][]byte // single-letter tag -> list of values
}

func (rc *RelayConn) Get() *ws.Client {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.client
}

func (rc *RelayConn) Reconnect(ctx context.Context) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if rc.client != nil {
		_ = rc.client.Close()
	}
	c, err := ws.RelayConnect(ctx, rc.url)
	if err != nil {
		return err
	}
	rc.client = c
	return nil
}

// loadCacheAndIndex parses examples.Cache (JSONL of events) and builds an index
func loadCacheAndIndex() (*CacheIndex, error) {
	scanner := bufio.NewScanner(bytes.NewReader(examples.Cache))
	idx := &CacheIndex{tags: make(map[byte][][]byte)}
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		ev := event.New()
		rem, err := ev.Unmarshal(line)
		_ = rem
		if err != nil {
			// skip malformed lines
			continue
		}
		idx.events = append(idx.events, ev)
		// collect fields
		if len(ev.ID) > 0 {
			idx.ids = append(idx.ids, append([]byte(nil), ev.ID...))
		}
		if len(ev.Pubkey) > 0 {
			idx.authors = append(idx.authors, append([]byte(nil), ev.Pubkey...))
		}
		idx.times = append(idx.times, ev.CreatedAt)
		if ev.Tags != nil {
			for _, tg := range *ev.Tags {
				if tg == nil || tg.Len() < 2 {
					continue
				}
				k := tg.Key()
				if len(k) != 1 {
					continue // only single-letter keys per requirement
				}
				key := k[0]
				for _, v := range tg.T[1:] {
					idx.tags[key] = append(
						idx.tags[key], append([]byte(nil), v...),
					)
				}
			}
		}
	}
	return idx, nil
}

// publishCacheEvents uploads all cache events to the relay using multiple concurrent connections
func publishCacheEvents(
	ctx context.Context, relayURL string, idx *CacheIndex,
) (sentCount int) {
	numWorkers := runtime.NumCPU()
	log.I.F("using %d concurrent connections for cache upload", numWorkers)
	
	// Channel to distribute events to workers
	eventChan := make(chan *event.E, len(idx.events))
	var totalSent atomic.Int64
	
	// Fill the event channel
	for _, ev := range idx.events {
		eventChan <- ev
	}
	close(eventChan)
	
	// Start worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			// Create separate connection for this worker
			client, err := ws.RelayConnect(ctx, relayURL)
			if err != nil {
				log.E.F("worker %d: failed to connect: %v", workerID, err)
				return
			}
			defer client.Close()
			
			rc := &RelayConn{client: client, url: relayURL}
			workerSent := 0
			
			// Process events from the channel
			for ev := range eventChan {
				select {
				case <-ctx.Done():
					return
				default:
				}
				
				// Get client connection
				wsClient := rc.Get()
				if wsClient == nil {
					if err := rc.Reconnect(ctx); err != nil {
						log.E.F("worker %d: reconnect failed: %v", workerID, err)
						continue
					}
					wsClient = rc.Get()
				}
				
				// Send event without waiting for OK response (fire-and-forget)
				envelope := eventenvelope.NewSubmissionWith(ev)
				envBytes := envelope.Marshal(nil)
				if err := <-wsClient.Write(envBytes); err != nil {
					log.E.F("worker %d: write error: %v", workerID, err)
					errStr := err.Error()
					if strings.Contains(errStr, "connection closed") {
						_ = rc.Reconnect(ctx)
					}
					time.Sleep(50 * time.Millisecond)
					continue
				}
				
				workerSent++
				totalSent.Add(1)
				log.T.F("worker %d: sent event %d (total: %d)", workerID, workerSent, totalSent.Load())
				
				// Small delay to prevent overwhelming the relay
				select {
				case <-time.After(10 * time.Millisecond):
				case <-ctx.Done():
					return
				}
			}
			
			log.I.F("worker %d: completed, sent %d events", workerID, workerSent)
		}(i)
	}
	
	// Wait for all workers to complete
	wg.Wait()
	
	return int(totalSent.Load())
}

// buildRandomFilter builds a filter combining random subsets of id, author, timestamp, and a single-letter tag value.
func buildRandomFilter(idx *CacheIndex, rng *rand.Rand, mask int) *filter.F {
	// pick a random base event as anchor for fields
	i := rng.Intn(len(idx.events))
	ev := idx.events[i]
	f := filter.New()
	// clear defaults we don't set
	f.Kinds = kind.NewS() // we don't constrain kinds
	// include fields based on mask bits: 1=id, 2=author, 4=timestamp, 8=tag
	if mask&1 != 0 {
		f.Ids.T = append(f.Ids.T, append([]byte(nil), ev.ID...))
	}
	if mask&2 != 0 {
		f.Authors.T = append(f.Authors.T, append([]byte(nil), ev.Pubkey...))
	}
	if mask&4 != 0 {
		// use a tight window around the event timestamp (exact match)
		f.Since = timestamp.FromUnix(ev.CreatedAt)
		f.Until = timestamp.FromUnix(ev.CreatedAt)
	}
	if mask&8 != 0 {
		// choose a random single-letter tag from this event if present; fallback to global index
		var key byte
		var val []byte
		chosen := false
		if ev.Tags != nil {
			for _, tg := range *ev.Tags {
				if tg == nil || tg.Len() < 2 {
					continue
				}
				k := tg.Key()
				if len(k) == 1 {
					key = k[0]
					vv := tg.T[1:]
					val = vv[rng.Intn(len(vv))]
					chosen = true
					break
				}
			}
		}
		if !chosen && len(idx.tags) > 0 {
			// pick a random entry from global tags map
			keys := make([]byte, 0, len(idx.tags))
			for k := range idx.tags {
				keys = append(keys, k)
			}
			key = keys[rng.Intn(len(keys))]
			vals := idx.tags[key]
			val = vals[rng.Intn(len(vals))]
		}
		if key != 0 && len(val) > 0 {
			f.Tags.Append(tag.NewFromBytesSlice([]byte{key}, val))
		}
	}
	return f
}

func publisherWorker(
	ctx context.Context, rc *RelayConn, id int, stats *uint64,
) {
	// Unique RNG per worker
	src := rand.NewSource(time.Now().UnixNano() ^ int64(id<<16))
	rng := rand.New(src)
	// Generate and reuse signing key per worker
	var signer *p8k.Signer
	var err error
	if signer, err = p8k.New(); err != nil {
		log.E.F("worker %d: signer create error: %v", id, err)
		return
	}
	if err := signer.Generate(); err != nil {
		log.E.F("worker %d: signer generate error: %v", id, err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		ev, err := makeEvent(rng, signer)
		if err != nil {
			log.E.F("worker %d: makeEvent error: %v", id, err)
			return
		}

		// Send event without waiting for OK response (fire-and-forget)
		client := rc.Get()
		if client == nil {
			_ = rc.Reconnect(ctx)
			continue
		}
		// Create EVENT envelope and send directly without waiting for OK
		envelope := eventenvelope.NewSubmissionWith(ev)
		envBytes := envelope.Marshal(nil)
		if err := <-client.Write(envBytes); err != nil {
			log.E.F("worker %d: write error: %v", id, err)
			errStr := err.Error()
			if strings.Contains(errStr, "connection closed") {
				for attempt := 0; attempt < 5; attempt++ {
					if ctx.Err() != nil {
						return
					}
					if err := rc.Reconnect(ctx); err == nil {
						log.I.F("worker %d: reconnected to %s", id, rc.url)
						break
					}
					select {
					case <-time.After(200 * time.Millisecond):
					case <-ctx.Done():
						return
					}
				}
			}
			// back off briefly on error to avoid tight loop if relay misbehaves
			select {
			case <-time.After(100 * time.Millisecond):
			case <-ctx.Done():
				return
			}
			continue
		}

		atomic.AddUint64(stats, 1)

		// Randomly fluctuate pacing: small random sleep 0..50ms plus occasional longer jitter
		sleep := time.Duration(rng.Intn(50)) * time.Millisecond
		if rng.Intn(10) == 0 { // 10% chance add extra 100..400ms
			sleep += time.Duration(100+rng.Intn(300)) * time.Millisecond
		}
		select {
		case <-time.After(sleep):
		case <-ctx.Done():
			return
		}
	}
}

func queryWorker(
	ctx context.Context, rc *RelayConn, idx *CacheIndex, id int,
	queries, results *uint64, subTimeout time.Duration,
	minInterval, maxInterval time.Duration,
) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano() ^ int64(id<<24)))
	mask := 1
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if len(idx.events) == 0 {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		f := buildRandomFilter(idx, rng, mask)
		mask++
		if mask > 15 { // all combinations of 4 criteria (excluding 0)
			mask = 1
		}
		client := rc.Get()
		if client == nil {
			_ = rc.Reconnect(ctx)
			continue
		}
		ff := filter.S{f}
		sCtx, cancel := context.WithTimeout(ctx, subTimeout)
		sub, err := client.Subscribe(
			sCtx, &ff, ws.WithLabel("stresstest-query"),
		)
		if err != nil {
			cancel()
			// reconnect on connection issues
			errStr := err.Error()
			if strings.Contains(errStr, "connection closed") {
				_ = rc.Reconnect(ctx)
			}
			continue
		}
		atomic.AddUint64(queries, 1)
		// read until EOSE or timeout
		innerDone := false
		for !innerDone {
			select {
			case <-sCtx.Done():
				innerDone = true
			case <-sub.EndOfStoredEvents:
				innerDone = true
			case ev, ok := <-sub.Events:
				if !ok {
					innerDone = true
					break
				}
				if ev != nil {
					atomic.AddUint64(results, 1)
				}
			}
		}
		sub.Unsub()
		cancel()
		// wait a random interval between queries
		interval := minInterval
		if maxInterval > minInterval {
			delta := rng.Int63n(int64(maxInterval - minInterval))
			interval += time.Duration(delta)
		}
		select {
		case <-time.After(interval):
		case <-ctx.Done():
			return
		}
	}
}

func startReader(ctx context.Context, rl *ws.Client, received *uint64) error {
	// Broad filter: subscribe to text notes since now-5m to catch our own writes
	f := filter.New()
	f.Kinds = kind.NewS(kind.TextNote)
	// We don't set authors to ensure we read all text notes coming in
	ff := filter.S{f}
	sub, err := rl.Subscribe(ctx, &ff, ws.WithLabel("stresstest-reader"))
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-sub.Events:
				if !ok {
					return
				}
				if ev != nil {
					atomic.AddUint64(received, 1)
				}
			}
		}
	}()

	return nil
}

func main() {
	var (
		address        string
		port           int
		workers        int
		duration       time.Duration
		publishTimeout time.Duration
		queryWorkers   int
		queryTimeout   time.Duration
		queryMinInt    time.Duration
		queryMaxInt    time.Duration
		skipCache      bool
	)

	flag.StringVar(
		&address, "address", "localhost", "relay address (host or IP)",
	)
	flag.IntVar(&port, "port", 3334, "relay port")
	flag.IntVar(
		&workers, "workers", 8, "number of concurrent publisher workers",
	)
	flag.DurationVar(
		&duration, "duration", 60*time.Second,
		"how long to run the stress test",
	)
	flag.DurationVar(
		&publishTimeout, "publish-timeout", 15*time.Second,
		"timeout waiting for OK per publish",
	)
	flag.IntVar(
		&queryWorkers, "query-workers", 4, "number of concurrent query workers",
	)
	flag.DurationVar(
		&queryTimeout, "query-timeout", 3*time.Second,
		"subscription timeout for queries",
	)
	flag.DurationVar(
		&queryMinInt, "query-min-interval", 50*time.Millisecond,
		"minimum interval between queries per worker",
	)
	flag.DurationVar(
		&queryMaxInt, "query-max-interval", 300*time.Millisecond,
		"maximum interval between queries per worker",
	)
	flag.BoolVar(
		&skipCache, "skip-cache", false,
		"skip uploading examples.Cache before running",
	)
	flag.Parse()

	relayURL := fmt.Sprintf("ws://%s:%d", address, port)
	log.I.F("stresstest: connecting to %s", relayURL)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle Ctrl+C
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt)
	go func() {
		select {
		case <-sigc:
			log.I.Ln("interrupt received, shutting down...")
			cancel()
		case <-ctx.Done():
		}
	}()

	rl, err := ws.RelayConnect(ctx, relayURL)
	if err != nil {
		log.E.F("failed to connect to relay %s: %v", relayURL, err)
		os.Exit(1)
	}
	defer rl.Close()

	rc := &RelayConn{client: rl, url: relayURL}

	// Load and publish cache events first (unless skipped)
	idx, err := loadCacheAndIndex()
	if err != nil {
		log.E.F("failed to load examples.Cache: %v", err)
	}
	cacheSent := 0
	if !skipCache && idx != nil && len(idx.events) > 0 {
		log.I.F("sending %d events from examples.Cache...", len(idx.events))
		cacheSent = publishCacheEvents(ctx, relayURL, idx)
		log.I.F("sent %d/%d cache events", cacheSent, len(idx.events))
	}

	var pubOK uint64
	var recvCount uint64
	var qCount uint64
	var qResults uint64

	if err := startReader(ctx, rl, &recvCount); err != nil {
		log.E.F("reader subscribe error: %v", err)
		// continue anyway, we can still write
	}

	wg := sync.WaitGroup{}
	// Start publisher workers
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		i := i
		go func() {
			defer wg.Done()
			publisherWorker(ctx, rc, i, &pubOK)
		}()
	}
	// Start query workers
	if idx != nil && len(idx.events) > 0 && queryWorkers > 0 {
		wg.Add(queryWorkers)
		for i := 0; i < queryWorkers; i++ {
			i := i
			go func() {
				defer wg.Done()
				queryWorker(
					ctx, rc, idx, i, &qCount, &qResults, queryTimeout,
					queryMinInt, queryMaxInt,
				)
			}()
		}
	}

	// Timer for duration and periodic stats
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	end := time.NewTimer(duration)
	start := time.Now()

loop:
	for {
		select {
		case <-ticker.C:
			elapsed := time.Since(start).Seconds()
			p := atomic.LoadUint64(&pubOK)
			r := atomic.LoadUint64(&recvCount)
			qc := atomic.LoadUint64(&qCount)
			qr := atomic.LoadUint64(&qResults)
			log.I.F(
				"elapsed=%.1fs sent=%d (%.0f/s) received=%d cache_sent=%d queries=%d results=%d",
				elapsed, p, float64(p)/elapsed, r, cacheSent, qc, qr,
			)
		case <-end.C:
			break loop
		case <-ctx.Done():
			break loop
		}
	}

	cancel()
	wg.Wait()
	p := atomic.LoadUint64(&pubOK)
	r := atomic.LoadUint64(&recvCount)
	qc := atomic.LoadUint64(&qCount)
	qr := atomic.LoadUint64(&qResults)
	log.I.F(
		"stresstest complete: cache_sent=%d sent=%d received=%d queries=%d results=%d duration=%s",
		cacheSent, p, r, qc, qr,
		time.Since(start).Truncate(time.Millisecond),
	)
}
