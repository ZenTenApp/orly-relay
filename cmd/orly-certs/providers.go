package main

import (
	"fmt"

	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/providers/dns"
)

// NewDNSProvider creates a DNS challenge provider by name.
// The provider will be configured using standard environment variables
// as documented by lego for each provider.
//
// Common providers and their environment variables:
//   - cloudflare: CF_API_TOKEN or CF_API_EMAIL + CF_API_KEY
//   - route53: AWS_ACCESS_KEY_ID + AWS_SECRET_ACCESS_KEY + AWS_REGION
//   - hetzner: HETZNER_API_KEY
//   - digitalocean: DO_AUTH_TOKEN
//   - google: GCE_PROJECT + GCE_SERVICE_ACCOUNT_FILE
//   - namecheap: NAMECHEAP_API_USER + NAMECHEAP_API_KEY
//   - godaddy: GODADDY_API_KEY + GODADDY_API_SECRET
//   - ovh: OVH_ENDPOINT + OVH_APPLICATION_KEY + OVH_APPLICATION_SECRET + OVH_CONSUMER_KEY
//   - vultr: VULTR_API_KEY
//   - linode: LINODE_TOKEN
//
// See https://go-acme.github.io/lego/dns/ for full list and documentation.
func NewDNSProvider(name string) (challenge.Provider, error) {
	provider, err := dns.NewDNSChallengeProviderByName(name)
	if err != nil {
		return nil, fmt.Errorf("failed to create DNS provider '%s': %w", name, err)
	}
	return provider, nil
}

// SupportedProviders returns a list of commonly used DNS providers.
// This is not exhaustive - lego supports 100+ providers.
func SupportedProviders() []string {
	return []string{
		"cloudflare",
		"route53",
		"hetzner",
		"digitalocean",
		"google",
		"namecheap",
		"godaddy",
		"ovh",
		"vultr",
		"linode",
		"gandi",
		"dnsimple",
		"duckdns",
		"azure",
		"alidns",
	}
}
