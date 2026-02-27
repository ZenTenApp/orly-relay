//go:build !(js && wasm)

package database

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"time"

	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/log"

	"next.orly.dev/pkg/nostr/crypto/keys"
	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/filter"
	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/nostr/encoders/kind"
	"next.orly.dev/pkg/nostr/encoders/tag"
	"next.orly.dev/pkg/nostr/interfaces/signer/p8k"
)

// NRC connection event kind - using application-specific data range
// Kind 30078 is commonly used for app-specific data
const KindNRCConnection = uint16(30078)

// NRCEventStore provides NRC connection management using events.
// This works with any Database implementation that supports SaveEvent/QueryEvents.
type NRCEventStore struct {
	db          Database
	relaySigner *p8k.Signer
	relayPubkey []byte
}

// NewNRCEventStore creates a new event-based NRC store.
// relaySigner is used to sign NRC connection events.
func NewNRCEventStore(db Database, relaySigner *p8k.Signer) *NRCEventStore {
	return &NRCEventStore{
		db:          db,
		relaySigner: relaySigner,
		relayPubkey: relaySigner.Pub(),
	}
}

// nrcConnectionContent is the JSON structure stored in event content
type nrcConnectionContent struct {
	ID            string `json:"id"`
	Label         string `json:"label"`
	Secret        string `json:"secret"`         // hex encoded
	DerivedPubkey string `json:"derived_pubkey"` // hex encoded
	RendezvousURL string `json:"rendezvous_url"` // WebSocket URL of rendezvous relay
	CreatedAt     int64  `json:"created_at"`
	LastUsed      int64  `json:"last_used"`
	CreatedBy     string `json:"created_by"` // hex encoded
}

// CreateNRCConnection generates a new NRC connection with a random secret.
func (s *NRCEventStore) CreateNRCConnection(label string, rendezvousURL string, createdBy []byte) (*NRCConnection, error) {
	// Generate random 32-byte secret
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("failed to generate random secret: %w", err)
	}

	// Derive pubkey from secret
	derivedPubkey, err := keys.SecretBytesToPubKeyBytes(secret)
	if err != nil {
		return nil, fmt.Errorf("failed to derive pubkey from secret: %w", err)
	}

	// Use first 8 bytes of secret as ID (hex encoded = 16 chars)
	id := string(hex.Enc(secret[:8]))

	conn := &NRCConnection{
		ID:            id,
		Label:         label,
		Secret:        secret,
		DerivedPubkey: derivedPubkey,
		RendezvousURL: rendezvousURL,
		CreatedAt:     time.Now().Unix(),
		LastUsed:      0,
		CreatedBy:     createdBy,
	}

	if err := s.SaveNRCConnection(conn); chk.E(err) {
		return nil, err
	}

	log.I.F("created NRC connection: id=%s label=%s rendezvous=%s", id, label, rendezvousURL)
	return conn, nil
}

// SaveNRCConnection stores an NRC connection as an event.
func (s *NRCEventStore) SaveNRCConnection(conn *NRCConnection) error {
	// Create content JSON
	content := nrcConnectionContent{
		ID:            conn.ID,
		Label:         conn.Label,
		Secret:        string(hex.Enc(conn.Secret)),
		DerivedPubkey: string(hex.Enc(conn.DerivedPubkey)),
		RendezvousURL: conn.RendezvousURL,
		CreatedAt:     conn.CreatedAt,
		LastUsed:      conn.LastUsed,
	}
	if len(conn.CreatedBy) > 0 {
		content.CreatedBy = string(hex.Enc(conn.CreatedBy))
	}

	contentJSON, err := json.Marshal(content)
	if err != nil {
		return fmt.Errorf("failed to marshal NRC connection: %w", err)
	}

	// Create replaceable event with d-tag = connection ID
	// Use "p" tag for derived_pubkey since only single-letter tags are indexed
	ev := &event.E{
		Kind:      KindNRCConnection,
		CreatedAt: time.Now().Unix(),
		Tags: tag.NewS(
			tag.NewFromAny("d", conn.ID),
			tag.NewFromAny("p", string(hex.Enc(conn.DerivedPubkey))), // derived pubkey for authorization lookup
		),
		Content: contentJSON,
	}

	// Sign with relay identity
	if err := ev.Sign(s.relaySigner); err != nil {
		return fmt.Errorf("failed to sign NRC connection event: %w", err)
	}

	// Save to database
	ctx := context.Background()
	if _, err := s.db.SaveEvent(ctx, ev); err != nil {
		return fmt.Errorf("failed to save NRC connection event: %w", err)
	}

	return nil
}

// GetNRCConnection retrieves an NRC connection by ID.
func (s *NRCEventStore) GetNRCConnection(id string) (*NRCConnection, error) {
	ctx := context.Background()

	// Query for the specific connection event
	limit := uint(1)
	f := &filter.F{
		Kinds:   kind.NewS(kind.New(KindNRCConnection)),
		Authors: tag.NewFromBytesSlice(s.relayPubkey),
		Tags:    tag.NewS(tag.NewFromAny("d", id)),
		Limit:   &limit,
	}

	events, err := s.db.QueryEvents(ctx, f)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, fmt.Errorf("NRC connection not found: %s", id)
	}

	return s.eventToConnection(events[0])
}

// GetNRCConnectionByDerivedPubkey retrieves an NRC connection by its derived pubkey.
func (s *NRCEventStore) GetNRCConnectionByDerivedPubkey(derivedPubkey []byte) (*NRCConnection, error) {
	ctx := context.Background()
	pubkeyHex := string(hex.Enc(derivedPubkey))

	// Query by "p" tag (derived pubkey) - single-letter tags are indexed
	limit := uint(1)
	f := &filter.F{
		Kinds:   kind.NewS(kind.New(KindNRCConnection)),
		Authors: tag.NewFromBytesSlice(s.relayPubkey),
		Tags:    tag.NewS(tag.NewFromAny("p", pubkeyHex)),
		Limit:   &limit,
	}

	events, err := s.db.QueryEvents(ctx, f)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, fmt.Errorf("NRC connection not found for pubkey")
	}

	return s.eventToConnection(events[0])
}

// DeleteNRCConnection removes an NRC connection by deleting its event.
func (s *NRCEventStore) DeleteNRCConnection(id string) error {
	ctx := context.Background()

	// Query to find the event
	limit := uint(1)
	f := &filter.F{
		Kinds:   kind.NewS(kind.New(KindNRCConnection)),
		Authors: tag.NewFromBytesSlice(s.relayPubkey),
		Tags:    tag.NewS(tag.NewFromAny("d", id)),
		Limit:   &limit,
	}

	events, err := s.db.QueryEvents(ctx, f)
	if err != nil {
		return err
	}
	if len(events) == 0 {
		return nil // Already deleted
	}

	// Delete the event
	return s.db.DeleteEvent(ctx, events[0].ID)
}

// GetAllNRCConnections returns all NRC connections.
func (s *NRCEventStore) GetAllNRCConnections() ([]*NRCConnection, error) {
	ctx := context.Background()

	// Query all NRC connection events from this relay
	f := &filter.F{
		Kinds:   kind.NewS(kind.New(KindNRCConnection)),
		Authors: tag.NewFromBytesSlice(s.relayPubkey),
	}

	events, err := s.db.QueryEvents(ctx, f)
	if err != nil {
		return nil, err
	}

	conns := make([]*NRCConnection, 0, len(events))
	for _, ev := range events {
		conn, err := s.eventToConnection(ev)
		if err != nil {
			log.W.F("failed to parse NRC connection event: %v", err)
			continue
		}
		conns = append(conns, conn)
	}

	return conns, nil
}

// GetNRCAuthorizedSecrets returns a map of derived pubkeys to labels.
func (s *NRCEventStore) GetNRCAuthorizedSecrets() (map[string]string, error) {
	conns, err := s.GetAllNRCConnections()
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, conn := range conns {
		pubkeyHex := string(hex.Enc(conn.DerivedPubkey))
		result[pubkeyHex] = conn.Label
	}

	return result, nil
}

// UpdateNRCConnectionLastUsed updates the last used timestamp.
func (s *NRCEventStore) UpdateNRCConnectionLastUsed(id string) error {
	conn, err := s.GetNRCConnection(id)
	if err != nil {
		return err
	}

	conn.LastUsed = time.Now().Unix()
	return s.SaveNRCConnection(conn)
}

// GetNRCConnectionURI generates the full connection URI for a connection.
// Uses the rendezvous URL stored in the connection.
func (s *NRCEventStore) GetNRCConnectionURI(conn *NRCConnection, relayPubkey []byte) (string, error) {
	if len(relayPubkey) != 32 {
		return "", fmt.Errorf("invalid relay pubkey length: %d", len(relayPubkey))
	}
	if conn.RendezvousURL == "" {
		return "", fmt.Errorf("connection has no rendezvous URL")
	}

	relayPubkeyHex := hex.Enc(relayPubkey)
	secretHex := hex.Enc(conn.Secret)

	uri := fmt.Sprintf("nostr+relayconnect://%s?relay=%s&secret=%s",
		relayPubkeyHex, conn.RendezvousURL, secretHex)

	if conn.Label != "" {
		uri += fmt.Sprintf("&name=%s", conn.Label)
	}

	return uri, nil
}

// eventToConnection parses an event into an NRCConnection.
func (s *NRCEventStore) eventToConnection(ev *event.E) (*NRCConnection, error) {
	var content nrcConnectionContent
	if err := json.Unmarshal(ev.Content, &content); err != nil {
		return nil, fmt.Errorf("failed to unmarshal NRC connection content: %w", err)
	}

	secret, err := hex.Dec(content.Secret)
	if err != nil {
		return nil, fmt.Errorf("failed to decode secret: %w", err)
	}

	derivedPubkey, err := hex.Dec(content.DerivedPubkey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode derived pubkey: %w", err)
	}

	var createdBy []byte
	if content.CreatedBy != "" {
		createdBy, _ = hex.Dec(content.CreatedBy)
	}

	return &NRCConnection{
		ID:            content.ID,
		Label:         content.Label,
		Secret:        secret,
		DerivedPubkey: derivedPubkey,
		RendezvousURL: content.RendezvousURL,
		CreatedAt:     content.CreatedAt,
		LastUsed:      content.LastUsed,
		CreatedBy:     createdBy,
	}, nil
}

// NRCEventAuthorizer wraps NRCEventStore to implement the NRC authorization interface.
type NRCEventAuthorizer struct {
	store *NRCEventStore
}

// NewNRCEventAuthorizer creates a new NRC authorizer from an event store.
func NewNRCEventAuthorizer(store *NRCEventStore) *NRCEventAuthorizer {
	return &NRCEventAuthorizer{store: store}
}

// GetNRCClientByPubkey looks up an authorized client by their derived pubkey.
func (a *NRCEventAuthorizer) GetNRCClientByPubkey(derivedPubkey []byte) (id string, label string, found bool, err error) {
	conn, err := a.store.GetNRCConnectionByDerivedPubkey(derivedPubkey)
	if err != nil {
		// Not found is not an error, just means not authorized
		return "", "", false, nil
	}
	if conn == nil {
		return "", "", false, nil
	}
	return conn.ID, conn.Label, true, nil
}

// UpdateNRCClientLastUsed updates the last used timestamp for tracking.
func (a *NRCEventAuthorizer) UpdateNRCClientLastUsed(id string) error {
	return a.store.UpdateNRCConnectionLastUsed(id)
}
