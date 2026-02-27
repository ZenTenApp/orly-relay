package relayinfo

import (
	"encoding/json"
	"errors"
	"os"
	"sort"
	"sync"

	"next.orly.dev/pkg/nostr/encoders/kind"
	"next.orly.dev/pkg/nostr/encoders/timestamp"
	"next.orly.dev/pkg/nostr/utils/number"
	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/log"
)

// NIP is a number and description of a nostr "improvement" possibility.
type NIP struct {
	Description string
	Number      int
}

// N returns the number of a nostr "improvement" possibility.
func (n NIP) N() int { return n.Number }

// GetList converts a NIP into a number.List of simple numbers, sorted in
// ascending order.
func GetList(items ...NIP) (n number.List) {
	for _, item := range items {
		n = append(n, item.N())
	}
	sort.Sort(n)
	return
}

// this is the list of all nips and their titles for use in the supported_nips
// field
var (
	BasicProtocol              = NIP{"Basic protocol flow description", 1}
	NIP1                       = BasicProtocol
	FollowList                 = NIP{"Follow List", 2}
	NIP2                       = FollowList
	OpenTimestampsAttestations = NIP{
		"OpenTimestamps Attestations for Events", 3,
	}
	NIP3                   = OpenTimestampsAttestations
	EncryptedDirectMessage = NIP{
		"Encrypted Direct Message -- unrecommended: deprecated in favor of NIP-17",
		4,
	}
	NIP4                  = EncryptedDirectMessage
	MappingNostrKeysToDNS = NIP{
		"Mapping Nostr keys to DNS-based internet identifiers", 5,
	}
	NIP5               = MappingNostrKeysToDNS
	BasicKeyDerivation = NIP{
		"Basic key derivation from mnemonic seed phrase", 6,
	}
	NIP6                  = BasicKeyDerivation
	WindowNostrCapability = NIP{
		"window.nostr capability for web browsers", 7,
	}
	NIP7             = WindowNostrCapability
	HandlingMentions = NIP{
		"Handling Mentions -- unrecommended: deprecated in favor of NIP-27", 8,
	}
	NIP8                     = HandlingMentions
	EventDeletion            = NIP{"Event Deletion Request", 9}
	NIP9                     = EventDeletion
	TextNotesAndThreads      = NIP{"Text Notes and Threads", 10}
	NIP10                    = TextNotesAndThreads
	RelayInformationDocument = NIP{"Relay Information Document", 11}
	NIP11                    = RelayInformationDocument
	GenericTagQueries        = NIP{"Generic Tag Queries", 12}
	NIP12                    = GenericTagQueries
	ProofOfWork              = NIP{"Proof of Work", 13}
	NIP13                    = ProofOfWork
	SubjectTag               = NIP{"Subject tag in text events", 14}
	NIP14                    = SubjectTag
	NostrMarketplace         = NIP{
		"Nostr Marketplace (for resilient marketplaces)", 15,
	}
	NIP15                 = NostrMarketplace
	EventTreatment        = NIP{"Event Treatment", 16}
	NIP16                 = EventTreatment
	PrivateDirectMessages = NIP{"Private Direct Messages", 17}
	NIP17                 = PrivateDirectMessages
	Reposts               = NIP{"Reposts", 18}
	NIP18                 = Reposts
	Bech32EncodedEntities = NIP{"bech32-encoded entities", 19}
	NIP19                 = Bech32EncodedEntities
	CommandResults        = NIP{"Command Results", 20}
	NIP20                 = CommandResults
	NostrURIScheme        = NIP{"nostr: URI scheme", 21}
	NIP21                 = NostrURIScheme
	Comment               = NIP{"Comment", 22}
	NIP22                 = Comment
	LongFormContent       = NIP{"Long-form Content", 23}
	NIP23                 = LongFormContent
	ExtraMetadata         = NIP{"Extra metadata fields and tags", 24}
	NIP24                 = ExtraMetadata
	Reactions             = NIP{"Reactions", 25}
	NIP25                 = Reactions
	DelegatedEventSigning = NIP{
		"Delegated Event Signing -- unrecommended: adds unnecessary burden for little gain",
		26,
	}
	NIP26                          = DelegatedEventSigning
	TextNoteReferences             = NIP{"Text Note References", 27}
	NIP27                          = TextNoteReferences
	PublicChat                     = NIP{"Public Chat", 28}
	NIP28                          = PublicChat
	RelayBasedGroups               = NIP{"Relay-based Groups", 29}
	NIP29                          = RelayBasedGroups
	CustomEmoji                    = NIP{"Custom Emoji", 30}
	NIP30                          = CustomEmoji
	DealingWithUnknownEvents       = NIP{"Dealing with Unknown Events", 31}
	NIP31                          = DealingWithUnknownEvents
	Labeling                       = NIP{"Labeling", 32}
	NIP32                          = Labeling
	ParameterizedReplaceableEvents = NIP{"Parameterized Replaceable Events", 33}
	NIP33                          = ParameterizedReplaceableEvents
	GitStuff                       = NIP{"git stuff", 34}
	NIP34                          = GitStuff
	Torrents                       = NIP{"Torrents", 35}
	NIP35                          = Torrents
	SensitiveContent               = NIP{"Sensitive Content", 36}
	NIP36                          = SensitiveContent
	DraftEvents                    = NIP{"Draft Events", 37}
	NIP37                          = DraftEvents
	UserStatuses                   = NIP{"User Statuses", 38}
	NIP38                          = UserStatuses
	ExternalIdentitiesInProfiles   = NIP{"External Identities in Profiles", 39}
	NIP39                          = ExternalIdentitiesInProfiles
	ExpirationTimestamp            = NIP{"Expiration Timestamp", 40}
	NIP40                          = ExpirationTimestamp
	Authentication                 = NIP{
		"Authentication of clients to relays", 42,
	}
	NIP42               = Authentication
	RelayAccessMetadata = NIP{
		"Relay Access Metadata and Requests", 43,
	}
	NIP43                     = RelayAccessMetadata
	VersionedEncryption       = NIP{"Encrypted Payloads (Versioned)", 44}
	NIP44                     = VersionedEncryption
	CountingResults           = NIP{"Counting results", 45}
	NIP45                     = CountingResults
	NostrRemoteSigning        = NIP{"Nostr Remote Signing", 46}
	NIP46                     = NostrRemoteSigning
	WalletConnect             = NIP{"Nostr Wallet Connect", 47}
	NIP47                     = WalletConnect
	ProxyTags                 = NIP{"Proxy Tags", 48}
	NIP48                     = ProxyTags
	PrivateKeyEncryption      = NIP{"Private Key Encryption", 49}
	NIP49                     = PrivateKeyEncryption
	SearchCapability          = NIP{"Search Capability", 50}
	NIP50                     = SearchCapability
	Lists                     = NIP{"Lists", 51}
	NIP51                     = Lists
	CalendarEvents            = NIP{"Calendar Events", 52}
	NIP52                     = CalendarEvents
	LiveActivities            = NIP{"Live Activities", 53}
	NIP53                     = LiveActivities
	Wiki                      = NIP{"Wiki", 54}
	NIP54                     = Wiki
	AndroidSignerApplication  = NIP{"Android Signer Application", 55}
	NIP55                     = AndroidSignerApplication
	Reporting                 = NIP{"Reporting", 56}
	NIP56                     = Reporting
	LightningZaps             = NIP{"Lightning Zaps", 57}
	NIP57                     = LightningZaps
	Badges                    = NIP{"Badges", 58}
	NIP58                     = Badges
	GiftWrap                  = NIP{"Gift Wrap", 59}
	NIP59                     = GiftWrap
	CashuWallet               = NIP{"Cashu Wallet", 60}
	NIP60                     = CashuWallet
	Nutzaps                   = NIP{"Nutzaps", 61}
	NIP61                     = Nutzaps
	RequestToVanish           = NIP{"Request to Vanish", 62}
	NIP62                     = RequestToVanish
	ChessPGN                  = NIP{"Chess (PGN)", 64}
	NIP64                     = ChessPGN
	RelayListMetadata         = NIP{"Relay List Metadata", 65}
	NIP65                     = RelayListMetadata
	RelayDiscoveryAndLiveness = NIP{
		"Relay Discovery and Liveness Monitoring", 66,
	}
	NIP66                          = RelayDiscoveryAndLiveness
	PictureFirstFeeds              = NIP{"Picture-first feeds", 68}
	NIP68                          = PictureFirstFeeds
	PeerToPeerOrderEvents          = NIP{"Peer-to-peer Order events", 69}
	NIP69                          = PeerToPeerOrderEvents
	ProtectedEvents                = NIP{"Protected Events", 70}
	NIP70                          = ProtectedEvents
	VideoEvents                    = NIP{"Video Events", 71}
	NIP71                          = VideoEvents
	ModeratedCommunities           = NIP{"Moderated Communities", 72}
	NIP72                          = ModeratedCommunities
	ExternalContentIDs             = NIP{"External Content IDs", 73}
	NIP73                          = ExternalContentIDs
	ZapGoals                       = NIP{"Zap Goals", 75}
	NIP75                          = ZapGoals
	NegentropySync                 = NIP{"Negentropy Syncing", 77}
	NIP77                          = NegentropySync
	ApplicationSpecificData        = NIP{"Application-specific data", 78}
	NIP78                          = ApplicationSpecificData
	Threads                        = NIP{"Threads", 0x7D}
	NIP7D                          = Threads
	Highlights                     = NIP{"Highlights", 84}
	NIP84                          = Highlights
	RelayManagementAPI             = NIP{"Relay Management API", 86}
	NIP86                          = RelayManagementAPI
	EcashMintDiscoverability       = NIP{"Ecash Mint Discoverability", 87}
	NIP87                          = EcashMintDiscoverability
	Polls                          = NIP{"Polls", 88}
	NIP88                          = Polls
	RecommendedApplicationHandlers = NIP{"Recommended Application Handlers", 89}
	NIP89                          = RecommendedApplicationHandlers
	DataVendingMachines            = NIP{"Data Vending Machines", 90}
	NIP90                          = DataVendingMachines
	MediaAttachments               = NIP{"Media Attachments", 92}
	NIP92                          = MediaAttachments
	FileMetadata                   = NIP{"File Metadata", 94}
	NIP94                          = FileMetadata
	HTTPFileStorageIntegration     = NIP{
		"HTTP File Storage Integration -- unrecommended: replaced by blossom APIs",
		96,
	}
	NIP96                         = HTTPFileStorageIntegration
	HTTPAuth                      = NIP{"HTTP Auth", 98}
	NIP98                         = HTTPAuth
	ClassifiedListings            = NIP{"Classified Listings", 99}
	NIP99                         = ClassifiedListings
	VoiceMessages                 = NIP{"Voice Messages", 0xA0}
	NIPA0                         = VoiceMessages
	WebBookmarks                  = NIP{"Web Bookmarks", 0xB0}
	NIPB0                         = WebBookmarks
	Blossom                       = NIP{"Blossom", 0xB7}
	NIPB7                         = Blossom
	CodeSnippets                  = NIP{"Code Snippets", 0xC0}
	NIPC0                         = CodeSnippets
	Chats                         = NIP{"Chats", 0xC7}
	NIPC7                         = Chats
	E2EEMessagingUsingMLSProtocol = NIP{
		"E2EE Messaging using MLS Protocol", 0xEE,
	}
	NIPEE = E2EEMessagingUsingMLSProtocol
)

var NIPMap = map[int]NIP{
	1: NIP1, 2: NIP2, 3: NIP3, 4: NIP4, 5: NIP5, 6: NIP6, 7: NIP7, 8: NIP8,
	9: NIP9, 10: NIP10,
	11: NIP11, 12: NIP12, 13: NIP13, 14: NIP14, 15: NIP15, 16: NIP16, 17: NIP17,
	18: NIP18, 19: NIP19, 20: NIP20,
	21: NIP21, 22: NIP22, 23: NIP23, 24: NIP24, 25: NIP25, 26: NIP26, 27: NIP27,
	28: NIP28, 29: NIP29, 30: NIP30,
	31: NIP31, 32: NIP32, 33: NIP33, 34: NIP34, 35: NIP35, 36: NIP36, 37: NIP37,
	38: NIP38, 39: NIP39, 40: NIP40,
	42: NIP42, 44: NIP44, 45: NIP45, 46: NIP46, 47: NIP47, 48: NIP48, 49: NIP49,
	50: NIP50,
	51: NIP51, 52: NIP52, 53: NIP53, 54: NIP54, 55: NIP55, 56: NIP56, 57: NIP57,
	58: NIP58, 59: NIP59, 60: NIP60,
	61: NIP61, 62: NIP62, 64: NIP64, 65: NIP65, 66: NIP66, 68: NIP68, 69: NIP69,
	70: NIP70,
	71: NIP71, 72: NIP72, 73: NIP73, 75: NIP75, 77: NIP77, 78: NIP78,
	0x7D: NIP7D, 84: NIP84,
	86: NIP86, 87: NIP87, 88: NIP88, 89: NIP89, 90: NIP90, 92: NIP92, 94: NIP94,
	96: NIP96, 98: NIP98, 99: NIP99,
	0xA0: NIPA0, 0xB0: NIPB0, 0xB7: NIPB7, 0xC0: NIPC0, 0xC7: NIPC7,
	0xEE: NIPEE,
}

// Limits are rules about what is acceptable for events and filters on a relay.
type Limits struct {
	// MaxMessageLength is the maximum number of bytes for incoming JSON that
	// the relay will attempt to decode and act upon. When you send large
	// subscriptions, you will be limited by this value. It also effectively
	// limits the maximum size of any event. Value is calculated from [ to ] and
	// is after UTF-8 serialization (so some Unicode characters will cost 2-3
	// bytes). It is equal to the maximum size of the WebSocket message frame.
	MaxMessageLength int `json:"max_message_length,omitempty"`
	// MaxSubscriptions is the total number of subscriptions that may be active
	// on a single websocket connection to this relay. It's possible that
	// authenticated clients with a (paid) relationship to the relay may have
	// higher limits.
	MaxSubscriptions int `json:"max_subscriptions,omitempty"`
	// MaxFilter is the maximum number of filter values in each subscription.
	// Must be one or higher.
	MaxFilters int `json:"max_filters,omitempty"`
	// MaxLimit is the relay server will clamp each filter's limit value to this
	// number. This means the client won't be able to get more than this number
	// of events from a single subscription filter. This clamping is typically
	// done silently by the relay, but with this number, you can know that there
	// are additional results if you narrowed your filter's time range or other
	// parameters.
	MaxLimit int `json:"max_limit,omitempty"`
	// MaxSubidLength is the maximum length of subscription id as a string.
	MaxSubidLength int `json:"max_subid_length,omitempty"`
	// MaxEventTags in any event, this is the maximum number of elements in the
	// tags list.
	MaxEventTags int `json:"max_event_tags,omitempty"`
	// MaxContentLength maximum number of characters in the content field of any
	// event. This is a count of Unicode characters. After serializing into JSON
	// it may be larger (in bytes), and is still subject to the
	// max_message_length, if defined.
	MaxContentLength int `json:"max_content_length,omitempty"`
	// MinPowDifficulty new events will require at least this difficulty of PoW,
	// based on NIP-13, or they will be rejected by this server.
	MinPowDifficulty int `json:"min_pow_difficulty,omitempty"`
	// AuthRequired means the relay requires NIP-42 authentication to happen
	// before a new connection may perform any other action. Even if set to
	// False, authentication may be required for specific actions.
	AuthRequired bool `json:"auth_required"`
	// PaymentRequired this relay requires payment before a new connection may
	// perform any action.
	PaymentRequired bool `json:"payment_required"`
	// RestrictedWrites means this relay requires some kind of condition to be
	// fulfilled to accept events (not necessarily, but including
	// payment_required and min_pow_difficulty). This should only be set to true
	// when users are expected to know the relay policy before trying to write
	// to it -- like belonging to a special pubkey-based whitelist or writing
	// only events of a specific niche kind or content. Normal anti-spam
	// heuristics, for example, do not qualify.q
	RestrictedWrites bool         `json:"restricted_writes"`
	Oldest           *timestamp.T `json:"created_at_lower_limit,omitempty"`
	Newest           *timestamp.T `json:"created_at_upper_limit,omitempty"`
}

// Payment is an amount and currency unit name.
type Payment struct {
	Amount int    `json:"amount"`
	Unit   string `json:"unit"`
}

// Sub is a subscription, with the Payment and the period it yields.
type Sub struct {
	Payment
	Period int `json:"period"`
}

// Pub is a limitation for what you can store on the relay as a kinds.S and the
// cost (for???).
type Pub struct {
	Kinds kind.S `json:"kinds"`
	Payment
}

// T is the relay information document.
type T struct {
	Name           string      `json:"name"`
	Description    string      `json:"description,omitempty"`
	PubKey         string      `json:"pubkey,omitempty"`
	Contact        string      `json:"contact,omitempty"`
	Nips           number.List `json:"supported_nips"`
	Software       string      `json:"software"`
	Version        string      `json:"version"`
	Limitation     Limits      `json:"limitation,omitempty"`
	Retention      any         `json:"retention,omitempty"`
	RelayCountries []string    `json:"relay_countries,omitempty"`
	LanguageTags   []string    `json:"language_tags,omitempty"`
	Tags           []string    `json:"tags,omitempty"`
	PostingPolicy  string      `json:"posting_policy,omitempty"`
	PaymentsURL    string      `json:"payments_url,omitempty"`
	Fees           *Fees       `json:"fees,omitempty"`
	Icon           string      `json:"icon"`
	sync.Mutex
}

// NewInfo populates the nips map, and if an Info structure is provided, it is
// used and its nips map is populated if it isn't already.
func NewInfo(inf *T) (info *T) {
	if inf != nil {
		info = inf
	} else {
		info = &T{
			Limitation: Limits{
				MaxLimit: 500,
			},
		}
	}
	return
}

// Clone replicates a relayinfo.T.
// todo: this could be done better, but i don't think it's in use.
func (ri *T) Clone() (r2 *T, err error) {
	r2 = new(T)
	var b []byte
	// beware, this will escape <, > and & to unicode escapes but that should be
	// ok since this data is not signed in events until after it is marshaled.
	if b, err = json.Marshal(ri); chk.E(err) {
		return
	}
	if err = json.Unmarshal(b, r2); chk.E(err) {
		return
	}
	return
}

// AddNIPs adds one or more numbers to the list of NIPs.
func (ri *T) AddNIPs(n ...int) {
	ri.Lock()
	for _, num := range n {
		ri.Nips = append(ri.Nips, num)
	}
	ri.Unlock()
}

// HasNIP returns true if the given number is found in the list.
func (ri *T) HasNIP(n int) (ok bool) {
	ri.Lock()
	for i := range ri.Nips {
		if ri.Nips[i] == n {
			ok = true
			break
		}
	}
	ri.Unlock()
	return
}

// Save the relayinfo.T to a given file as JSON.
func (ri *T) Save(filename string) (err error) {
	if ri == nil {
		err = errors.New("cannot save nil relay info document")
		log.E.Ln(err)
		return
	}
	var b []byte
	// beware, this will escape <, > and & to unicode escapes but that should be
	// ok since this data is not signed in events until after it is marshaled.
	if b, err = json.MarshalIndent(ri, "", "    "); chk.E(err) {
		return
	}
	if err = os.WriteFile(filename, b, 0600); chk.E(err) {
		return
	}
	return
}

// Load a given file and decode the JSON relayinfo.T encoded in it.
func (ri *T) Load(filename string) (err error) {
	if ri == nil {
		err = errors.New("cannot load into nil config")
		log.E.Ln(err)
		return
	}
	var b []byte
	if b, err = os.ReadFile(filename); chk.E(err) {
		return
	}
	// log.S.ToSliceOfBytes("realy information document\n%s", string(b))
	if err = json.Unmarshal(b, ri); chk.E(err) {
		return
	}
	return
}
