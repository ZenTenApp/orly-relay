package p256k1

import (
	"crypto/rand"
	"testing"
)

func TestContextCreate(t *testing.T) {
	// Test creating context with different flags
	testCases := []struct {
		name  string
		flags uint
	}{
		{"none", ContextNone},
		{"sign", ContextSign},
		{"verify", ContextVerify},
		{"both", ContextSign | ContextVerify},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := ContextCreate(tc.flags)
			if ctx == nil {
				t.Error("ContextCreate should not return nil")
			}
			if ctx.flags != tc.flags {
				t.Errorf("context flags should be %d, got %d", tc.flags, ctx.flags)
			}

			// Check capabilities
			expectedCanSign := (tc.flags & ContextSign) != 0
			expectedCanVerify := (tc.flags & ContextVerify) != 0

			if ctx.canSign() != expectedCanSign {
				t.Errorf("canSign() should be %v", expectedCanSign)
			}
			if ctx.canVerify() != expectedCanVerify {
				t.Errorf("canVerify() should be %v", expectedCanVerify)
			}

			// Clean up
			ContextDestroy(ctx)
		})
	}
}

func TestContextDestroy(t *testing.T) {
	// Test destroying nil context (should not panic)
	ContextDestroy(nil)

	// Test destroying valid context
	ctx := ContextCreate(ContextSign | ContextVerify)
	ContextDestroy(ctx)

	// After destruction, context should be cleared
	if ctx.flags != 0 {
		t.Error("context flags should be cleared after destruction")
	}
	if ctx.ecmultGenCtx != nil {
		t.Error("ecmult_gen context should be cleared after destruction")
	}
}

func TestContextRandomize(t *testing.T) {
	ctx := ContextCreate(ContextSign | ContextVerify)
	defer ContextDestroy(ctx)

	// Test with nil seed (should generate random seed)
	err := ContextRandomize(ctx, nil)
	if err != nil {
		t.Errorf("ContextRandomize with nil seed failed: %v", err)
	}

	// Test with provided seed
	seed := make([]byte, 32)
	if _, err := rand.Read(seed); err != nil {
		t.Fatal(err)
	}

	err = ContextRandomize(ctx, seed)
	if err != nil {
		t.Errorf("ContextRandomize with seed failed: %v", err)
	}

	// Test with invalid seed length
	invalidSeed := make([]byte, 16) // Wrong length
	err = ContextRandomize(ctx, invalidSeed)
	if err == nil {
		t.Error("ContextRandomize should fail with invalid seed length")
	}

	// Test with nil context
	err = ContextRandomize(nil, seed)
	if err == nil {
		t.Error("ContextRandomize should fail with nil context")
	}
}

func TestContextStatic(t *testing.T) {
	// Test that static context exists and has correct properties
	if ContextStatic == nil {
		t.Error("ContextStatic should not be nil")
	}

	if ContextStatic.flags != ContextVerify {
		t.Errorf("ContextStatic should have ContextVerify flag, got %d", ContextStatic.flags)
	}

	if !ContextStatic.canVerify() {
		t.Error("ContextStatic should be able to verify")
	}

	if ContextStatic.canSign() {
		t.Error("ContextStatic should not be able to sign")
	}
}

func TestContextCapabilities(t *testing.T) {
	// Test signing context
	signCtx := ContextCreate(ContextSign)
	defer ContextDestroy(signCtx)

	if !signCtx.canSign() {
		t.Error("sign context should be able to sign")
	}
	if signCtx.canVerify() {
		t.Error("sign-only context should not be able to verify")
	}

	// Test verify context
	verifyCtx := ContextCreate(ContextVerify)
	defer ContextDestroy(verifyCtx)

	if verifyCtx.canSign() {
		t.Error("verify-only context should not be able to sign")
	}
	if !verifyCtx.canVerify() {
		t.Error("verify context should be able to verify")
	}

	// Test combined context
	bothCtx := ContextCreate(ContextSign | ContextVerify)
	defer ContextDestroy(bothCtx)

	if !bothCtx.canSign() {
		t.Error("combined context should be able to sign")
	}
	if !bothCtx.canVerify() {
		t.Error("combined context should be able to verify")
	}

	// Test none context
	noneCtx := ContextCreate(ContextNone)
	defer ContextDestroy(noneCtx)

	if noneCtx.canSign() {
		t.Error("none context should not be able to sign")
	}
	if noneCtx.canVerify() {
		t.Error("none context should not be able to verify")
	}
}

func BenchmarkContextCreate(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := ContextCreate(ContextSign | ContextVerify)
		ContextDestroy(ctx)
	}
}

func BenchmarkContextRandomize(b *testing.B) {
	ctx := ContextCreate(ContextSign | ContextVerify)
	defer ContextDestroy(ctx)

	seed := make([]byte, 32)
	rand.Read(seed)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ContextRandomize(ctx, seed)
	}
}
