package vegadns

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// Domain struct containing a domain object.
type Domain struct {
	Status   string `json:"status"`
	Domain   string `json:"domain"`
	DomainID int    `json:"domain_id"`
	OwnerID  int    `json:"owner_id"`
}

// DomainResponse api response of a domain list.
type DomainResponse struct {
	Status  string   `json:"status"`
	Total   int      `json:"total_domains"`
	Domains []Domain `json:"domains"`
}

// GetDomainID returns the id for a domain.
func (c *Client) GetDomainID(ctx context.Context, domain string) (int, error) {
	domains, err := c.GetDomains(ctx, domain)
	if err != nil {
		return -1, err
	}

	for _, d := range domains {
		if d.Domain == domain {
			return d.DomainID, nil
		}
	}

	return -1, fmt.Errorf("domain %s not found", domain)
}

// GetDomains gets domains.
// https://generator.swagger.io/?url=https://raw.githubusercontent.com/shupp/VegaDNS-API/refs/heads/master/swagger/vegadns.swagger.json#/Domains/get_domains
func (c *Client) GetDomains(ctx context.Context, domain string) ([]Domain, error) {
	params := make(url.Values)

	if domain != "" {
		params.Set("search", domain)
	}

	req, err := c.newRequest(ctx, http.MethodGet, c.baseURL.JoinPath("domains"), params)
	if err != nil {
		return nil, err
	}

	answer := DomainResponse{}

	err = c.do(req, http.StatusOK, &answer)
	if err != nil {
		return nil, err
	}

	return answer.Domains, nil
}
