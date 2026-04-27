package bridge

import (
	"archive/zip"
	"bytes"
	"fmt"

	bridgesmtp "git.smesh.lol/orly/pkg/bridge/smtp"
)

const maxZipBytes = 25 * 1024 * 1024 // 25MB per spec

// ZipParts bundles non-plaintext MIME parts (HTML + attachments) into a
// flat zip archive. The text/plain body is excluded — it goes directly
// into the DM content.
func ZipParts(htmlBody string, attachments []bridgesmtp.Attachment) ([]byte, error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	if htmlBody != "" {
		f, err := w.Create("body.html")
		if err != nil {
			return nil, fmt.Errorf("create body.html: %w", err)
		}
		if _, err := f.Write([]byte(htmlBody)); err != nil {
			return nil, fmt.Errorf("write body.html: %w", err)
		}
	}

	for _, att := range attachments {
		name := att.Filename
		if name == "" {
			name = "attachment"
		}

		f, err := w.Create(name)
		if err != nil {
			return nil, fmt.Errorf("create %s: %w", name, err)
		}
		if _, err := f.Write(att.Data); err != nil {
			return nil, fmt.Errorf("write %s: %w", name, err)
		}
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("close zip: %w", err)
	}

	if buf.Len() > maxZipBytes {
		return nil, fmt.Errorf("zip exceeds %d byte limit (%d bytes)", maxZipBytes, buf.Len())
	}

	return buf.Bytes(), nil
}
