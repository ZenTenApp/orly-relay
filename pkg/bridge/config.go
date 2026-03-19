// Package bridge implements a bidirectional Nostr-Email bridge using the
// Marmot protocol (MLS-based E2E encrypted messaging) for all Nostr-side
// communication. It receives inbound email via SMTP and delivers it as
// Marmot DMs, and receives outbound email as Marmot DMs to send via SMTP.
package bridge

// Config holds the bridge configuration. Constructed by callers from
// environment values (see app/config/config.go GetBridgeConfigValues).
type Config struct {
	// Domain is the email domain (e.g., "relay.example.com").
	// Users get addresses like npub1abc...@relay.example.com.
	Domain string

	// NSEC is the bridge identity secret key in nsec or hex format.
	// If empty, the bridge attempts to read it from the database
	// (monolithic mode) or from a file (standalone mode).
	NSEC string

	// RelayURL is the WebSocket URL of the relay to connect to.
	// Used in standalone mode. In monolithic mode this can be empty
	// (events are routed in-process).
	RelayURL string

	// PublicRelayURL is the public wss:// URL advertised in kind 10002
	// and kind 10050 relay list events. Separate from RelayURL because
	// the bridge may connect via ws://localhost internally.
	PublicRelayURL string

	// SMTPPort is the port the SMTP server listens on.
	SMTPPort int

	// SMTPHost is the address the SMTP server binds to.
	SMTPHost string

	// DataDir is where the bridge persists its state (group state,
	// subscriptions, identity file for standalone mode).
	DataDir string

	// DKIMKeyPath is the path to the DKIM private key PEM file.
	DKIMKeyPath string

	// DKIMSelector is the DKIM selector for DNS TXT lookup.
	DKIMSelector string

	// NWCURI is the NWC connection string for subscription payments.
	NWCURI string

	// MonthlyPriceSats is the price in sats for one month subscription.
	MonthlyPriceSats int64

	// ComposeURL is the public URL of the compose form page.
	ComposeURL string

	// SMTPRelayHost is the smarthost for outbound email delivery
	// (e.g., "smtp.migadu.com"). If empty, direct MX delivery is used.
	SMTPRelayHost string

	// SMTPRelayPort is the smarthost port (typically 587 for STARTTLS).
	SMTPRelayPort int

	// SMTPRelayUsername is the SMTP AUTH username for the smarthost.
	SMTPRelayUsername string

	// SMTPRelayPassword is the SMTP AUTH password for the smarthost.
	SMTPRelayPassword string

	// ACLGRPCServer is the gRPC address of the ACL server.
	// When set, the bridge uses ACL-backed subscriptions instead of file store.
	ACLGRPCServer string

	// AliasPriceSats is the monthly price in sats for an alias email address.
	// Must be >= MonthlyPriceSats. If zero, defaults to 2x MonthlyPriceSats.
	AliasPriceSats int64

	// ProfilePath is the path to a profile template file (email-header format).
	// If the file exists, the bridge publishes a kind 0 metadata event on startup.
	// Default: $DataDir/profile.txt
	ProfilePath string

	// MLSEnabled enables MLS (NIP-EE) protocol support. When true, the bridge
	// publishes MLS key packages and can exchange DMs with MLS-capable clients
	// like White Noise.
	MLSEnabled bool
}
