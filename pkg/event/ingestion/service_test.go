package ingestion

import (
	"testing"
)

func TestResultConstructors(t *testing.T) {
	t.Run("Accepted", func(t *testing.T) {
		r := Accepted("msg")
		if !r.Accepted || !r.Saved || r.RequireAuth || r.Error != nil {
			t.Errorf("unexpected Accepted result: %+v", r)
		}
		if r.Message != "msg" {
			t.Errorf("unexpected message: %s", r.Message)
		}
	})

	t.Run("AcceptedNotSaved", func(t *testing.T) {
		r := AcceptedNotSaved("msg")
		if !r.Accepted || r.Saved || r.RequireAuth || r.Error != nil {
			t.Error("unexpected AcceptedNotSaved result")
		}
	})

	t.Run("Rejected", func(t *testing.T) {
		r := Rejected("msg")
		if r.Accepted || r.Saved || r.RequireAuth || r.Error != nil {
			t.Error("unexpected Rejected result")
		}
		if r.Message != "msg" {
			t.Errorf("unexpected message: %s", r.Message)
		}
	})

	t.Run("AuthRequired", func(t *testing.T) {
		r := AuthRequired("msg")
		if r.Accepted || r.Saved || !r.RequireAuth || r.Error != nil {
			t.Error("unexpected AuthRequired result")
		}
	})

	t.Run("Errored", func(t *testing.T) {
		err := &testError{msg: "test"}
		r := Errored(err)
		if r.Accepted || r.Saved || r.RequireAuth || r.Error != err {
			t.Error("unexpected Errored result")
		}
	})
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestConnectionContext(t *testing.T) {
	ctx := &ConnectionContext{
		AuthedPubkey: []byte{1, 2, 3},
		Remote:       "127.0.0.1:1234",
		ConnectionID: "conn-123",
	}

	if string(ctx.AuthedPubkey) != string([]byte{1, 2, 3}) {
		t.Error("AuthedPubkey mismatch")
	}
	if ctx.Remote != "127.0.0.1:1234" {
		t.Error("Remote mismatch")
	}
	if ctx.ConnectionID != "conn-123" {
		t.Error("ConnectionID mismatch")
	}
}

func TestConfig(t *testing.T) {
	cfg := Config{
		SprocketChecker: nil,
		SpecialKinds:    nil,
	}

	// Just verify Config can be created
	if cfg.SprocketChecker != nil {
		t.Error("expected nil SprocketChecker")
	}
}
