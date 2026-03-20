package sha256

import "testing"

func TestSumEmpty(t *testing.T) {
	got := SumHex(nil)
	want := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if got != want {
		t.Errorf("SHA256('')\ngot  %s\nwant %s", got, want)
	}
}

func TestSumABC(t *testing.T) {
	got := SumHex([]byte("abc"))
	want := "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got != want {
		t.Errorf("SHA256('abc')\ngot  %s\nwant %s", got, want)
	}
}

func TestSumLong(t *testing.T) {
	// 56 bytes: triggers two blocks due to padding.
	data := []byte("abcdbcdecdefdefgefghfghighijhijkijkljklmklmnlmnomnopnopq")
	got := SumHex(data)
	want := "248d6a61d20638b8e5c026930c3e6039a33ce45964ff2167f6ecedd419db06c1"
	if got != want {
		t.Errorf("SHA256(56-byte)\ngot  %s\nwant %s", got, want)
	}
}
