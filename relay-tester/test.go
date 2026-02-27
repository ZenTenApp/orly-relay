package relaytester

import (
	"encoding/json"
	"time"

	"next.orly.dev/pkg/lol/errorf"
)

// TestResult represents the result of a test.
type TestResult struct {
	Name     string `json:"test"`
	Pass     bool   `json:"pass"`
	Required bool   `json:"required"`
	Info     string `json:"info,omitempty"`
}

// TestFunc is a function that runs a test case.
type TestFunc func(client *Client, key1, key2 *KeyPair) (result TestResult)

// TestCase represents a test case with dependencies.
type TestCase struct {
	Name         string
	Required     bool
	Func         TestFunc
	Dependencies []string // Names of tests that must run before this one
}

// TestSuite runs all tests against a relay.
type TestSuite struct {
	relayURL string
	key1     *KeyPair
	key2     *KeyPair
	tests    map[string]*TestCase
	results  map[string]TestResult
	order    []string
}

// NewTestSuite creates a new test suite.
func NewTestSuite(relayURL string) (suite *TestSuite, err error) {
	suite = &TestSuite{
		relayURL: relayURL,
		tests:    make(map[string]*TestCase),
		results:  make(map[string]TestResult),
	}
	if suite.key1, err = GenerateKeyPair(); err != nil {
		return
	}
	if suite.key2, err = GenerateKeyPair(); err != nil {
		return
	}
	suite.registerTests()
	return
}

// AddTest adds a test case to the suite.
func (s *TestSuite) AddTest(tc *TestCase) {
	s.tests[tc.Name] = tc
}

// registerTests registers all test cases.
func (s *TestSuite) registerTests() {
	allTests := []*TestCase{
		{
			Name:     "Publishes basic event",
			Required: true,
			Func:     testPublishBasicEvent,
		},
		{
			Name:         "Finds event by ID",
			Required:     true,
			Func:         testFindByID,
			Dependencies: []string{"Publishes basic event"},
		},
		{
			Name:         "Finds event by author",
			Required:     true,
			Func:         testFindByAuthor,
			Dependencies: []string{"Publishes basic event"},
		},
		{
			Name:         "Finds event by kind",
			Required:     true,
			Func:         testFindByKind,
			Dependencies: []string{"Publishes basic event"},
		},
		{
			Name:         "Finds event by tags",
			Required:     true,
			Func:         testFindByTags,
			Dependencies: []string{"Publishes basic event"},
		},
		{
			Name:         "Finds by multiple tags",
			Required:     true,
			Func:         testFindByMultipleTags,
			Dependencies: []string{"Publishes basic event"},
		},
		{
			Name:         "Finds by time range",
			Required:     true,
			Func:         testFindByTimeRange,
			Dependencies: []string{"Publishes basic event"},
		},
		{
			Name:     "Rejects invalid signature",
			Required: true,
			Func:     testRejectInvalidSignature,
		},
		{
			Name:     "Rejects future event",
			Required: true,
			Func:     testRejectFutureEvent,
		},
		{
			Name:     "Rejects expired event",
			Required: false,
			Func:     testRejectExpiredEvent,
		},
		{
			Name:     "Handles replaceable events",
			Required: true,
			Func:     testReplaceableEvents,
		},
		{
			Name:     "Handles ephemeral events",
			Required: false,
			Func:     testEphemeralEvents,
		},
		{
			Name:     "Handles parameterized replaceable events",
			Required: true,
			Func:     testParameterizedReplaceableEvents,
		},
		{
			Name:         "Handles deletion events",
			Required:     true,
			Func:         testDeletionEvents,
			Dependencies: []string{"Publishes basic event"},
		},
		{
			Name:         "Handles COUNT request",
			Required:     true,
			Func:         testCountRequest,
			Dependencies: []string{"Publishes basic event"},
		},
		{
			Name:         "Handles limit parameter",
			Required:     true,
			Func:         testLimitParameter,
			Dependencies: []string{"Publishes basic event"},
		},
		{
			Name:         "Handles multiple filters",
			Required:     true,
			Func:         testMultipleFilters,
			Dependencies: []string{"Publishes basic event"},
		},
		{
			Name:     "Handles subscription close",
			Required: true,
			Func:     testSubscriptionClose,
		},
		// Filter tests
		{
			Name:         "Since and until filters are inclusive",
			Required:     true,
			Func:         testSinceUntilAreInclusive,
			Dependencies: []string{"Publishes basic event"},
		},
		{
			Name:     "Limit zero works",
			Required: true,
			Func:     testLimitZero,
		},
		// Find tests
		{
			Name:         "Events are ordered from newest to oldest",
			Required:     true,
			Func:         testEventsOrderedFromNewestToOldest,
			Dependencies: []string{"Publishes basic event"},
		},
		{
			Name:         "Newest events are returned when filter is limited",
			Required:     true,
			Func:         testNewestEventsWhenLimited,
			Dependencies: []string{"Publishes basic event"},
		},
		{
			Name:         "Finds by pubkey and kind",
			Required:     true,
			Func:         testFindByPubkeyAndKind,
			Dependencies: []string{"Publishes basic event"},
		},
		{
			Name:         "Finds by pubkey and tags",
			Required:     true,
			Func:         testFindByPubkeyAndTags,
			Dependencies: []string{"Publishes basic event"},
		},
		{
			Name:         "Finds by kind and tags",
			Required:     true,
			Func:         testFindByKindAndTags,
			Dependencies: []string{"Publishes basic event"},
		},
		{
			Name:         "Finds by scrape",
			Required:     true,
			Func:         testFindByScrape,
			Dependencies: []string{"Publishes basic event"},
		},
		// Replaceable event tests
		{
			Name:         "Replaces metadata",
			Required:     true,
			Func:         testReplacesMetadata,
			Dependencies: []string{"Publishes basic event"},
		},
		{
			Name:         "Replaces contact list",
			Required:     true,
			Func:         testReplacesContactList,
			Dependencies: []string{"Publishes basic event"},
		},
		{
			Name:         "Replaced events are still available by ID",
			Required:     false,
			Func:         testReplacedEventsStillAvailableByID,
			Dependencies: []string{"Publishes basic event"},
		},
		{
			Name:         "Replaceable events replace older ones",
			Required:     true,
			Func:         testReplaceableEventRemovesPrevious,
			Dependencies: []string{"Publishes basic event"},
		},
		{
			Name:         "Replaceable events rejected if a newer one exists",
			Required:     true,
			Func:         testReplaceableEventRejectedIfFuture,
			Dependencies: []string{"Publishes basic event"},
		},
		{
			Name:         "Addressable events replace older ones",
			Required:     true,
			Func:         testAddressableEventRemovesPrevious,
			Dependencies: []string{"Publishes basic event"},
		},
		{
			Name:         "Addressable events rejected if a newer one exists",
			Required:     true,
			Func:         testAddressableEventRejectedIfFuture,
			Dependencies: []string{"Publishes basic event"},
		},
		// Deletion tests
		{
			Name:         "Deletes by a-tag address",
			Required:     true,
			Func:         testDeleteByAddr,
			Dependencies: []string{"Publishes basic event"},
		},
		{
			Name:         "Delete by a-tag deletes older but not newer",
			Required:     true,
			Func:         testDeleteByAddrOnlyDeletesOlder,
			Dependencies: []string{"Publishes basic event"},
		},
		{
			Name:         "Delete by a-tag is bound by a-tag",
			Required:     true,
			Func:         testDeleteByAddrIsBoundByTag,
			Dependencies: []string{"Publishes basic event"},
		},
		// Ephemeral tests
		{
			Name:         "Ephemeral subscriptions work",
			Required:     false,
			Func:         testEphemeralSubscriptionsWork,
			Dependencies: []string{"Publishes basic event"},
		},
		{
			Name:         "Persists ephemeral events",
			Required:     false,
			Func:         testPersistsEphemeralEvents,
			Dependencies: []string{"Publishes basic event"},
		},
		// EOSE tests
		{
			Name:     "Supports EOSE",
			Required: true,
			Func:     testSupportsEose,
		},
		{
			Name:     "Subscription receives event after ping period",
			Required: true,
			Func:     testSubscriptionReceivesEventAfterPingPeriod,
		},
		{
			Name:     "Closes complete subscriptions after EOSE",
			Required: false,
			Func:     testClosesCompleteSubscriptionsAfterEose,
		},
		{
			Name:     "Keeps open incomplete subscriptions after EOSE",
			Required: true,
			Func:     testKeepsOpenIncompleteSubscriptionsAfterEose,
		},
		// JSON tests
		{
			Name:         "Accepts events with empty tags",
			Required:     false,
			Func:         testAcceptsEventsWithEmptyTags,
			Dependencies: []string{"Publishes basic event"},
		},
		{
			Name:         "Accepts NIP-01 JSON escape sequences",
			Required:     true,
			Func:         testAcceptsNip1JsonEscapeSequences,
			Dependencies: []string{"Publishes basic event"},
		},
		// Registration tests
		{
			Name:     "Sends OK after EVENT",
			Required: true,
			Func:     testSendsOkAfterEvent,
		},
		{
			Name:     "Verifies event signatures",
			Required: true,
			Func:     testVerifiesSignatures,
		},
		{
			Name:     "Verifies event ID hashes",
			Required: true,
			Func:     testVerifiesIdHashes,
		},
	}
	for _, tc := range allTests {
		s.AddTest(tc)
	}
	s.topologicalSort()
}

// topologicalSort orders tests based on dependencies.
func (s *TestSuite) topologicalSort() {
	visited := make(map[string]bool)
	temp := make(map[string]bool)
	var visit func(name string)
	visit = func(name string) {
		if temp[name] {
			return
		}
		if visited[name] {
			return
		}
		temp[name] = true
		if tc, exists := s.tests[name]; exists {
			for _, dep := range tc.Dependencies {
				visit(dep)
			}
		}
		temp[name] = false
		visited[name] = true
		s.order = append(s.order, name)
	}
	for name := range s.tests {
		if !visited[name] {
			visit(name)
		}
	}
}

// Run runs all tests in the suite.
func (s *TestSuite) Run() (results []TestResult, err error) {
	for _, name := range s.order {
		tc := s.tests[name]
		if tc == nil {
			continue
		}
		// Create a new client for each test to avoid connection issues
		client, clientErr := NewClient(s.relayURL)
		if clientErr != nil {
			return nil, errorf.E("failed to connect to relay: %w", clientErr)
		}
		result := tc.Func(client, s.key1, s.key2)
		result.Name = name
		result.Required = tc.Required
		s.results[name] = result
		results = append(results, result)
		client.Close()
		time.Sleep(100 * time.Millisecond) // Small delay between tests
	}
	return
}

// RunTest runs a specific test by name.
func (s *TestSuite) RunTest(testName string) (result TestResult, err error) {
	tc, exists := s.tests[testName]
	if !exists {
		return result, errorf.E("test %s not found", testName)
	}
	// Check dependencies
	for _, dep := range tc.Dependencies {
		if _, exists := s.results[dep]; !exists {
			return result, errorf.E("test %s depends on %s which has not been run", testName, dep)
		}
		if !s.results[dep].Pass {
			return result, errorf.E("test %s depends on %s which failed", testName, dep)
		}
	}
	// Create a new client for the test
	client, clientErr := NewClient(s.relayURL)
	if clientErr != nil {
		return result, errorf.E("failed to connect to relay: %w", clientErr)
	}
	defer client.Close()
	result = tc.Func(client, s.key1, s.key2)
	result.Name = testName
	result.Required = tc.Required
	s.results[testName] = result
	return
}

// GetResults returns all test results.
func (s *TestSuite) GetResults() map[string]TestResult {
	return s.results
}

// ListTests returns a list of all test names in execution order.
func (s *TestSuite) ListTests() []string {
	return s.order
}

// GetTestNames returns all registered test names as a map (name -> required).
func (s *TestSuite) GetTestNames() map[string]bool {
	result := make(map[string]bool)
	for name, tc := range s.tests {
		result[name] = tc.Required
	}
	return result
}

// FormatJSON formats results as JSON.
func FormatJSON(results []TestResult) (output string, err error) {
	var data []byte
	if data, err = json.Marshal(results); err != nil {
		return
	}
	return string(data), nil
}
