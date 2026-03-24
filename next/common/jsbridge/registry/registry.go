package registry

// Registry — cross-module hook system via JS global self.$hooks.
// Identity state and hook dispatch for isolated SW domain modules.

// Identity state — accessible from any module.
func SetSeckey(hex string) { panic("jsbridge") }
func Seckey() string       { panic("jsbridge") }
func SetPubkey(pub string) { panic("jsbridge") }
func Pubkey() string       { panic("jsbridge") }
func SetHasKey(v bool)     { panic("jsbridge") }
func HasKey() bool         { panic("jsbridge") }

// --- Register extension hooks (called from extension init) ---

func OnEncryptNip04(fn func(string, string, func(string)))         { panic("jsbridge") }
func OnEncryptNip17(fn func(string, string, func(string, string))) { panic("jsbridge") }
func OnDecryptDM(fn func(string, func(string)))                    { panic("jsbridge") }
func OnMarmotInit(fn func(string))                                 { panic("jsbridge") }
func OnMarmotSend(fn func(string, string))                         { panic("jsbridge") }
func OnMarmotSubscribe(fn func())                                  { panic("jsbridge") }
func OnMarmotPublishKP(fn func(string))                            { panic("jsbridge") }
func OnMarmotListGroups(fn func(string))                           { panic("jsbridge") }

// --- Register core hooks (called from core init) ---

func OnSaveDMRecord(fn func(string))         { panic("jsbridge") }
func OnBroadcastToClients(fn func(string))   { panic("jsbridge") }
func OnSendToClient(fn func(string, string)) { panic("jsbridge") }

// --- Call extension hooks ---

func EncryptNip04(pubkey, content string, cb func(string))         { panic("jsbridge") }
func EncryptNip17(pubkey, content string, cb func(string, string)) { panic("jsbridge") }
func DecryptDM(evJSON string, cb func(string))                     { panic("jsbridge") }
func MakeDMRecord(peer, from, content string, ts int64, proto, eid string) string {
	panic("jsbridge")
}
func MarmotInit(relayURLsJSON string)      { panic("jsbridge") }
func MarmotSend(recipient, content string) { panic("jsbridge") }
func MarmotSubscribe()                     { panic("jsbridge") }
func MarmotPublishKP(relayURLsJSON string) { panic("jsbridge") }
func MarmotListGroups(clientID string)     { panic("jsbridge") }

// --- Call core hooks ---

func SaveDMRecord(dmJSON string)        { panic("jsbridge") }
func BroadcastToClients(msg string)     { panic("jsbridge") }
func SendToClient(clientID, msg string) { panic("jsbridge") }

// --- Module management ---

func HasHook(name string) bool          { panic("jsbridge") }
func LoadModule(name string, cb func()) { panic("jsbridge") }
