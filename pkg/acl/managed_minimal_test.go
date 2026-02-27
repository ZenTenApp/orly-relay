package acl

import (
	"context"
	"testing"
	"time"

	"next.orly.dev/app/config"
	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/nostr/encoders/event"
)

func TestManagedACL_BasicFunctionality(t *testing.T) {
	// Setup test database
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use a temporary directory for the test database
	tmpDir := t.TempDir()
	db, err := database.New(ctx, cancel, tmpDir, "test.db")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Setup managed ACL
	cfg := &config.C{
		AuthRequired: false,
		Owners:       []string{"owner1"},
		Admins:       []string{"admin1"},
	}

	managed := &Managed{
		Ctx:        ctx,
		cfg:        cfg,
		db:         db,
		managedACL: database.NewManagedACL(db),
		owners:     [][]byte{[]byte("owner1")},
		admins:     [][]byte{[]byte("admin1")},
	}

	// Test basic functionality
	t.Run("owner should get owner access", func(t *testing.T) {
		level := managed.GetAccessLevel([]byte("owner1"), "127.0.0.1")
		if level != "owner" {
			t.Errorf("GetAccessLevel() = %v, want owner", level)
		}
	})

	t.Run("admin should get admin access", func(t *testing.T) {
		level := managed.GetAccessLevel([]byte("admin1"), "127.0.0.1")
		if level != "admin" {
			t.Errorf("GetAccessLevel() = %v, want admin", level)
		}
	})

	t.Run("default user should get read access", func(t *testing.T) {
		level := managed.GetAccessLevel([]byte("user1"), "127.0.0.1")
		if level != "read" {
			t.Errorf("GetAccessLevel() = %v, want read", level)
		}
	})

	t.Run("owner event should be allowed", func(t *testing.T) {
		ev := createMinimalTestEvent("owner1", 1)
		allowed, err := managed.CheckPolicy(ev)
		if err != nil {
			t.Fatalf("CheckPolicy() error = %v", err)
		}
		if !allowed {
			t.Errorf("CheckPolicy() = %v, want true", allowed)
		}
	})

	t.Run("admin event should be allowed", func(t *testing.T) {
		ev := createMinimalTestEvent("admin1", 1)
		allowed, err := managed.CheckPolicy(ev)
		if err != nil {
			t.Fatalf("CheckPolicy() error = %v", err)
		}
		if !allowed {
			t.Errorf("CheckPolicy() = %v, want true", allowed)
		}
	})

	t.Run("default event should be allowed", func(t *testing.T) {
		ev := createMinimalTestEvent("user1", 1)
		allowed, err := managed.CheckPolicy(ev)
		if err != nil {
			t.Fatalf("CheckPolicy() error = %v", err)
		}
		if !allowed {
			t.Errorf("CheckPolicy() = %v, want true", allowed)
		}
	})
}

func createMinimalTestEvent(pubkey string, kind uint16) *event.E {
	ev := event.New()
	ev.Pubkey = []byte(pubkey)
	ev.Kind = kind
	ev.CreatedAt = time.Now().Unix()
	ev.Content = []byte("test content")
	ev.Tags = nil
	ev.ID = ev.GetIDBytes()
	return ev
}
