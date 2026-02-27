package specialkinds

import (
	"context"
	"errors"
	"testing"

	"next.orly.dev/pkg/nostr/encoders/event"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry returned nil")
	}
	if r.HandlerCount() != 0 {
		t.Errorf("expected 0 handlers, got %d", r.HandlerCount())
	}
}

func TestRegistryRegister(t *testing.T) {
	r := NewRegistry()

	h := NewHandlerFunc(
		"test-handler",
		func(ev *event.E) bool { return true },
		func(ctx context.Context, ev *event.E, hctx *HandlerContext) Result {
			return Handled("test")
		},
	)

	r.Register(h)

	if r.HandlerCount() != 1 {
		t.Errorf("expected 1 handler, got %d", r.HandlerCount())
	}

	names := r.ListHandlers()
	if len(names) != 1 || names[0] != "test-handler" {
		t.Errorf("unexpected handler names: %v", names)
	}
}

func TestRegistryTryHandle_NoMatch(t *testing.T) {
	r := NewRegistry()

	// Register a handler that never matches
	h := NewHandlerFunc(
		"never-matches",
		func(ev *event.E) bool { return false },
		func(ctx context.Context, ev *event.E, hctx *HandlerContext) Result {
			return Handled("should not be called")
		},
	)
	r.Register(h)

	ev := &event.E{Kind: 1}
	result, matched := r.TryHandle(context.Background(), ev, nil)

	if matched {
		t.Error("expected no match, got matched")
	}
	if result != nil {
		t.Error("expected nil result when no match")
	}
}

func TestRegistryTryHandle_Match(t *testing.T) {
	r := NewRegistry()

	// Register a handler that matches kind 1
	h := NewHandlerFunc(
		"kind-1-handler",
		func(ev *event.E) bool { return ev.Kind == 1 },
		func(ctx context.Context, ev *event.E, hctx *HandlerContext) Result {
			return Handled("processed kind 1")
		},
	)
	r.Register(h)

	ev := &event.E{Kind: 1}
	result, matched := r.TryHandle(context.Background(), ev, nil)

	if !matched {
		t.Error("expected match, got no match")
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if !result.Handled {
		t.Error("expected Handled=true")
	}
	if result.Message != "processed kind 1" {
		t.Errorf("unexpected message: %s", result.Message)
	}
}

func TestRegistryTryHandle_FirstMatchWins(t *testing.T) {
	r := NewRegistry()

	// Register two handlers that both match kind 1
	h1 := NewHandlerFunc(
		"first-handler",
		func(ev *event.E) bool { return ev.Kind == 1 },
		func(ctx context.Context, ev *event.E, hctx *HandlerContext) Result {
			return Handled("first")
		},
	)
	h2 := NewHandlerFunc(
		"second-handler",
		func(ev *event.E) bool { return ev.Kind == 1 },
		func(ctx context.Context, ev *event.E, hctx *HandlerContext) Result {
			return Handled("second")
		},
	)
	r.Register(h1)
	r.Register(h2)

	ev := &event.E{Kind: 1}
	result, _ := r.TryHandle(context.Background(), ev, nil)

	if result.Message != "first" {
		t.Errorf("expected first handler to win, got message: %s", result.Message)
	}
}

func TestHandlerContext(t *testing.T) {
	r := NewRegistry()

	var capturedCtx *HandlerContext

	h := NewHandlerFunc(
		"context-capture",
		func(ev *event.E) bool { return true },
		func(ctx context.Context, ev *event.E, hctx *HandlerContext) Result {
			capturedCtx = hctx
			return Handled("captured")
		},
	)
	r.Register(h)

	hctx := &HandlerContext{
		AuthedPubkey: []byte{1, 2, 3},
		Remote:       "127.0.0.1:12345",
		ConnectionID: "conn-abc",
	}

	r.TryHandle(context.Background(), &event.E{Kind: 1}, hctx)

	if capturedCtx == nil {
		t.Fatal("handler context not captured")
	}
	if string(capturedCtx.AuthedPubkey) != string(hctx.AuthedPubkey) {
		t.Error("AuthedPubkey mismatch")
	}
	if capturedCtx.Remote != hctx.Remote {
		t.Error("Remote mismatch")
	}
	if capturedCtx.ConnectionID != hctx.ConnectionID {
		t.Error("ConnectionID mismatch")
	}
}

func TestResultConstructors(t *testing.T) {
	t.Run("Handled", func(t *testing.T) {
		r := Handled("msg")
		if !r.Handled || r.Continue || r.SaveEvent || r.Error != nil {
			t.Error("unexpected Handled result")
		}
		if r.Message != "msg" {
			t.Errorf("unexpected message: %s", r.Message)
		}
	})

	t.Run("HandledWithSave", func(t *testing.T) {
		r := HandledWithSave("msg")
		if !r.Handled || r.Continue || !r.SaveEvent || r.Error != nil {
			t.Error("unexpected HandledWithSave result")
		}
	})

	t.Run("ContinueProcessing", func(t *testing.T) {
		r := ContinueProcessing()
		if r.Handled || !r.Continue || r.SaveEvent || r.Error != nil {
			t.Error("unexpected ContinueProcessing result")
		}
	})

	t.Run("ErrorResult", func(t *testing.T) {
		err := errors.New("test error")
		r := ErrorResult(err)
		if r.Handled || r.Continue || r.SaveEvent || r.Error != err {
			t.Error("unexpected ErrorResult result")
		}
	})
}

func TestHandlerFuncInterface(t *testing.T) {
	h := NewHandlerFunc(
		"test",
		func(ev *event.E) bool { return ev.Kind == 42 },
		func(ctx context.Context, ev *event.E, hctx *HandlerContext) Result {
			return Handled("42")
		},
	)

	// Check interface compliance
	var _ Handler = h

	if h.Name() != "test" {
		t.Errorf("unexpected name: %s", h.Name())
	}

	if !h.CanHandle(&event.E{Kind: 42}) {
		t.Error("expected CanHandle to return true for kind 42")
	}
	if h.CanHandle(&event.E{Kind: 1}) {
		t.Error("expected CanHandle to return false for kind 1")
	}

	result := h.Handle(context.Background(), &event.E{Kind: 42}, nil)
	if result.Message != "42" {
		t.Errorf("unexpected result message: %s", result.Message)
	}
}
