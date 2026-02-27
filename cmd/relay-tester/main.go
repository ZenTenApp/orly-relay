package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"next.orly.dev/pkg/lol/log"
	relaytester "next.orly.dev/relay-tester"
)

func main() {
	var (
		relayURL = flag.String("url", "", "relay websocket URL (required, e.g., ws://127.0.0.1:3334)")
		testName = flag.String("test", "", "run specific test by name (default: run all tests)")
		jsonOut  = flag.Bool("json", false, "output results in JSON format")
		verbose  = flag.Bool("v", false, "verbose output")
		listTests = flag.Bool("list", false, "list all available tests and exit")
	)
	flag.Parse()

	if *listTests {
		listAllTests()
		return
	}

	if *relayURL == "" {
		log.E.F("required flag: -url (relay websocket URL)")
		flag.Usage()
		os.Exit(1)
	}

	// Validate URL format
	if !strings.HasPrefix(*relayURL, "ws://") && !strings.HasPrefix(*relayURL, "wss://") {
		log.E.F("URL must start with ws:// or wss://")
		os.Exit(1)
	}

	// Create test suite
	if *verbose {
		log.I.F("Creating test suite for %s...", *relayURL)
	}
	suite, err := relaytester.NewTestSuite(*relayURL)
	if err != nil {
		log.E.F("failed to create test suite: %v", err)
		os.Exit(1)
	}

	// Run tests
	var results []relaytester.TestResult
	if *testName != "" {
		if *verbose {
			log.I.F("Running test: %s", *testName)
		}
		result, err := suite.RunTest(*testName)
		if err != nil {
			log.E.F("failed to run test %s: %v", *testName, err)
			os.Exit(1)
		}
		results = []relaytester.TestResult{result}
	} else {
		if *verbose {
			log.I.F("Running all tests...")
		}
		if results, err = suite.Run(); err != nil {
			log.E.F("failed to run tests: %v", err)
			os.Exit(1)
		}
	}

	// Output results
	if *jsonOut {
		jsonOutput, err := relaytester.FormatJSON(results)
		if err != nil {
			log.E.F("failed to format JSON: %v", err)
			os.Exit(1)
		}
		fmt.Println(jsonOutput)
	} else {
		outputResults(results, *verbose)
	}

	// Check exit code
	hasRequiredFailures := false
	for _, result := range results {
		if result.Required && !result.Pass {
			hasRequiredFailures = true
			break
		}
	}

	if hasRequiredFailures {
		os.Exit(1)
	}
}

func outputResults(results []relaytester.TestResult, verbose bool) {
	passed := 0
	failed := 0
	requiredFailed := 0

	for _, result := range results {
		if result.Pass {
			passed++
			if verbose {
				fmt.Printf("PASS: %s", result.Name)
				if result.Info != "" {
					fmt.Printf(" - %s", result.Info)
				}
				fmt.Println()
			} else {
				fmt.Printf("PASS: %s\n", result.Name)
			}
		} else {
			failed++
			if result.Required {
				requiredFailed++
				fmt.Printf("FAIL (required): %s", result.Name)
			} else {
				fmt.Printf("FAIL (optional): %s", result.Name)
			}
			if result.Info != "" {
				fmt.Printf(" - %s", result.Info)
			}
			fmt.Println()
		}
	}

	fmt.Println()
	fmt.Println("Test Summary:")
	fmt.Printf("  Total: %d\n", len(results))
	fmt.Printf("  Passed: %d\n", passed)
	fmt.Printf("  Failed: %d\n", failed)
	fmt.Printf("  Required Failed: %d\n", requiredFailed)
}

func listAllTests() {
	// Create a dummy test suite to get the list of tests
	suite, err := relaytester.NewTestSuite("ws://127.0.0.1:0")
	if err != nil {
		log.E.F("failed to create test suite: %v", err)
		os.Exit(1)
	}

	fmt.Println("Available tests:")
	fmt.Println()

	testNames := suite.ListTests()
	testInfo := suite.GetTestNames()

	for _, name := range testNames {
		required := ""
		if testInfo[name] {
			required = " (required)"
		}
		fmt.Printf("  - %s%s\n", name, required)
	}
}

