package bridge

import (
	"archive/zip"
	"bytes"
	"testing"

	bridgesmtp "next.orly.dev/pkg/bridge/smtp"
)

func TestZipParts_HTMLOnly(t *testing.T) {
	data, err := ZipParts("<html><body>Hello</body></html>", nil)
	if err != nil {
		t.Fatalf("ZipParts: %v", err)
	}

	// Verify it's a valid zip
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("read zip: %v", err)
	}

	if len(r.File) != 1 {
		t.Fatalf("expected 1 file, got %d", len(r.File))
	}
	if r.File[0].Name != "body.html" {
		t.Errorf("file name = %q, want body.html", r.File[0].Name)
	}
}

func TestZipParts_WithAttachments(t *testing.T) {
	attachments := []bridgesmtp.Attachment{
		{Filename: "test.pdf", ContentType: "application/pdf", Data: []byte("pdf-content")},
		{Filename: "image.png", ContentType: "image/png", Data: []byte("png-content")},
	}

	data, err := ZipParts("", attachments)
	if err != nil {
		t.Fatalf("ZipParts: %v", err)
	}

	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("read zip: %v", err)
	}

	if len(r.File) != 2 {
		t.Fatalf("expected 2 files, got %d", len(r.File))
	}

	names := map[string]bool{}
	for _, f := range r.File {
		names[f.Name] = true
	}

	if !names["test.pdf"] {
		t.Error("missing test.pdf in zip")
	}
	if !names["image.png"] {
		t.Error("missing image.png in zip")
	}
}

func TestZipParts_HTMLAndAttachments(t *testing.T) {
	attachments := []bridgesmtp.Attachment{
		{Filename: "doc.txt", ContentType: "text/plain", Data: []byte("content")},
	}

	data, err := ZipParts("<p>HTML</p>", attachments)
	if err != nil {
		t.Fatalf("ZipParts: %v", err)
	}

	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("read zip: %v", err)
	}

	if len(r.File) != 2 { // body.html + doc.txt
		t.Errorf("expected 2 files, got %d", len(r.File))
	}
}

func TestZipParts_EmptyFilename(t *testing.T) {
	attachments := []bridgesmtp.Attachment{
		{Filename: "", ContentType: "application/octet-stream", Data: []byte("data")},
	}

	data, err := ZipParts("", attachments)
	if err != nil {
		t.Fatalf("ZipParts: %v", err)
	}

	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("read zip: %v", err)
	}

	if r.File[0].Name != "attachment" {
		t.Errorf("fallback name = %q, want 'attachment'", r.File[0].Name)
	}
}
