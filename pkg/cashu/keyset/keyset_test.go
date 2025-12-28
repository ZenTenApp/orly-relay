package keyset

import (
	"testing"
	"time"
)

func TestNewKeyset(t *testing.T) {
	k, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Check ID is 14 characters (7 bytes hex)
	if len(k.ID) != 14 {
		t.Errorf("ID length = %d, want 14", len(k.ID))
	}

	// Check keys are set
	if k.PrivateKey == nil {
		t.Error("PrivateKey is nil")
	}
	if k.PublicKey == nil {
		t.Error("PublicKey is nil")
	}

	// Check times are set
	if k.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}
	if !k.IsActiveForSigning() {
		t.Error("New keyset should be active for signing")
	}
	if !k.IsValidForVerification() {
		t.Error("New keyset should be valid for verification")
	}
}

func TestKeysetIDDeterministic(t *testing.T) {
	// Same private key should produce same ID
	privKeyBytes := make([]byte, 32)
	for i := range privKeyBytes {
		privKeyBytes[i] = byte(i)
	}

	k1, err := NewFromPrivateKey(privKeyBytes, time.Now(), DefaultActiveWindow, DefaultVerifyWindow)
	if err != nil {
		t.Fatalf("NewFromPrivateKey failed: %v", err)
	}

	k2, err := NewFromPrivateKey(privKeyBytes, time.Now(), DefaultActiveWindow, DefaultVerifyWindow)
	if err != nil {
		t.Fatalf("NewFromPrivateKey failed: %v", err)
	}

	if k1.ID != k2.ID {
		t.Errorf("IDs should match: %s != %s", k1.ID, k2.ID)
	}
}

func TestKeysetExpiration(t *testing.T) {
	// Create keyset with very short TTL
	k, err := NewWithTTL(100*time.Millisecond, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("NewWithTTL failed: %v", err)
	}

	// Should be active initially
	if !k.IsActiveForSigning() {
		t.Error("New keyset should be active for signing")
	}

	// Wait for signing to expire
	time.Sleep(150 * time.Millisecond)

	if k.IsActiveForSigning() {
		t.Error("Keyset should not be active for signing after expiry")
	}
	if !k.IsValidForVerification() {
		t.Error("Keyset should still be valid for verification")
	}

	// Wait for verification to expire
	time.Sleep(100 * time.Millisecond)

	if k.IsValidForVerification() {
		t.Error("Keyset should not be valid for verification after verify expiry")
	}
}

func TestKeysetDeactivate(t *testing.T) {
	k, _ := New()

	if !k.Active {
		t.Error("New keyset should be active")
	}

	k.Deactivate()

	if k.Active {
		t.Error("Keyset should not be active after Deactivate()")
	}
	if k.IsActiveForSigning() {
		t.Error("Deactivated keyset should not be active for signing")
	}
}

func TestKeysetInfo(t *testing.T) {
	k, _ := New()
	info := k.Info()

	if info.ID != k.ID {
		t.Errorf("Info ID = %s, want %s", info.ID, k.ID)
	}
	if len(info.PublicKey) != 66 { // 33 bytes * 2 hex chars
		t.Errorf("Info PublicKey length = %d, want 66", len(info.PublicKey))
	}
	if !info.Active {
		t.Error("Info Active should be true for new keyset")
	}
}

func TestManager(t *testing.T) {
	store := NewMemoryStore()
	manager := NewManager(store, DefaultActiveWindow, DefaultVerifyWindow)

	if err := manager.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Should have a signing keyset
	signing := manager.GetSigningKeyset()
	if signing == nil {
		t.Fatal("GetSigningKeyset returned nil")
	}

	// Should have at least one verification keyset
	verification := manager.GetVerificationKeysets()
	if len(verification) == 0 {
		t.Error("GetVerificationKeysets returned empty")
	}

	// Should find keyset by ID
	found := manager.FindByID(signing.ID)
	if found == nil {
		t.Error("FindByID returned nil for signing keyset")
	}
	if found.ID != signing.ID {
		t.Errorf("FindByID returned wrong keyset: %s != %s", found.ID, signing.ID)
	}
}

func TestManagerRotation(t *testing.T) {
	store := NewMemoryStore()
	manager := NewManager(store, 50*time.Millisecond, 200*time.Millisecond)

	if err := manager.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	initialID := manager.GetSigningKeyset().ID

	// Rotation should not happen yet
	rotated, err := manager.RotateIfNeeded()
	if err != nil {
		t.Fatalf("RotateIfNeeded failed: %v", err)
	}
	if rotated {
		t.Error("Should not rotate when keyset is still active")
	}

	// Wait for signing to expire
	time.Sleep(60 * time.Millisecond)

	// Now rotation should happen
	rotated, err = manager.RotateIfNeeded()
	if err != nil {
		t.Fatalf("RotateIfNeeded failed: %v", err)
	}
	if !rotated {
		t.Error("Should rotate when keyset is expired")
	}

	newID := manager.GetSigningKeyset().ID
	if newID == initialID {
		t.Error("New keyset should have different ID")
	}

	// Old keyset should still be valid for verification
	old := manager.FindByID(initialID)
	if old == nil {
		t.Error("Old keyset should still be found for verification")
	}
}

func TestManagerPersistence(t *testing.T) {
	store := NewMemoryStore()

	// First manager creates keyset
	m1 := NewManager(store, DefaultActiveWindow, DefaultVerifyWindow)
	if err := m1.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	id := m1.GetSigningKeyset().ID

	// Second manager should load existing keyset
	m2 := NewManager(store, DefaultActiveWindow, DefaultVerifyWindow)
	if err := m2.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if m2.GetSigningKeyset().ID != id {
		t.Error("Second manager should use same keyset as first")
	}
}

func TestManagerListKeysetInfo(t *testing.T) {
	store := NewMemoryStore()
	manager := NewManager(store, DefaultActiveWindow, DefaultVerifyWindow)
	manager.Init()

	infos := manager.ListKeysetInfo()
	if len(infos) == 0 {
		t.Error("ListKeysetInfo returned empty")
	}

	for _, info := range infos {
		if info.ID == "" {
			t.Error("KeysetInfo has empty ID")
		}
		if info.PublicKey == "" {
			t.Error("KeysetInfo has empty PublicKey")
		}
	}
}

func TestMemoryStore(t *testing.T) {
	store := NewMemoryStore()

	k, _ := New()

	// Save
	if err := store.SaveKeyset(k); err != nil {
		t.Fatalf("SaveKeyset failed: %v", err)
	}

	// Load
	loaded, err := store.LoadKeyset(k.ID)
	if err != nil {
		t.Fatalf("LoadKeyset failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadKeyset returned nil")
	}
	if loaded.ID != k.ID {
		t.Errorf("Loaded ID = %s, want %s", loaded.ID, k.ID)
	}

	// List active
	active, err := store.ListActiveKeysets()
	if err != nil {
		t.Fatalf("ListActiveKeysets failed: %v", err)
	}
	if len(active) != 1 {
		t.Errorf("ListActiveKeysets returned %d, want 1", len(active))
	}

	// Delete
	if err := store.DeleteKeyset(k.ID); err != nil {
		t.Fatalf("DeleteKeyset failed: %v", err)
	}

	// Should be gone
	loaded, _ = store.LoadKeyset(k.ID)
	if loaded != nil {
		t.Error("Keyset should be deleted")
	}
}
