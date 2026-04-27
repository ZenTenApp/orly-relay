package smtp

import (
	"fmt"
	"io"
	"strings"
	"time"

	gosmtp "github.com/emersion/go-smtp"
	"git.smesh.lol/orly/pkg/lol/log"
)

// backend implements gosmtp.Backend.
type backend struct {
	domain  string
	handler InboundHandler
}

func (b *backend) NewSession(c *gosmtp.Conn) (gosmtp.Session, error) {
	return &session{
		domain:  b.domain,
		handler: b.handler,
	}, nil
}

// session implements gosmtp.Session for a single SMTP transaction.
type session struct {
	domain  string
	handler InboundHandler
	from    string
	to      []string
}

// Mail is called when the client issues MAIL FROM.
func (s *session) Mail(from string, opts *gosmtp.MailOptions) error {
	s.from = from
	log.D.F("SMTP MAIL FROM: %s", from)
	return nil
}

// Rcpt is called when the client issues RCPT TO.
// Validates that the recipient is in the bridge's domain.
func (s *session) Rcpt(to string, opts *gosmtp.RcptOptions) error {
	// Extract domain from recipient
	parts := strings.SplitN(to, "@", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid recipient: %s", to)
	}

	recipientDomain := strings.ToLower(parts[1])
	if recipientDomain != strings.ToLower(s.domain) {
		return fmt.Errorf("relay access denied for domain %s", recipientDomain)
	}

	// The local part should be an npub or hex pubkey.
	// We validate format here but defer pubkey resolution to the handler.
	localPart := strings.ToLower(parts[0])
	if localPart == "" {
		return fmt.Errorf("empty local part in recipient")
	}

	s.to = append(s.to, to)
	log.D.F("SMTP RCPT TO: %s", to)
	return nil
}

// Data is called when the client sends the message body.
func (s *session) Data(r io.Reader) error {
	if len(s.to) == 0 {
		return fmt.Errorf("no valid recipients")
	}

	// Read the full message
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read message: %w", err)
	}

	email := &InboundEmail{
		From:       s.from,
		To:         s.to,
		RawMessage: data,
		ReceivedAt: time.Now(),
	}

	log.D.F("SMTP received message from %s to %v (%d bytes)", s.from, s.to, len(data))

	if err := s.handler(email); err != nil {
		log.E.F("handler error for message from %s: %v", s.from, err)
		return fmt.Errorf("message processing failed: %w", err)
	}

	return nil
}

// Reset is called on RSET command.
func (s *session) Reset() {
	s.from = ""
	s.to = nil
}

// Logout is called when the connection is closed.
func (s *session) Logout() error {
	return nil
}
