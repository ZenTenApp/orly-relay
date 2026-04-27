package nwc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/pkg/nostr/crypto/encryption"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/kind"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/nostr/encoders/timestamp"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer"
	"git.smesh.lol/orly/pkg/nostr/ws"
)

type Client struct {
	relay           string
	clientSecretKey signer.I
	walletPublicKey []byte
	conversationKey []byte
}

func NewClient(connectionURI string) (cl *Client, err error) {
	var parts *ConnectionParams
	if parts, err = ParseConnectionURI(connectionURI); chk.E(err) {
		return
	}
	cl = &Client{
		relay:           parts.relay,
		clientSecretKey: parts.clientSecretKey,
		walletPublicKey: parts.walletPublicKey,
		conversationKey: parts.conversationKey,
	}
	return
}

func (cl *Client) Request(
	c context.Context, method string, params, result any,
) (err error) {
	delay := time.Second
	const maxRetries = 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			log.D.F("NWC request %s retry %d/%d (delay %v)",
				method, attempt, maxRetries-1, delay)
			select {
			case <-time.After(delay):
				delay *= 2
			case <-c.Done():
				return c.Err()
			}
		}
		err = cl.requestOnce(c, method, params, result)
		if err == nil {
			return nil
		}
		log.W.F("NWC request %s attempt %d failed: %v", method, attempt+1, err)
	}
	return
}

func (cl *Client) requestOnce(
	c context.Context, method string, params, result any,
) (err error) {
	ctx, cancel := context.WithTimeout(c, 30*time.Second)
	defer cancel()

	request := map[string]any{"method": method}
	if params != nil {
		request["params"] = params
	}

	var req []byte
	if req, err = json.Marshal(request); chk.E(err) {
		return
	}

	var content string
	if content, err = encryption.Encrypt(cl.conversationKey, req, nil); chk.E(err) {
		return
	}

	ev := &event.E{
		Content:   []byte(content),
		CreatedAt: time.Now().Unix(),
		Kind:      23194,
		Tags: tag.NewS(
			tag.NewFromAny("encryption", "nip44_v2"),
			tag.NewFromAny("p", hex.Enc(cl.walletPublicKey)),
		),
	}

	if err = ev.Sign(cl.clientSecretKey); chk.E(err) {
		return
	}

	var rc *ws.Client
	if rc, err = ws.RelayConnect(ctx, cl.relay); chk.E(err) {
		return
	}
	defer rc.Close()

	// Filter must include authors (wallet pubkey) and #p tag (our client
	// pubkey) — NWC relays like Alby require this and will CLOSE the
	// subscription without it. Use Since 5 seconds ago to avoid missing
	// fast responses.
	since := time.Now().Unix() - 5
	var sub *ws.Subscription
	if sub, err = rc.Subscribe(
		ctx, filter.NewS(
			&filter.F{
				Kinds:   kind.NewS(kind.New(23195)),
				Authors: tag.NewFromAny(cl.walletPublicKey),
				Tags: tag.NewS(
					tag.NewFromAny("p", hex.Enc(cl.clientSecretKey.Pub())),
				),
				Since: &timestamp.T{V: since},
			},
		),
	); chk.E(err) {
		return
	}
	defer sub.Unsub()

	if err = rc.Publish(ctx, ev); chk.E(err) {
		return fmt.Errorf("publish failed: %w", err)
	}

	select {
	case <-ctx.Done():
		return fmt.Errorf("no response from wallet (connection may be inactive)")
	case reason := <-sub.ClosedReason:
		return fmt.Errorf("relay closed subscription: %s", reason)
	case e := <-sub.Events:
		if e == nil {
			return fmt.Errorf("subscription closed (wallet connection inactive)")
		}
		if len(e.Content) == 0 {
			return fmt.Errorf("empty response content")
		}
		var raw string
		if raw, err = encryption.Decrypt(
			cl.conversationKey, string(e.Content),
		); chk.E(err) {
			return fmt.Errorf(
				"decryption failed (invalid conversation key): %w", err,
			)
		}

		var resp map[string]any
		if err = json.Unmarshal([]byte(raw), &resp); chk.E(err) {
			return
		}

		if errData, ok := resp["error"].(map[string]any); ok {
			code, _ := errData["code"].(string)
			msg, _ := errData["message"].(string)
			return fmt.Errorf("%s: %s", code, msg)
		}

		if result != nil && resp["result"] != nil {
			var resultBytes []byte
			if resultBytes, err = json.Marshal(resp["result"]); chk.E(err) {
				return
			}
			if err = json.Unmarshal(resultBytes, result); chk.E(err) {
				return
			}
		}
	}

	return
}

// NotificationHandler is a callback for handling NWC notifications
type NotificationHandler func(
	notificationType string, notification map[string]any,
) error

// SubscribeNotifications subscribes to NWC notification events (kinds 23197/23196)
// and handles them with the provided callback. It maintains a persistent connection
// with auto-reconnection on disconnect.
func (cl *Client) SubscribeNotifications(
	c context.Context, handler NotificationHandler,
) (err error) {
	delay := time.Second
	for {
		if err = cl.subscribeNotificationsOnce(c, handler); err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			select {
			case <-time.After(delay):
				if delay < 30*time.Second {
					delay *= 2
				}
			case <-c.Done():
				return context.Canceled
			}
			continue
		}
		delay = time.Second
	}
}

// subscribeNotificationsOnce performs a single subscription attempt
func (cl *Client) subscribeNotificationsOnce(
	c context.Context, handler NotificationHandler,
) (err error) {
	// Connect to relay
	var rc *ws.Client
	if rc, err = ws.RelayConnect(c, cl.relay); chk.E(err) {
		return fmt.Errorf("relay connection failed: %w", err)
	}
	defer rc.Close()

	// Subscribe to notification events. Filter must include authors (wallet
	// pubkey) and #p tag (our client pubkey) — NWC relays require this.
	var sub *ws.Subscription
	if sub, err = rc.Subscribe(
		c, filter.NewS(
			&filter.F{
				Kinds:   kind.NewS(kind.New(23197), kind.New(23196)),
				Authors: tag.NewFromAny(cl.walletPublicKey),
				Tags: tag.NewS(
					tag.NewFromAny("p", hex.Enc(cl.clientSecretKey.Pub())),
				),
				Since: &timestamp.T{V: time.Now().Unix() - 5},
			},
		),
	); chk.E(err) {
		return fmt.Errorf("subscription failed: %w", err)
	}
	defer sub.Unsub()

	log.D.F(
		"subscribed to NWC notifications from wallet %s",
		hex.Enc(cl.walletPublicKey),
	)

	// Process notification events
	for {
		select {
		case <-c.Done():
			return context.Canceled
		case ev := <-sub.Events:
			if ev == nil {
				// Channel closed, subscription ended
				return fmt.Errorf("subscription closed")
			}

			// Process the notification event
			if err := cl.processNotificationEvent(ev, handler); err != nil {
				log.E.F("error processing notification: %v", err)
				// Continue processing other notifications even if one fails
			}
		}
	}
}

// processNotificationEvent decrypts and processes a single notification event
func (cl *Client) processNotificationEvent(
	ev *event.E, handler NotificationHandler,
) (err error) {
	// Decrypt the notification content
	var decrypted string
	if decrypted, err = encryption.Decrypt(
		cl.conversationKey, string(ev.Content),
	); err != nil {
		return fmt.Errorf("failed to decrypt notification: %w", err)
	}

	// Parse the notification JSON
	var notification map[string]any
	if err = json.Unmarshal([]byte(decrypted), &notification); err != nil {
		return fmt.Errorf("failed to parse notification JSON: %w", err)
	}

	// Extract notification type
	notificationType, ok := notification["notification_type"].(string)
	if !ok {
		return fmt.Errorf("missing or invalid notification_type")
	}

	// Extract notification data
	notificationData, ok := notification["notification"].(map[string]any)
	if !ok {
		return fmt.Errorf("missing or invalid notification data")
	}

	// Route to type-specific handler
	return handler(notificationType, notificationData)
}
