package policy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer/p8k"
)

// Helper function to create test event for benchmarks (reuses signer)
func createTestEventBench(b *testing.B, signer *p8k.Signer, content string, kind uint16) *event.E {
	ev := event.New()
	ev.CreatedAt = time.Now().Unix()
	ev.Kind = kind
	ev.Content = []byte(content)
	ev.Tags = tag.NewS()

	// Sign the event properly
	if err := ev.Sign(signer); chk.E(err) {
		b.Fatalf("Failed to sign test event: %v", err)
	}

	return ev
}

func BenchmarkCheckKindsPolicy(b *testing.B) {
	policy := &P{
		Kind: Kinds{
			Whitelist: []int{1, 3, 5, 7, 9735},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		policy.checkKindsPolicy("write", 1)
	}
}

func BenchmarkCheckRulePolicy(b *testing.B) {
	// Generate keypair once for all events
	signer, pubkey := generateTestKeypairB(b)
	testEvent := createTestEventBench(b, signer, "test content", 1)

	rule := Rule{
		Description: "test rule",
		AccessControl: AccessControl{
			WriteAllow: []string{hex.Enc(pubkey)},
		},
		Constraints: Constraints{
			SizeLimit:    int64Ptr(10000),
			ContentLimit: int64Ptr(1000),
		},
		TagValidationConfig: TagValidationConfig{
			MustHaveTags: []string{"p"},
		},
	}

	policy := &P{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		policy.checkRulePolicy("write", testEvent, rule, pubkey)
	}
}

func BenchmarkCheckPolicy(b *testing.B) {
	// Generate keypair once for all events
	signer, pubkey := generateTestKeypairB(b)
	testEvent := createTestEventBench(b, signer, "test content", 1)

	policy := &P{
		Kind: Kinds{
			Whitelist: []int{1, 3, 5},
		},
		rules: map[int]Rule{
			1: {
				Description: "test rule",
				AccessControl: AccessControl{
					WriteAllow: []string{hex.Enc(pubkey)},
				},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		policy.CheckPolicy("write", testEvent, pubkey, "127.0.0.1")
	}
}

func BenchmarkCheckPolicyWithScript(b *testing.B) {
	// Create temporary directory
	tempDir := b.TempDir()
	scriptPath := filepath.Join(tempDir, "policy.sh")

	// Create a simple test script
	scriptContent := `#!/bin/bash
while IFS= read -r line; do
    echo '{"id":"test","action":"accept","msg":""}'
done
`
	err := os.WriteFile(scriptPath, []byte(scriptContent), 0755)
	if err != nil {
		b.Fatalf("Failed to create test script: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager := &PolicyManager{
		ctx:        ctx,
		cancel:     cancel,
		configDir:  tempDir,
		scriptPath: scriptPath,
		enabled:    true,
		runners:    make(map[string]*ScriptRunner),
	}

	// Get or create runner and start it
	runner := manager.getOrCreateRunner(scriptPath)
	err = runner.Start()
	if err != nil {
		b.Fatalf("Failed to start policy script: %v", err)
	}
	defer runner.Stop()

	// Give the script time to start
	time.Sleep(100 * time.Millisecond)

	// Generate keypair once for all events
	signer, pubkey := generateTestKeypairB(b)
	testEvent := createTestEventBench(b, signer, "test content", 1)

	policy := &P{
		manager: manager,
		Kind:    Kinds{},
		rules: map[int]Rule{
			1: {
				Description: "test rule with script",
				Script:      "policy.sh",
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		policy.CheckPolicy("write", testEvent, pubkey, "127.0.0.1")
	}
}

func BenchmarkLoadFromFile(b *testing.B) {
	// Create temporary directory
	tempDir := b.TempDir()
	configPath := filepath.Join(tempDir, "policy.json")

	// Create test config
	configData := `{
		"kind": {
			"whitelist": [1, 3, 5, 7, 9735],
			"blacklist": []
		},
		"rules": {
			"1": {
				"description": "text notes",
				"write_allow": [],
				"size_limit": 32000
			},
			"3": {
				"description": "contacts",
				"write_allow": ["npub1example1"],
				"script": "policy.sh"
			}
		}
	}`

	err := os.WriteFile(configPath, []byte(configData), 0644)
	if err != nil {
		b.Fatalf("Failed to create test config: %v", err)
	}

	policy := &P{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := policy.LoadFromFile(configPath)
		if err != nil {
			b.Fatalf("Failed to load config: %v", err)
		}
	}
}

func BenchmarkCheckPolicyMultipleKinds(b *testing.B) {
	// Create policy with many rules
	rules := make(map[int]Rule)
	for i := 1; i <= 100; i++ {
		rules[i] = Rule{
			Description: "test rule",
			AccessControl: AccessControl{
				WriteAllow: []string{"test-pubkey"},
			},
		}
	}

	policy := &P{
		Kind:  Kinds{},
		rules: rules,
	}

	// Generate keypair once for all events
	signer, pubkey := generateTestKeypairB(b)

	// Create test events with different kinds
	events := make([]*event.E, 100)
	for i := 0; i < 100; i++ {
		events[i] = createTestEventBench(b, signer, "test content", uint16(i+1))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		event := events[i%100]
		policy.CheckPolicy("write", event, pubkey, "127.0.0.1")
	}
}

func BenchmarkCheckPolicyLargeWhitelist(b *testing.B) {
	// Create large whitelist
	whitelist := make([]int, 1000)
	for i := 0; i < 1000; i++ {
		whitelist[i] = i + 1
	}

	policy := &P{
		Kind: Kinds{
			Whitelist: whitelist,
		},
		rules: map[int]Rule{},
	}

	// Generate keypair once for all events
	signer, pubkey := generateTestKeypairB(b)
	testEvent := createTestEventBench(b, signer, "test content", 500) // Kind in the middle of the whitelist

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		policy.CheckPolicy("write", testEvent, pubkey, "127.0.0.1")
	}
}

func BenchmarkCheckPolicyLargeBlacklist(b *testing.B) {
	// Create large blacklist
	blacklist := make([]int, 1000)
	for i := 0; i < 1000; i++ {
		blacklist[i] = i + 1
	}

	policy := &P{
		Kind: Kinds{
			Blacklist: blacklist,
		},
		rules: map[int]Rule{},
	}

	// Generate keypair once for all events
	signer, pubkey := generateTestKeypairB(b)
	testEvent := createTestEventBench(b, signer, "test content", 1500) // Kind not in blacklist

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		policy.CheckPolicy("write", testEvent, pubkey, "127.0.0.1")
	}
}

func BenchmarkCheckPolicyComplexRule(b *testing.B) {
	// Generate keypair once for all events
	signer, pubkey := generateTestKeypairB(b)
	testEvent := createTestEventBench(b, signer, "test content", 1)

	// Add many tags
	for i := 0; i < 100; i++ {
		tagItem1 := tag.New()
		tagItem1.T = append(tagItem1.T, []byte("p"), []byte(hex.Enc(pubkey)))
		*testEvent.Tags = append(*testEvent.Tags, tagItem1)

		tagItem2 := tag.New()
		tagItem2.T = append(tagItem2.T, []byte("e"), []byte("test-event"))
		*testEvent.Tags = append(*testEvent.Tags, tagItem2)
	}

	rule := Rule{
		Description: "complex rule",
		AccessControl: AccessControl{
			WriteAllow: []string{hex.Enc(pubkey)},
		},
		Constraints: Constraints{
			SizeLimit:    int64Ptr(100000),
			ContentLimit: int64Ptr(10000),
			Privileged:   true,
		},
		TagValidationConfig: TagValidationConfig{
			MustHaveTags: []string{"p", "e"},
		},
	}

	policy := &P{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		policy.checkRulePolicy("write", testEvent, rule, pubkey)
	}
}

func BenchmarkCheckPolicyLargeEvent(b *testing.B) {
	// Create large content
	largeContent := strings.Repeat("a", 100000) // 100KB content

	policy := &P{
		Kind: Kinds{},
		rules: map[int]Rule{
			1: {
				Description: "size limit test",
				Constraints: Constraints{
					SizeLimit:    int64Ptr(200000), // 200KB limit
					ContentLimit: int64Ptr(200000), // 200KB content limit
				},
			},
		},
	}

	// Generate keypair once for all events
	signer, pubkey := generateTestKeypairB(b)
	testEvent := createTestEventBench(b, signer, largeContent, 1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		policy.CheckPolicy("write", testEvent, pubkey, "127.0.0.1")
	}
}
