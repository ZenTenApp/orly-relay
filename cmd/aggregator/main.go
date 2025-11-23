package main

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
	"github.com/minio/sha256-simd"
	"git.mleku.dev/mleku/nostr/encoders/bech32encoding"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"git.mleku.dev/mleku/nostr/encoders/timestamp"
	"git.mleku.dev/mleku/nostr/interfaces/signer"
	"git.mleku.dev/mleku/nostr/ws"
)

const (
	// Bloom filter parameters for ~0.1% false positive rate with 1M events
	bloomFilterBits      = 14377588 // ~1.75MB for 1M events at 0.1% FPR
	bloomFilterHashFuncs = 10       // Optimal number of hash functions
	maxMemoryMB          = 256      // Maximum memory usage in MB
	memoryCheckInterval  = 30 * time.Second

	// Rate limiting parameters
	baseRetryDelay = 1 * time.Second
	maxRetryDelay  = 60 * time.Second
	maxRetries     = 5
	batchSize      = time.Hour * 24 * 7 // 1 week batches

	// Timeout parameters
	maxRunTime           = 30 * time.Minute // Maximum total runtime
	relayTimeout         = 5 * time.Minute  // Timeout per relay
	stuckProgressTimeout = 2 * time.Minute  // Timeout if no progress is made
)

var relays = []string{
	"wss://nostr.wine/",
	"wss://nostr.land/",
	"wss://orly-relay.imwald.eu",
	"wss://relay.orly.dev/",
	"wss://relay.damus.io/",
	"wss://nos.lol/",
	"wss://theforest.nostr1.com/",
}

// BloomFilter implements a memory-efficient bloom filter for event deduplication
type BloomFilter struct {
	bits     []byte
	size     uint32
	hashFunc int
	mutex    sync.RWMutex
}

// NewBloomFilter creates a new bloom filter with specified parameters
func NewBloomFilter(bits uint32, hashFuncs int) *BloomFilter {
	return &BloomFilter{
		bits:     make([]byte, (bits+7)/8), // Round up to nearest byte
		size:     bits,
		hashFunc: hashFuncs,
	}
}

// hash generates multiple hash values for a given input using SHA256
func (bf *BloomFilter) hash(data []byte) []uint32 {
	hashes := make([]uint32, bf.hashFunc)

	// Use SHA256 as base hash
	baseHash := sha256.Sum256(data)

	// Generate multiple hash values by combining with different salts
	for i := 0; i < bf.hashFunc; i++ {
		// Create salt by appending index
		saltedData := make([]byte, len(baseHash)+4)
		copy(saltedData, baseHash[:])
		binary.LittleEndian.PutUint32(saltedData[len(baseHash):], uint32(i))

		// Hash the salted data
		h := sha256.Sum256(saltedData)

		// Convert first 4 bytes to uint32 and mod by filter size
		hashVal := binary.LittleEndian.Uint32(h[:4])
		hashes[i] = hashVal % bf.size
	}

	return hashes
}

// Add adds an item to the bloom filter
func (bf *BloomFilter) Add(data []byte) {
	bf.mutex.Lock()
	defer bf.mutex.Unlock()

	hashes := bf.hash(data)
	for _, h := range hashes {
		byteIndex := h / 8
		bitIndex := h % 8
		bf.bits[byteIndex] |= 1 << bitIndex
	}
}

// Contains checks if an item might be in the bloom filter
func (bf *BloomFilter) Contains(data []byte) bool {
	bf.mutex.RLock()
	defer bf.mutex.RUnlock()

	hashes := bf.hash(data)
	for _, h := range hashes {
		byteIndex := h / 8
		bitIndex := h % 8
		if bf.bits[byteIndex]&(1<<bitIndex) == 0 {
			return false
		}
	}
	return true
}

// EstimatedItems estimates the number of items added to the filter
func (bf *BloomFilter) EstimatedItems() uint32 {
	bf.mutex.RLock()
	defer bf.mutex.RUnlock()

	// Count set bits
	setBits := uint32(0)
	for _, b := range bf.bits {
		for i := 0; i < 8; i++ {
			if b&(1<<i) != 0 {
				setBits++
			}
		}
	}

	// Estimate items using bloom filter formula: n ≈ -(m/k) * ln(1 - X/m)
	// where m = filter size, k = hash functions, X = set bits
	if setBits == 0 {
		return 0
	}

	m := float64(bf.size)
	k := float64(bf.hashFunc)
	x := float64(setBits)

	if x >= m {
		return uint32(m / k) // Saturated filter
	}

	estimated := -(m / k) * math.Log(1-(x/m))
	return uint32(estimated)
}

// MemoryUsage returns the memory usage in bytes
func (bf *BloomFilter) MemoryUsage() int {
	return len(bf.bits)
}

// ToBase64 serializes the bloom filter to a base64 encoded string
func (bf *BloomFilter) ToBase64() string {
	bf.mutex.RLock()
	defer bf.mutex.RUnlock()

	// Create a serialization format: [size:4][hashFunc:4][bits:variable]
	serialized := make([]byte, 8+len(bf.bits))

	// Write size (4 bytes)
	binary.LittleEndian.PutUint32(serialized[0:4], bf.size)

	// Write hash function count (4 bytes)
	binary.LittleEndian.PutUint32(serialized[4:8], uint32(bf.hashFunc))

	// Write bits data
	copy(serialized[8:], bf.bits)

	return base64.StdEncoding.EncodeToString(serialized)
}

// FromBase64 deserializes a bloom filter from a base64 encoded string
func FromBase64(encoded string) (*BloomFilter, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	if len(data) < 8 {
		return nil, fmt.Errorf("invalid bloom filter data: too short")
	}

	// Read size (4 bytes)
	size := binary.LittleEndian.Uint32(data[0:4])

	// Read hash function count (4 bytes)
	hashFunc := int(binary.LittleEndian.Uint32(data[4:8]))

	// Read bits data
	bits := make([]byte, len(data)-8)
	copy(bits, data[8:])

	// Validate that the bits length matches the expected size
	expectedBytesLen := (size + 7) / 8
	if uint32(len(bits)) != expectedBytesLen {
		return nil, fmt.Errorf("invalid bloom filter data: bits length mismatch")
	}

	return &BloomFilter{
		bits:     bits,
		size:     size,
		hashFunc: hashFunc,
	}, nil
}

// RelayState tracks the state and rate limiting for each relay
type RelayState struct {
	url           string
	retryCount    int
	nextRetryTime time.Time
	rateLimited   bool
	completed     bool
	mutex         sync.RWMutex
}

// TimeWindow represents a time range for progressive fetching
type TimeWindow struct {
	since *timestamp.T
	until *timestamp.T
}

// CompletionTracker tracks which relay-time window combinations have been completed
type CompletionTracker struct {
	completed map[string]map[string]bool // relay -> timewindow -> completed
	mutex     sync.RWMutex
}

func NewCompletionTracker() *CompletionTracker {
	return &CompletionTracker{
		completed: make(map[string]map[string]bool),
	}
}

func (ct *CompletionTracker) MarkCompleted(relayURL string, timeWindow string) {
	ct.mutex.Lock()
	defer ct.mutex.Unlock()

	if ct.completed[relayURL] == nil {
		ct.completed[relayURL] = make(map[string]bool)
	}
	ct.completed[relayURL][timeWindow] = true
}

func (ct *CompletionTracker) IsCompleted(relayURL string, timeWindow string) bool {
	ct.mutex.RLock()
	defer ct.mutex.RUnlock()

	if ct.completed[relayURL] == nil {
		return false
	}
	return ct.completed[relayURL][timeWindow]
}

func (ct *CompletionTracker) GetCompletionStatus() (completed, total int) {
	ct.mutex.RLock()
	defer ct.mutex.RUnlock()

	for _, windows := range ct.completed {
		for _, isCompleted := range windows {
			total++
			if isCompleted {
				completed++
			}
		}
	}
	return
}

type Aggregator struct {
	npub              string
	pubkeyBytes       []byte
	seenEvents        *BloomFilter
	seenRelays        map[string]bool
	relayQueue        chan string
	relayMutex        sync.RWMutex
	ctx               context.Context
	cancel            context.CancelFunc
	since             *timestamp.T
	until             *timestamp.T
	wg                sync.WaitGroup
	progressiveEnd    *timestamp.T
	memoryTicker      *time.Ticker
	eventCount        uint64
	relayStates       map[string]*RelayState
	relayStatesMutex  sync.RWMutex
	completionTracker *CompletionTracker
	timeWindows       []TimeWindow
	// Track actual time range of processed events
	actualSince *timestamp.T
	actualUntil *timestamp.T
	timeMutex   sync.RWMutex
	// Bloom filter file for loading existing state
	bloomFilterFile string
	appendMode      bool
	// Progress tracking for timeout detection
	startTime        time.Time
	lastProgress     int
	lastProgressTime time.Time
	progressMutex    sync.RWMutex
	// Authentication support
	signer        signer.I // Optional signer for relay authentication
	hasPrivateKey bool     // Whether we have a private key for auth
}

func NewAggregator(keyInput string, since, until *timestamp.T, bloomFilterFile string) (agg *Aggregator, err error) {
	var pubkeyBytes []byte
	var signer signer.I
	var hasPrivateKey bool

	// Determine if input is nsec (private key) or npub (public key)
	if strings.HasPrefix(keyInput, "nsec") {
		// Handle nsec (private key) - derive pubkey and enable authentication
		var secretBytes []byte
		if secretBytes, err = bech32encoding.NsecToBytes(keyInput); chk.E(err) {
			return nil, fmt.Errorf("failed to decode nsec: %w", err)
		}

		// Create signer from private key
		var signerErr error
		if signer, signerErr = p8k.New(); signerErr != nil {
			return nil, fmt.Errorf("failed to create signer: %w", signerErr)
		}
		if err = signer.InitSec(secretBytes); chk.E(err) {
			return nil, fmt.Errorf("failed to initialize signer: %w", err)
		}

		// Get public key from signer
		pubkeyBytes = signer.Pub()
		hasPrivateKey = true

		log.I.F("using private key (nsec) - authentication enabled")
	} else if strings.HasPrefix(keyInput, "npub") {
		// Handle npub (public key only) - no authentication
		if pubkeyBytes, err = bech32encoding.NpubToBytes(keyInput); chk.E(err) {
			return nil, fmt.Errorf("failed to decode npub: %w", err)
		}
		hasPrivateKey = false

		log.I.F("using public key (npub) - authentication disabled")
	} else {
		return nil, fmt.Errorf("key input must start with 'nsec' or 'npub', got: %s", keyInput[:4])
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Set progressive end to current time if until is not specified
	progressiveEnd := until
	if progressiveEnd == nil {
		progressiveEnd = timestamp.Now()
	}

	// Initialize bloom filter - either new or loaded from file
	var bloomFilter *BloomFilter
	var appendMode bool

	if bloomFilterFile != "" {
		// Try to load existing bloom filter
		if bloomFilter, err = loadBloomFilterFromFile(bloomFilterFile); err != nil {
			log.W.F("failed to load bloom filter from %s: %v, creating new filter", bloomFilterFile, err)
			bloomFilter = NewBloomFilter(bloomFilterBits, bloomFilterHashFuncs)
		} else {
			log.I.F("loaded existing bloom filter from %s", bloomFilterFile)
			appendMode = true
		}
	} else {
		bloomFilter = NewBloomFilter(bloomFilterBits, bloomFilterHashFuncs)
	}

	agg = &Aggregator{
		npub:              keyInput,
		pubkeyBytes:       pubkeyBytes,
		seenEvents:        bloomFilter,
		seenRelays:        make(map[string]bool),
		relayQueue:        make(chan string, 100),
		ctx:               ctx,
		cancel:            cancel,
		since:             since,
		until:             until,
		progressiveEnd:    progressiveEnd,
		memoryTicker:      time.NewTicker(memoryCheckInterval),
		eventCount:        0,
		relayStates:       make(map[string]*RelayState),
		completionTracker: NewCompletionTracker(),
		bloomFilterFile:   bloomFilterFile,
		appendMode:        appendMode,
		startTime:         time.Now(),
		lastProgress:      0,
		lastProgressTime:  time.Now(),
		signer:            signer,
		hasPrivateKey:     hasPrivateKey,
	}

	// Calculate time windows for progressive fetching
	agg.calculateTimeWindows()

	// Add initial relays to queue
	for _, relayURL := range relays {
		agg.addRelay(relayURL)
	}

	return
}

// loadBloomFilterFromFile loads a bloom filter from a file containing base64 encoded data
func loadBloomFilterFromFile(filename string) (*BloomFilter, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Find the base64 data between the markers
	content := string(data)
	startMarker := "Bloom filter (base64):\n"
	endMarker := "\n=== END BLOOM FILTER ==="

	startIdx := strings.Index(content, startMarker)
	if startIdx == -1 {
		return nil, fmt.Errorf("bloom filter start marker not found")
	}
	startIdx += len(startMarker)

	endIdx := strings.Index(content[startIdx:], endMarker)
	if endIdx == -1 {
		return nil, fmt.Errorf("bloom filter end marker not found")
	}

	base64Data := strings.TrimSpace(content[startIdx : startIdx+endIdx])
	return FromBase64(base64Data)
}

// updateActualTimeRange updates the actual time range of processed events
func (a *Aggregator) updateActualTimeRange(eventTime *timestamp.T) {
	a.timeMutex.Lock()
	defer a.timeMutex.Unlock()

	if a.actualSince == nil || eventTime.I64() < a.actualSince.I64() {
		a.actualSince = eventTime
	}

	if a.actualUntil == nil || eventTime.I64() > a.actualUntil.I64() {
		a.actualUntil = eventTime
	}
}

// getActualTimeRange returns the actual time range of processed events
func (a *Aggregator) getActualTimeRange() (since, until *timestamp.T) {
	a.timeMutex.RLock()
	defer a.timeMutex.RUnlock()
	return a.actualSince, a.actualUntil
}

// calculateTimeWindows pre-calculates all time windows for progressive fetching
func (a *Aggregator) calculateTimeWindows() {
	if a.since == nil {
		// If no since time, we'll just work backwards from progressiveEnd
		// We can't pre-calculate windows without a start time
		return
	}

	var windows []TimeWindow
	currentUntil := a.progressiveEnd

	for currentUntil.I64() > a.since.I64() {
		currentSince := timestamp.FromUnix(currentUntil.I64() - int64(batchSize.Seconds()))
		if currentSince.I64() < a.since.I64() {
			currentSince = a.since
		}

		windows = append(windows, TimeWindow{
			since: currentSince,
			until: currentUntil,
		})

		currentUntil = currentSince
		if currentUntil.I64() <= a.since.I64() {
			break
		}
	}

	a.timeWindows = windows
	log.I.F("calculated %d time windows for progressive fetching", len(windows))
}

// getOrCreateRelayState gets or creates a relay state for rate limiting
func (a *Aggregator) getOrCreateRelayState(relayURL string) *RelayState {
	a.relayStatesMutex.Lock()
	defer a.relayStatesMutex.Unlock()

	if state, exists := a.relayStates[relayURL]; exists {
		return state
	}

	state := &RelayState{
		url:           relayURL,
		retryCount:    0,
		nextRetryTime: time.Now(),
		rateLimited:   false,
		completed:     false,
	}
	a.relayStates[relayURL] = state
	return state
}

// shouldRetryRelay checks if a relay should be retried based on rate limiting
func (a *Aggregator) shouldRetryRelay(relayURL string) bool {
	state := a.getOrCreateRelayState(relayURL)
	state.mutex.RLock()
	defer state.mutex.RUnlock()

	if state.completed {
		return false
	}

	if state.rateLimited && time.Now().Before(state.nextRetryTime) {
		return false
	}

	return state.retryCount < maxRetries
}

// markRelayRateLimited marks a relay as rate limited and sets retry time
func (a *Aggregator) markRelayRateLimited(relayURL string) {
	state := a.getOrCreateRelayState(relayURL)
	state.mutex.Lock()
	defer state.mutex.Unlock()

	state.rateLimited = true
	state.retryCount++

	if state.retryCount >= maxRetries {
		log.W.F("relay %s permanently failed after %d retries", relayURL, maxRetries)
		state.completed = true // Mark as completed to exclude from future attempts
		return
	}

	// Exponential backoff with jitter
	delay := time.Duration(float64(baseRetryDelay) * math.Pow(2, float64(state.retryCount-1)))
	if delay > maxRetryDelay {
		delay = maxRetryDelay
	}

	state.nextRetryTime = time.Now().Add(delay)
	log.W.F("relay %s rate limited, retry %d/%d in %v", relayURL, state.retryCount, maxRetries, delay)
}

// markRelayCompleted marks a relay as completed for all time windows
func (a *Aggregator) markRelayCompleted(relayURL string) {
	state := a.getOrCreateRelayState(relayURL)
	state.mutex.Lock()
	defer state.mutex.Unlock()

	state.completed = true
	log.I.F("relay %s marked as completed", relayURL)
}

// checkAllCompleted checks if all relay-time window combinations are completed
func (a *Aggregator) checkAllCompleted() bool {
	if len(a.timeWindows) == 0 {
		// If no time windows calculated, we can't determine completion
		return false
	}

	a.relayStatesMutex.RLock()
	allRelays := make([]string, 0, len(a.relayStates))
	for relayURL := range a.relayStates {
		allRelays = append(allRelays, relayURL)
	}
	a.relayStatesMutex.RUnlock()

	// Check if all relay-time window combinations are completed
	totalCombinations := len(allRelays) * len(a.timeWindows)
	completedCombinations := 0
	availableCombinations := 0 // Combinations from relays that haven't permanently failed

	for _, relayURL := range allRelays {
		state := a.getOrCreateRelayState(relayURL)
		state.mutex.RLock()
		isRelayFailed := state.retryCount >= maxRetries
		state.mutex.RUnlock()

		for _, window := range a.timeWindows {
			windowKey := fmt.Sprintf("%d-%d", window.since.I64(), window.until.I64())
			if a.completionTracker.IsCompleted(relayURL, windowKey) {
				completedCombinations++
			}

			// Only count combinations from relays that haven't permanently failed
			if !isRelayFailed {
				availableCombinations++
			}
		}
	}

	// Update progress tracking
	a.progressMutex.Lock()
	if completedCombinations > a.lastProgress {
		a.lastProgress = completedCombinations
		a.lastProgressTime = time.Now()
	}
	a.progressMutex.Unlock()

	if totalCombinations > 0 {
		progress := float64(completedCombinations) / float64(totalCombinations) * 100
		log.I.F("completion progress: %d/%d (%.1f%%) - available: %d", completedCombinations, totalCombinations, progress, availableCombinations)

		// Consider complete if we've finished all available combinations (excluding permanently failed relays)
		if availableCombinations > 0 {
			return completedCombinations >= availableCombinations
		}
		return completedCombinations == totalCombinations
	}

	return false
}

func (a *Aggregator) isEventSeen(eventID string) (seen bool) {
	return a.seenEvents.Contains([]byte(eventID))
}

func (a *Aggregator) markEventSeen(eventID string) {
	a.seenEvents.Add([]byte(eventID))
	a.eventCount++
}

func (a *Aggregator) addRelay(relayURL string) {
	a.relayMutex.Lock()
	defer a.relayMutex.Unlock()

	if !a.seenRelays[relayURL] {
		a.seenRelays[relayURL] = true
		select {
		case a.relayQueue <- relayURL:
			log.I.F("added new relay to queue: %s", relayURL)
		default:
			log.W.F("relay queue full, skipping: %s", relayURL)
		}
	}
}

func (a *Aggregator) processRelayListEvent(ev *event.E) {
	// Extract relay URLs from "r" tags in kind 10002 events
	if ev.Kind != 10002 { // RelayListMetadata
		return
	}

	log.I.F("processing relay list event from %s", hex.Enc(ev.Pubkey))

	for _, tag := range ev.Tags.GetAll([]byte("r")) {
		if len(tag.T) >= 2 {
			relayURL := string(tag.T[1])
			if relayURL != "" {
				log.I.F("discovered relay from relay list: %s", relayURL)
				a.addRelay(relayURL)
			}
		}
	}
}

func (a *Aggregator) outputEvent(ev *event.E) (err error) {
	// Convert event to JSON and output to stdout
	var jsonBytes []byte
	if jsonBytes, err = json.Marshal(map[string]interface{}{
		"id":         hex.Enc(ev.ID),
		"pubkey":     hex.Enc(ev.Pubkey),
		"created_at": ev.CreatedAt,
		"kind":       ev.Kind,
		"tags":       ev.Tags,
		"content":    string(ev.Content),
		"sig":        hex.Enc(ev.Sig),
	}); chk.E(err) {
		return fmt.Errorf("failed to marshal event to JSON: %w", err)
	}

	fmt.Println(string(jsonBytes))
	return
}

func (a *Aggregator) connectToRelay(relayURL string) {
	defer func() {
		log.I.F("relay connection finished: %s", relayURL)
		a.wg.Done()
	}()

	log.I.F("connecting to relay: %s", relayURL)

	// Create context with timeout for connection
	connCtx, connCancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer connCancel()

	// Connect to relay
	var client *ws.Client
	var err error
	if client, err = ws.RelayConnect(connCtx, relayURL); chk.E(err) {
		log.E.F("failed to connect to relay %s: %v", relayURL, err)
		return
	}
	defer client.Close()

	log.I.F("connected to relay: %s", relayURL)

	// Attempt authentication if we have a private key
	if a.hasPrivateKey && a.signer != nil {
		authCtx, authCancel := context.WithTimeout(a.ctx, 5*time.Second)
		defer authCancel()

		if err = client.Auth(authCtx, a.signer); err != nil {
			log.W.F("authentication failed for relay %s: %v", relayURL, err)
			// Continue without authentication - some relays may not require it
		} else {
			log.I.F("successfully authenticated to relay: %s", relayURL)
		}
	}

	// Perform progressive backward fetching
	a.progressiveFetch(client, relayURL)
}

func (a *Aggregator) progressiveFetch(client *ws.Client, relayURL string) {
	// Check if relay should be retried
	if !a.shouldRetryRelay(relayURL) {
		log.W.F("skipping relay %s due to rate limiting or max retries", relayURL)
		return
	}

	// Create hex-encoded pubkey for p-tags
	pubkeyHex := hex.Enc(a.pubkeyBytes)

	// Use pre-calculated time windows if available, otherwise calculate on the fly
	var windows []TimeWindow
	if len(a.timeWindows) > 0 {
		windows = a.timeWindows
	} else {
		// Fallback to dynamic calculation for unlimited time ranges
		currentUntil := a.progressiveEnd
		for {
			currentSince := timestamp.FromUnix(currentUntil.I64() - int64(batchSize.Seconds()))
			if a.since != nil && currentSince.I64() < a.since.I64() {
				currentSince = a.since
			}

			windows = append(windows, TimeWindow{
				since: currentSince,
				until: currentUntil,
			})

			currentUntil = currentSince
			if a.since != nil && currentUntil.I64() <= a.since.I64() {
				break
			}

			// Prevent infinite loops for unlimited ranges
			if len(windows) > 1000 {
				log.W.F("limiting to 1000 time windows for relay %s", relayURL)
				break
			}
		}
	}

	// Process each time window
	for _, window := range windows {
		windowKey := fmt.Sprintf("%d-%d", window.since.I64(), window.until.I64())

		// Skip if already completed
		if a.completionTracker.IsCompleted(relayURL, windowKey) {
			continue
		}

		select {
		case <-a.ctx.Done():
			log.I.F("context cancelled, stopping progressive fetch for relay %s", relayURL)
			return
		default:
		}

		log.I.F("fetching batch from %s: %d to %d", relayURL, window.since.I64(), window.until.I64())

		// Try to fetch this time window with retry logic
		success := a.fetchTimeWindow(client, relayURL, window, pubkeyHex)

		if success {
			// Mark this time window as completed for this relay
			a.completionTracker.MarkCompleted(relayURL, windowKey)
		} else {
			// If fetch failed, mark relay as rate limited and return
			a.markRelayRateLimited(relayURL)
			return
		}
	}

	// Mark relay as completed for all time windows
	a.markRelayCompleted(relayURL)
	log.I.F("completed all time windows for relay %s", relayURL)
}

func (a *Aggregator) fetchTimeWindow(client *ws.Client, relayURL string, window TimeWindow, pubkeyHex string) bool {
	// Create filters for this time batch
	f1 := &filter.F{
		Authors: tag.NewFromBytesSlice(a.pubkeyBytes),
		Since:   window.since,
		Until:   window.until,
	}

	f2 := &filter.F{
		Tags:  tag.NewSWithCap(1),
		Since: window.since,
		Until: window.until,
	}
	pTag := tag.NewFromAny("p", pubkeyHex)
	f2.Tags.Append(pTag)

	// Add relay list filter to discover new relays
	f3 := &filter.F{
		Authors: tag.NewFromBytesSlice(a.pubkeyBytes),
		Kinds:   kind.NewS(kind.New(10002)), // RelayListMetadata
		Since:   window.since,
		Until:   window.until,
	}

	// Subscribe to events using all filters with a dedicated context and timeout
	// Use a longer timeout to avoid premature cancellation by completion monitor
	subCtx, subCancel := context.WithTimeout(context.Background(), 10*time.Minute)

	var sub *ws.Subscription
	var err error
	if sub, err = client.Subscribe(subCtx, filter.NewS(f1, f2, f3)); chk.E(err) {
		subCancel() // Cancel context on error
		log.E.F("failed to subscribe to relay %s: %v", relayURL, err)
		return false
	}

	// Ensure subscription is cleaned up when we're done
	defer func() {
		sub.Unsub()
		subCancel()
	}()

	log.I.F("subscribed to batch from %s for pubkey %s (authored by, mentioning, and relay lists)", relayURL, a.npub)

	// Process events for this batch
	batchComplete := false
	rateLimited := false

	for !batchComplete && !rateLimited {
		select {
		case <-a.ctx.Done():
			log.I.F("aggregator context cancelled, stopping batch for relay %s", relayURL)
			return false
		case <-subCtx.Done():
			log.W.F("subscription timeout for relay %s", relayURL)
			return false
		case ev := <-sub.Events:
			if ev == nil {
				log.I.F("event channel closed for relay %s", relayURL)
				return false
			}

			eventID := hex.Enc(ev.ID)

			// Check if we've already seen this event
			if a.isEventSeen(eventID) {
				continue
			}

			// Mark event as seen
			a.markEventSeen(eventID)

			// Update actual time range
			a.updateActualTimeRange(timestamp.FromUnix(ev.CreatedAt))

			// Process relay list events to discover new relays
			if ev.Kind == 10002 {
				a.processRelayListEvent(ev)
			}

			// Output event to stdout
			if err = a.outputEvent(ev); chk.E(err) {
				log.E.F("failed to output event: %v", err)
			}
		case <-sub.EndOfStoredEvents:
			log.I.F("end of stored events for batch on relay %s", relayURL)
			batchComplete = true
		case reason := <-sub.ClosedReason:
			reasonStr := string(reason)
			log.W.F("subscription closed for relay %s: %s", relayURL, reasonStr)

			// Check for rate limiting messages
			if a.isRateLimitMessage(reasonStr) {
				log.W.F("detected rate limiting from relay %s", relayURL)
				rateLimited = true
			}

			sub.Unsub()
			return !rateLimited
			// Note: NOTICE messages are handled at the client level, not subscription level
			// Rate limiting detection will primarily rely on CLOSED messages
		}
	}

	sub.Unsub()
	return !rateLimited
}

// isRateLimitMessage checks if a message indicates rate limiting
func (a *Aggregator) isRateLimitMessage(message string) bool {
	message = strings.ToLower(message)
	rateLimitIndicators := []string{
		"too many",
		"rate limit",
		"slow down",
		"concurrent req",
		"throttle",
		"backoff",
	}

	for _, indicator := range rateLimitIndicators {
		if strings.Contains(message, indicator) {
			return true
		}
	}
	return false
}

func (a *Aggregator) Start() (err error) {
	log.I.F("starting aggregator for key: %s", a.npub)
	log.I.F("pubkey bytes: %s", hex.Enc(a.pubkeyBytes))
	log.I.F("bloom filter: %d bits (%.2fMB), %d hash functions, ~0.1%% false positive rate",
		bloomFilterBits, float64(a.seenEvents.MemoryUsage())/1024/1024, bloomFilterHashFuncs)

	// Start memory monitoring goroutine
	go a.memoryMonitor()

	// Start relay processor goroutine
	go a.processRelayQueue()

	// Start completion monitoring goroutine
	go a.completionMonitor()

	// Add initial relay count to wait group
	a.wg.Add(len(relays))
	log.I.F("waiting for %d initial relay connections to complete", len(relays))

	// Wait for all relay connections to finish OR completion
	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.I.F("all relay connections completed")
	case <-a.ctx.Done():
		log.I.F("aggregator terminated due to completion")
	}

	// Stop memory monitoring
	a.memoryTicker.Stop()

	// Output bloom filter summary
	a.outputBloomFilter()

	log.I.F("aggregator finished")
	return
}

// completionMonitor periodically checks if all work is completed and terminates if so
func (a *Aggregator) completionMonitor() {
	ticker := time.NewTicker(10 * time.Second) // Check every 10 seconds
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			// Check for various termination conditions
			if a.shouldTerminate() {
				return
			}

			// Also check for rate-limited relays that can be retried
			a.retryRateLimitedRelays()
		}
	}
}

// shouldTerminate checks various conditions that should cause the aggregator to terminate
func (a *Aggregator) shouldTerminate() bool {
	now := time.Now()

	// Check if all work is completed
	if a.checkAllCompleted() {
		log.I.F("all relay-time window combinations completed, terminating aggregator")
		a.cancel()
		return true
	}

	// Check for maximum runtime timeout
	if now.Sub(a.startTime) > maxRunTime {
		log.W.F("maximum runtime (%v) exceeded, terminating aggregator", maxRunTime)
		a.cancel()
		return true
	}

	// Check for stuck progress timeout
	a.progressMutex.RLock()
	timeSinceProgress := now.Sub(a.lastProgressTime)
	a.progressMutex.RUnlock()

	if timeSinceProgress > stuckProgressTimeout {
		log.W.F("no progress made for %v, terminating aggregator", timeSinceProgress)
		a.cancel()
		return true
	}

	return false
}

// retryRateLimitedRelays checks for rate-limited relays that can be retried
func (a *Aggregator) retryRateLimitedRelays() {
	a.relayStatesMutex.RLock()
	defer a.relayStatesMutex.RUnlock()

	for relayURL, state := range a.relayStates {
		state.mutex.RLock()
		canRetry := state.rateLimited &&
			time.Now().After(state.nextRetryTime) &&
			state.retryCount < maxRetries &&
			!state.completed
		state.mutex.RUnlock()

		if canRetry {
			log.I.F("retrying rate-limited relay: %s", relayURL)

			// Reset rate limiting status
			state.mutex.Lock()
			state.rateLimited = false
			state.mutex.Unlock()

			// Add back to queue for retry
			select {
			case a.relayQueue <- relayURL:
				a.wg.Add(1)
			default:
				log.W.F("relay queue full, skipping retry for %s", relayURL)
			}
		}
	}
}

func (a *Aggregator) processRelayQueue() {
	initialRelayCount := len(relays)
	processedInitial := 0

	for {
		select {
		case <-a.ctx.Done():
			log.I.F("relay queue processor stopping")
			return
		case relayURL := <-a.relayQueue:
			log.I.F("processing relay from queue: %s", relayURL)

			// For dynamically discovered relays (after initial ones), add to wait group
			if processedInitial >= initialRelayCount {
				a.wg.Add(1)
			} else {
				processedInitial++
			}

			go a.connectToRelay(relayURL)
		}
	}
}

func (a *Aggregator) Stop() {
	a.cancel()
	if a.memoryTicker != nil {
		a.memoryTicker.Stop()
	}
}

// outputBloomFilter outputs the bloom filter as base64 to stderr with statistics
func (a *Aggregator) outputBloomFilter() {
	base64Filter := a.seenEvents.ToBase64()
	estimatedEvents := a.seenEvents.EstimatedItems()
	memoryUsage := float64(a.seenEvents.MemoryUsage()) / 1024 / 1024

	// Get actual time range of processed events
	actualSince, actualUntil := a.getActualTimeRange()

	// Output to stderr so it doesn't interfere with JSONL event output to stdout
	fmt.Fprintf(os.Stderr, "\n=== BLOOM FILTER SUMMARY ===\n")
	fmt.Fprintf(os.Stderr, "Events processed: %d\n", a.eventCount)
	fmt.Fprintf(os.Stderr, "Estimated unique events: %d\n", estimatedEvents)
	fmt.Fprintf(os.Stderr, "Bloom filter size: %.2f MB\n", memoryUsage)
	fmt.Fprintf(os.Stderr, "False positive rate: ~0.1%%\n")
	fmt.Fprintf(os.Stderr, "Hash functions: %d\n", bloomFilterHashFuncs)

	// Output time range information
	if actualSince != nil && actualUntil != nil {
		fmt.Fprintf(os.Stderr, "Time range covered: %d to %d\n", actualSince.I64(), actualUntil.I64())
		fmt.Fprintf(os.Stderr, "Time range (human): %s to %s\n",
			time.Unix(actualSince.I64(), 0).UTC().Format(time.RFC3339),
			time.Unix(actualUntil.I64(), 0).UTC().Format(time.RFC3339))
	} else if a.since != nil && a.until != nil {
		// Fallback to requested range if no events were processed
		fmt.Fprintf(os.Stderr, "Requested time range: %d to %d\n", a.since.I64(), a.until.I64())
		fmt.Fprintf(os.Stderr, "Requested range (human): %s to %s\n",
			time.Unix(a.since.I64(), 0).UTC().Format(time.RFC3339),
			time.Unix(a.until.I64(), 0).UTC().Format(time.RFC3339))
	} else {
		fmt.Fprintf(os.Stderr, "Time range: unbounded\n")
	}

	fmt.Fprintf(os.Stderr, "\nBloom filter (base64):\n%s\n", base64Filter)
	fmt.Fprintf(os.Stderr, "=== END BLOOM FILTER ===\n")
}

// getMemoryUsageMB returns current memory usage in MB
func (a *Aggregator) getMemoryUsageMB() float64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return float64(m.Alloc) / 1024 / 1024
}

// memoryMonitor monitors memory usage and logs statistics
func (a *Aggregator) memoryMonitor() {
	for {
		select {
		case <-a.ctx.Done():
			log.I.F("memory monitor stopping")
			return
		case <-a.memoryTicker.C:
			memUsage := a.getMemoryUsageMB()
			bloomMemMB := float64(a.seenEvents.MemoryUsage()) / 1024 / 1024
			estimatedEvents := a.seenEvents.EstimatedItems()

			log.I.F("memory stats: total=%.2fMB, bloom=%.2fMB, events=%d, estimated_events=%d",
				memUsage, bloomMemMB, a.eventCount, estimatedEvents)

			// Check if we're approaching memory limits
			if memUsage > maxMemoryMB {
				log.W.F("high memory usage detected: %.2fMB (limit: %dMB)", memUsage, maxMemoryMB)

				// Force garbage collection
				runtime.GC()

				// Check again after GC
				newMemUsage := a.getMemoryUsageMB()
				log.I.F("memory usage after GC: %.2fMB", newMemUsage)

				// If still too high, warn but continue (bloom filter has fixed size)
				if newMemUsage > maxMemoryMB*1.2 {
					log.E.F("critical memory usage: %.2fMB, but continuing with bloom filter", newMemUsage)
				}
			}
		}
	}
}

func parseTimestamp(s string) (ts *timestamp.T, err error) {
	if s == "" {
		return nil, nil
	}

	var t int64
	if t, err = strconv.ParseInt(s, 10, 64); chk.E(err) {
		return nil, fmt.Errorf("invalid timestamp format: %w", err)
	}

	ts = timestamp.FromUnix(t)
	return
}

func main() {
	var keyInput string
	var sinceStr string
	var untilStr string
	var bloomFilterFile string
	var outputFile string

	flag.StringVar(&keyInput, "key", "", "nsec (private key) or npub (public key) to search for events")
	flag.StringVar(&sinceStr, "since", "", "start timestamp (Unix timestamp) - only events after this time")
	flag.StringVar(&untilStr, "until", "", "end timestamp (Unix timestamp) - only events before this time")
	flag.StringVar(&bloomFilterFile, "filter", "", "file containing base64 encoded bloom filter to exclude already seen events")
	flag.StringVar(&outputFile, "output", "", "output file for events (default: stdout)")
	flag.Parse()

	if keyInput == "" {
		fmt.Fprintf(os.Stderr, "Usage: %s -key <nsec|npub> [-since <timestamp>] [-until <timestamp>] [-filter <file>] [-output <file>]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Example: %s -key npub1... -since 1640995200 -until 1672531200 -filter bloom.txt -output events.jsonl\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Example: %s -key nsec1... -since 1640995200 -until 1672531200 -output events.jsonl\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nKey types:\n")
		fmt.Fprintf(os.Stderr, "  nsec: Private key (enables authentication to relays that require it)\n")
		fmt.Fprintf(os.Stderr, "  npub: Public key (authentication disabled)\n")
		fmt.Fprintf(os.Stderr, "\nTimestamps should be Unix timestamps (seconds since epoch)\n")
		fmt.Fprintf(os.Stderr, "If -filter is provided, output will be appended to the output file\n")
		os.Exit(1)
	}

	var since, until *timestamp.T
	var err error

	if since, err = parseTimestamp(sinceStr); chk.E(err) {
		fmt.Fprintf(os.Stderr, "Error parsing since timestamp: %v\n", err)
		os.Exit(1)
	}

	if until, err = parseTimestamp(untilStr); chk.E(err) {
		fmt.Fprintf(os.Stderr, "Error parsing until timestamp: %v\n", err)
		os.Exit(1)
	}

	// Validate that since is before until if both are provided
	if since != nil && until != nil && since.I64() >= until.I64() {
		fmt.Fprintf(os.Stderr, "Error: since timestamp must be before until timestamp\n")
		os.Exit(1)
	}

	// Set up output redirection if needed
	if outputFile != "" {
		var file *os.File
		if bloomFilterFile != "" {
			// Append mode if bloom filter is provided
			file, err = os.OpenFile(outputFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		} else {
			// Truncate mode if no bloom filter
			file, err = os.OpenFile(outputFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening output file: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()

		// Redirect stdout to file
		os.Stdout = file
	}

	var agg *Aggregator
	if agg, err = NewAggregator(keyInput, since, until, bloomFilterFile); chk.E(err) {
		fmt.Fprintf(os.Stderr, "Error creating aggregator: %v\n", err)
		os.Exit(1)
	}

	if err = agg.Start(); chk.E(err) {
		fmt.Fprintf(os.Stderr, "Error running aggregator: %v\n", err)
		os.Exit(1)
	}
}
