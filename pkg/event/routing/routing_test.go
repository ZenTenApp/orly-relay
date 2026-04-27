package routing

import (
	"errors"
	"testing"

	"git.smesh.lol/orly/pkg/nostr/encoders/event"
)

func TestNew(t *testing.T) {
	r := New()
	if r == nil {
		t.Fatal("New() returned nil")
	}
	if r.handlers == nil {
		t.Fatal("handlers map is nil")
	}
	if r.kindChecks == nil {
		t.Fatal("kindChecks slice is nil")
	}
}

func TestResultConstructors(t *testing.T) {
	// ContinueResult
	r := ContinueResult()
	if r.Action != Continue {
		t.Error("ContinueResult should have Action=Continue")
	}

	// HandledResult
	r = HandledResult("success")
	if r.Action != Handled {
		t.Error("HandledResult should have Action=Handled")
	}
	if r.Message != "success" {
		t.Error("HandledResult should preserve message")
	}

	// ErrorResult
	err := errors.New("test error")
	r = ErrorResult(err)
	if r.Action != Error {
		t.Error("ErrorResult should have Action=Error")
	}
	if r.Error != err {
		t.Error("ErrorResult should preserve error")
	}
}

func TestDefaultRouter_Register(t *testing.T) {
	r := New()

	called := false
	handler := func(ev *event.E, authedPubkey []byte) Result {
		called = true
		return HandledResult("handled")
	}

	r.Register(1, handler)

	ev := event.New()
	ev.Kind = 1

	result := r.Route(ev, nil)
	if !called {
		t.Error("handler should have been called")
	}
	if result.Action != Handled {
		t.Error("result should be Handled")
	}
}

func TestDefaultRouter_RegisterKindCheck(t *testing.T) {
	r := New()

	called := false
	handler := func(ev *event.E, authedPubkey []byte) Result {
		called = true
		return HandledResult("ephemeral")
	}

	// Register handler for ephemeral events (20000-29999)
	r.RegisterKindCheck("ephemeral", func(k uint16) bool {
		return k >= 20000 && k < 30000
	}, handler)

	ev := event.New()
	ev.Kind = 20001

	result := r.Route(ev, nil)
	if !called {
		t.Error("kind check handler should have been called")
	}
	if result.Action != Handled {
		t.Error("result should be Handled")
	}
}

func TestDefaultRouter_NoMatch(t *testing.T) {
	r := New()

	// Register handler for kind 1
	r.Register(1, func(ev *event.E, authedPubkey []byte) Result {
		return HandledResult("kind 1")
	})

	ev := event.New()
	ev.Kind = 2 // Different kind

	result := r.Route(ev, nil)
	if result.Action != Continue {
		t.Error("unmatched kind should return Continue")
	}
}

func TestDefaultRouter_ExactMatchPriority(t *testing.T) {
	r := New()

	exactCalled := false
	checkCalled := false

	// Register exact match for kind 20001
	r.Register(20001, func(ev *event.E, authedPubkey []byte) Result {
		exactCalled = true
		return HandledResult("exact")
	})

	// Register kind check for ephemeral (also matches 20001)
	r.RegisterKindCheck("ephemeral", func(k uint16) bool {
		return k >= 20000 && k < 30000
	}, func(ev *event.E, authedPubkey []byte) Result {
		checkCalled = true
		return HandledResult("check")
	})

	ev := event.New()
	ev.Kind = 20001

	result := r.Route(ev, nil)
	if !exactCalled {
		t.Error("exact match should be called")
	}
	if checkCalled {
		t.Error("kind check should not be called when exact match exists")
	}
	if result.Message != "exact" {
		t.Errorf("expected 'exact', got '%s'", result.Message)
	}
}

func TestDefaultRouter_HasHandler(t *testing.T) {
	r := New()

	// Initially no handlers
	if r.HasHandler(1) {
		t.Error("should not have handler for kind 1 yet")
	}

	// Register exact handler
	r.Register(1, func(ev *event.E, authedPubkey []byte) Result {
		return HandledResult("")
	})

	if !r.HasHandler(1) {
		t.Error("should have handler for kind 1")
	}

	// Register kind check for ephemeral
	r.RegisterKindCheck("ephemeral", func(k uint16) bool {
		return k >= 20000 && k < 30000
	}, func(ev *event.E, authedPubkey []byte) Result {
		return HandledResult("")
	})

	if !r.HasHandler(20001) {
		t.Error("should have handler for ephemeral kind 20001")
	}

	if r.HasHandler(19999) {
		t.Error("should not have handler for kind 19999")
	}
}

func TestDefaultRouter_PassesPubkey(t *testing.T) {
	r := New()

	var receivedPubkey []byte
	r.Register(1, func(ev *event.E, authedPubkey []byte) Result {
		receivedPubkey = authedPubkey
		return HandledResult("")
	})

	testPubkey := []byte("testpubkey12345")
	ev := event.New()
	ev.Kind = 1

	r.Route(ev, testPubkey)

	if string(receivedPubkey) != string(testPubkey) {
		t.Error("handler should receive the authed pubkey")
	}
}

func TestDefaultRouter_MultipleKindChecks(t *testing.T) {
	r := New()

	firstCalled := false
	secondCalled := false

	// First check matches 10000-19999
	r.RegisterKindCheck("first", func(k uint16) bool {
		return k >= 10000 && k < 20000
	}, func(ev *event.E, authedPubkey []byte) Result {
		firstCalled = true
		return HandledResult("first")
	})

	// Second check matches 15000-25000 (overlaps)
	r.RegisterKindCheck("second", func(k uint16) bool {
		return k >= 15000 && k < 25000
	}, func(ev *event.E, authedPubkey []byte) Result {
		secondCalled = true
		return HandledResult("second")
	})

	// Kind 15000 matches both - first registered wins
	ev := event.New()
	ev.Kind = 15000

	result := r.Route(ev, nil)
	if !firstCalled {
		t.Error("first check should be called")
	}
	if secondCalled {
		t.Error("second check should not be called")
	}
	if result.Message != "first" {
		t.Errorf("expected 'first', got '%s'", result.Message)
	}
}
