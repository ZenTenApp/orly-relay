package bridge

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSubscription_IsActive(t *testing.T) {
	active := &Subscription{
		PubkeyHex: "abc123",
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
	}
	if !active.IsActive() {
		t.Error("subscription should be active")
	}

	expired := &Subscription{
		PubkeyHex: "abc123",
		ExpiresAt: time.Now().Add(-1 * time.Hour),
		CreatedAt: time.Now().Add(-25 * time.Hour),
	}
	if expired.IsActive() {
		t.Error("subscription should be expired")
	}
}

func TestMemorySubscriptionStore_CRUD(t *testing.T) {
	store := NewMemorySubscriptionStore()

	sub := &Subscription{
		PubkeyHex: "aabbccdd",
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
		CreatedAt: time.Now(),
	}

	// Save
	if err := store.Save(sub); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Get
	got, err := store.Get("aabbccdd")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.PubkeyHex != "aabbccdd" {
		t.Errorf("PubkeyHex = %q", got.PubkeyHex)
	}

	// Get not found
	_, err = store.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent key")
	}

	// List
	subs, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(subs) != 1 {
		t.Errorf("List returned %d, want 1", len(subs))
	}

	// Delete
	if err := store.Delete("aabbccdd"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = store.Get("aabbccdd")
	if err == nil {
		t.Error("expected error after delete")
	}

	// List after delete
	subs, err = store.List()
	if err != nil {
		t.Fatalf("List after delete: %v", err)
	}
	if len(subs) != 0 {
		t.Errorf("List returned %d after delete, want 0", len(subs))
	}
}

func TestMemorySubscriptionStore_Overwrite(t *testing.T) {
	store := NewMemorySubscriptionStore()

	sub1 := &Subscription{
		PubkeyHex:   "aabbccdd",
		ExpiresAt:   time.Now().Add(30 * 24 * time.Hour),
		CreatedAt:   time.Now(),
		InvoiceHash: "hash1",
	}
	sub2 := &Subscription{
		PubkeyHex:   "aabbccdd",
		ExpiresAt:   time.Now().Add(60 * 24 * time.Hour),
		CreatedAt:   time.Now(),
		InvoiceHash: "hash2",
	}

	store.Save(sub1)
	store.Save(sub2)

	got, _ := store.Get("aabbccdd")
	if got.InvoiceHash != "hash2" {
		t.Errorf("InvoiceHash = %q, want hash2 (overwrite)", got.InvoiceHash)
	}
}

func TestFileSubscriptionStore_Persistence(t *testing.T) {
	dir := t.TempDir()

	// Create store and save
	store1, err := NewFileSubscriptionStore(dir)
	if err != nil {
		t.Fatalf("NewFileSubscriptionStore: %v", err)
	}

	sub := &Subscription{
		PubkeyHex:   "aabbccdd",
		ExpiresAt:   time.Now().Add(30 * 24 * time.Hour).Truncate(time.Second),
		CreatedAt:   time.Now().Truncate(time.Second),
		InvoiceHash: "testhash",
	}

	if err := store1.Save(sub); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify file exists
	path := filepath.Join(dir, "subscriptions.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created: %v", err)
	}

	// Create a new store from same dir — should load persisted data
	store2, err := NewFileSubscriptionStore(dir)
	if err != nil {
		t.Fatalf("NewFileSubscriptionStore (reload): %v", err)
	}

	got, err := store2.Get("aabbccdd")
	if err != nil {
		t.Fatalf("Get after reload: %v", err)
	}
	if got.InvoiceHash != "testhash" {
		t.Errorf("InvoiceHash = %q after reload, want testhash", got.InvoiceHash)
	}

	// Delete and verify persistence
	if err := store2.Delete("aabbccdd"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	store3, err := NewFileSubscriptionStore(dir)
	if err != nil {
		t.Fatalf("NewFileSubscriptionStore (after delete): %v", err)
	}
	_, err = store3.Get("aabbccdd")
	if err == nil {
		t.Error("expected error: subscription should be deleted after reload")
	}
}

func TestFileSubscriptionStore_MultipleSubscriptions(t *testing.T) {
	dir := t.TempDir()

	store, err := NewFileSubscriptionStore(dir)
	if err != nil {
		t.Fatalf("NewFileSubscriptionStore: %v", err)
	}

	for i, pk := range []string{"aaaa", "bbbb", "cccc"} {
		sub := &Subscription{
			PubkeyHex: pk,
			ExpiresAt: time.Now().Add(time.Duration(i+1) * 24 * time.Hour),
			CreatedAt: time.Now(),
		}
		if err := store.Save(sub); err != nil {
			t.Fatalf("Save %s: %v", pk, err)
		}
	}

	subs, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(subs) != 3 {
		t.Errorf("List returned %d, want 3", len(subs))
	}

	// Delete middle one
	if err := store.Delete("bbbb"); err != nil {
		t.Fatalf("Delete bbbb: %v", err)
	}

	subs, err = store.List()
	if err != nil {
		t.Fatalf("List after delete: %v", err)
	}
	if len(subs) != 2 {
		t.Errorf("List after delete returned %d, want 2", len(subs))
	}
}

func TestFileSubscriptionStore_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	store, err := NewFileSubscriptionStore(dir)
	if err != nil {
		t.Fatalf("NewFileSubscriptionStore: %v", err)
	}

	subs, err := store.List()
	if err != nil {
		t.Fatalf("List empty: %v", err)
	}
	if len(subs) != 0 {
		t.Errorf("List empty returned %d, want 0", len(subs))
	}
}
