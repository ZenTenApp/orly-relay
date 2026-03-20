package helpers

import "testing"

func TestBech32RoundTrip(t *testing.T) {
	data := HexDecode("79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798")
	encoded := EncodeNpub(data)
	t.Logf("npub: %s", encoded)

	decoded := DecodeNpub(encoded)
	if decoded == nil {
		t.Fatal("DecodeNpub returned nil")
	}
	got := HexEncode(decoded)
	want := "79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"
	if got != want {
		t.Errorf("round-trip failed:\ngot  %s\nwant %s", got, want)
	}
}

func TestNpubKnown(t *testing.T) {
	// Decode a known npub.
	npub := "npub1fjqqy4a93z5zsjwsfxqhc2764kvykfdyttvldkkkdera8dr78vhsmmleku"
	data := DecodeNpub(npub)
	if data == nil {
		t.Fatal("failed to decode npub")
	}
	hex := HexEncode(data)
	t.Logf("pubkey hex: %s", hex)
	if len(hex) != 64 {
		t.Errorf("expected 64 hex chars, got %d", len(hex))
	}
}
