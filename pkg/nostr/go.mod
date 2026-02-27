module git.mleku.dev/mleku/nostr

go 1.25.3

require (
	github.com/davecgh/go-spew v1.1.1
	github.com/emersion/go-mls v0.0.0-20251224142416-08bd20b5f8f2
	github.com/gorilla/websocket v1.5.3
	github.com/minio/sha256-simd v1.0.1
	github.com/puzpuzpuz/xsync/v3 v3.5.1
	github.com/stretchr/testify v1.11.1
	github.com/templexxx/xhex v0.0.0-20200614015412-aed53437177b
	golang.org/x/crypto v0.46.0
	golang.org/x/exp v0.0.0-20251209150349-8475f28825e9
	golang.org/x/net v0.48.0
	lol.mleku.dev v1.0.5
	lukechampine.com/frand v1.5.1
	p256k1.mleku.dev v1.1.2
)

require (
	github.com/cloudflare/circl v1.6.1 // indirect
	github.com/ebitengine/purego v0.9.1 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/templexxx/cpu v0.0.1 // indirect
	golang.org/x/sys v0.39.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace p256k1.mleku.dev => ../p256k1
