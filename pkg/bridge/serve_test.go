package bridge

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestComposeHandler(t *testing.T) {
	handler := ComposeHandler()

	req := httptest.NewRequest("GET", "/compose", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Marmot Email Bridge") {
		t.Error("compose page missing title")
	}
	if !strings.Contains(body, "copyToClipboard") {
		t.Error("compose page missing JS function")
	}
}

func TestDecryptHandler(t *testing.T) {
	handler := DecryptHandler()

	req := httptest.NewRequest("GET", "/decrypt", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Decrypt") {
		t.Error("decrypt page missing title")
	}
	if !strings.Contains(body, "ChaCha20") {
		t.Error("decrypt page missing crypto reference")
	}
}
