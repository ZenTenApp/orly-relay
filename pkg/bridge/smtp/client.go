package smtp

import (
	"bytes"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/emersion/go-message"
	gosmtp "github.com/emersion/go-smtp"
	"lol.mleku.dev/log"
)

// ClientConfig holds outbound SMTP client configuration.
type ClientConfig struct {
	// FromDomain is the bridge's email domain for the From address.
	FromDomain string
	// DKIMSigner signs outbound messages. Nil disables DKIM.
	DKIMSigner *DKIMSigner
}

// Client sends outbound emails.
type Client struct {
	cfg ClientConfig
}

// NewClient creates an outbound SMTP client.
func NewClient(cfg ClientConfig) *Client {
	return &Client{cfg: cfg}
}

// OutboundEmail represents an email to be sent.
type OutboundEmail struct {
	From    string   // e.g., "npub1abc@relay.example.com"
	To      []string // recipient email addresses
	Cc      []string
	Bcc     []string
	Subject string
	Body    string
}

// Send delivers an outbound email via direct MX lookup.
func (c *Client) Send(email *OutboundEmail) error {
	// Build the message
	msg, err := c.buildMessage(email)
	if err != nil {
		return fmt.Errorf("build message: %w", err)
	}

	// Sign with DKIM if configured
	var msgBytes []byte
	if c.cfg.DKIMSigner != nil {
		signed, err := c.cfg.DKIMSigner.Sign(msg)
		if err != nil {
			log.W.F("DKIM signing failed, sending unsigned: %v", err)
			msgBytes = msg
		} else {
			msgBytes = signed
		}
	} else {
		msgBytes = msg
	}

	// Collect all recipients
	allRecipients := make([]string, 0, len(email.To)+len(email.Cc)+len(email.Bcc))
	allRecipients = append(allRecipients, email.To...)
	allRecipients = append(allRecipients, email.Cc...)
	allRecipients = append(allRecipients, email.Bcc...)

	// Group recipients by domain for MX delivery
	byDomain := groupByDomain(allRecipients)

	var lastErr error
	for domain, addrs := range byDomain {
		if err := c.deliverToMX(domain, email.From, addrs, msgBytes); err != nil {
			log.E.F("failed to deliver to %s: %v", domain, err)
			lastErr = err
		}
	}

	return lastErr
}

func (c *Client) buildMessage(email *OutboundEmail) ([]byte, error) {
	var buf bytes.Buffer

	h := message.Header{}
	h.Set("From", email.From)
	h.Set("To", strings.Join(email.To, ", "))
	if len(email.Cc) > 0 {
		h.Set("Cc", strings.Join(email.Cc, ", "))
	}
	// BCC is NOT included in headers (that's the point of BCC)
	h.Set("Subject", email.Subject)
	h.Set("Date", time.Now().Format("Mon, 02 Jan 2006 15:04:05 -0700"))
	h.Set("Message-Id", fmt.Sprintf("<%d.bridge@%s>", time.Now().UnixNano(), c.cfg.FromDomain))
	h.Set("MIME-Version", "1.0")
	h.SetContentType("text/plain", map[string]string{"charset": "utf-8"})

	w, err := message.CreateWriter(&buf, h)
	if err != nil {
		return nil, fmt.Errorf("create writer: %w", err)
	}
	if _, err := w.Write([]byte(email.Body)); err != nil {
		return nil, fmt.Errorf("write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("close writer: %w", err)
	}

	return buf.Bytes(), nil
}

// deliverToMX looks up the MX records for a domain and delivers the message.
func (c *Client) deliverToMX(domain, from string, to []string, msg []byte) error {
	mxRecords, err := net.LookupMX(domain)
	if err != nil {
		return fmt.Errorf("MX lookup for %s: %w", domain, err)
	}

	if len(mxRecords) == 0 {
		// Fall back to A record
		mxRecords = []*net.MX{{Host: domain, Pref: 0}}
	}

	var lastErr error
	for _, mx := range mxRecords {
		host := strings.TrimSuffix(mx.Host, ".")
		addr := net.JoinHostPort(host, "25")

		if err := c.deliverDirect(addr, from, to, msg); err != nil {
			lastErr = err
			log.D.F("MX %s failed: %v, trying next", host, err)
			continue
		}
		return nil // success
	}

	return fmt.Errorf("all MX servers failed for %s: %w", domain, lastErr)
}

// deliverDirect sends the message to a specific SMTP server.
func (c *Client) deliverDirect(addr, from string, to []string, msg []byte) error {
	cl, err := gosmtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("dial %s: %w", addr, err)
	}
	defer cl.Quit()

	if err := cl.Hello(c.cfg.FromDomain); err != nil {
		return fmt.Errorf("HELO: %w", err)
	}

	if err := cl.Mail(from, nil); err != nil {
		return fmt.Errorf("MAIL FROM: %w", err)
	}

	for _, rcpt := range to {
		if err := cl.Rcpt(rcpt, nil); err != nil {
			return fmt.Errorf("RCPT TO %s: %w", rcpt, err)
		}
	}

	w, err := cl.Data()
	if err != nil {
		return fmt.Errorf("DATA: %w", err)
	}

	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("write data: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("close data: %w", err)
	}

	log.I.F("SMTP delivered to %s via %s", to, addr)
	return nil
}

// groupByDomain groups email addresses by their domain part.
func groupByDomain(addrs []string) map[string][]string {
	result := make(map[string][]string)
	for _, addr := range addrs {
		parts := strings.SplitN(addr, "@", 2)
		if len(parts) == 2 {
			domain := strings.ToLower(parts[1])
			result[domain] = append(result[domain], addr)
		}
	}
	return result
}
