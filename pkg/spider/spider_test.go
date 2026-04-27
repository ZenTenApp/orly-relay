package spider

import (
	"context"
	"os"
	"testing"
	"time"

	"git.smesh.lol/orly/pkg/database"
)

func TestSpiderCreation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a temporary database for testing
	tempDir, err := os.MkdirTemp("", "spider-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	db, err := database.New(ctx, cancel, tempDir, "error")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Test spider creation
	spider, err := New(ctx, db, nil, "follows")
	if err != nil {
		t.Fatalf("Failed to create spider: %v", err)
	}

	if spider == nil {
		t.Fatal("Spider is nil")
	}

	// Test that spider is not running initially
	spider.mu.RLock()
	running := spider.running
	spider.mu.RUnlock()

	if running {
		t.Error("Spider should not be running initially")
	}
}

func TestSpiderCallbacks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a temporary database for testing
	tempDir, err := os.MkdirTemp("", "spider-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	db, err := database.New(ctx, cancel, tempDir, "error")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	spider, err := New(ctx, db, nil, "follows")
	if err != nil {
		t.Fatalf("Failed to create spider: %v", err)
	}

	// Test callback setup
	testRelays := []string{"wss://relay1.example.com", "wss://relay2.example.com"}
	testPubkeys := [][]byte{{1, 2, 3}, {4, 5, 6}}

	spider.SetCallbacks(
		func() []string { return testRelays },
		func() [][]byte { return testPubkeys },
	)

	// Verify callbacks are set
	spider.mu.RLock()
	hasCallbacks := spider.getAdminRelays != nil && spider.getFollowList != nil
	spider.mu.RUnlock()

	if !hasCallbacks {
		t.Error("Callbacks should be set")
	}

	// Test that start fails without callbacks being set first
	spider2, err := New(ctx, db, nil, "follows")
	if err != nil {
		t.Fatalf("Failed to create second spider: %v", err)
	}

	err = spider2.Start()
	if err == nil {
		t.Error("Start should fail when callbacks are not set")
	}
}

func TestSpiderModeValidation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a temporary database for testing
	tempDir, err := os.MkdirTemp("", "spider-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	db, err := database.New(ctx, cancel, tempDir, "error")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Test valid mode
	spider, err := New(ctx, db, nil, "follows")
	if err != nil {
		t.Fatalf("Failed to create spider with valid mode: %v", err)
	}
	if spider == nil {
		t.Fatal("Spider should not be nil for valid mode")
	}

	// Test invalid mode
	_, err = New(ctx, db, nil, "invalid")
	if err == nil {
		t.Error("Should fail with invalid mode")
	}

	// Test none mode (should succeed but be a no-op)
	spider2, err := New(ctx, db, nil, "none")
	if err != nil {
		t.Errorf("Should succeed with 'none' mode: %v", err)
	}
	if spider2 == nil {
		t.Error("Spider should not be nil for 'none' mode")
	}

	// Test that 'none' mode doesn't require callbacks
	err = spider2.Start()
	if err != nil {
		t.Errorf("'none' mode should start without callbacks: %v", err)
	}
}

func TestSpiderBatching(t *testing.T) {
	// Test batch creation logic
	followList := make([][]byte, 50) // 50 pubkeys
	for i := range followList {
		followList[i] = make([]byte, 32)
		for j := range followList[i] {
			followList[i][j] = byte(i)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rc := &RelayConnection{
		url: "wss://test.relay.com",
		ctx: ctx,
	}

	batches := rc.createBatches(followList)

	// Should create 3 batches: 20, 20, 10
	expectedBatches := 3
	if len(batches) != expectedBatches {
		t.Errorf("Expected %d batches, got %d", expectedBatches, len(batches))
	}

	// Check batch sizes
	if len(batches[0]) != BatchSize {
		t.Errorf("First batch should have %d pubkeys, got %d", BatchSize, len(batches[0]))
	}
	if len(batches[1]) != BatchSize {
		t.Errorf("Second batch should have %d pubkeys, got %d", BatchSize, len(batches[1]))
	}
	if len(batches[2]) != 10 {
		t.Errorf("Third batch should have 10 pubkeys, got %d", len(batches[2]))
	}
}

func TestSpiderStartStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a temporary database for testing
	tempDir, err := os.MkdirTemp("", "spider-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	db, err := database.New(ctx, cancel, tempDir, "error")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	spider, err := New(ctx, db, nil, "follows")
	if err != nil {
		t.Fatalf("Failed to create spider: %v", err)
	}

	// Set up callbacks
	spider.SetCallbacks(
		func() []string { return []string{"wss://test.relay.com"} },
		func() [][]byte { return [][]byte{{1, 2, 3}} },
	)

	// Test start
	err = spider.Start()
	if err != nil {
		t.Fatalf("Failed to start spider: %v", err)
	}

	// Verify spider is running
	spider.mu.RLock()
	running := spider.running
	spider.mu.RUnlock()

	if !running {
		t.Error("Spider should be running after start")
	}

	// Test stop
	spider.Stop()

	// Give it a moment to stop
	time.Sleep(100 * time.Millisecond)

	// Verify spider is stopped
	spider.mu.RLock()
	running = spider.running
	spider.mu.RUnlock()

	if running {
		t.Error("Spider should not be running after stop")
	}
}
